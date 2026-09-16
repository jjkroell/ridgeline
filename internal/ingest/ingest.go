// Package ingest connects to a MeshCore observer MQTT broker, decodes incoming
// packets, and persists them via the store.
package ingest

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/jjkroell/ridgeline/internal/config"
	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// envelope is the JSON wrapper an observer publishes for each received packet.
// Field names mirror the MeshCore observer convention. Note that real
// observers encode SNR/RSSI as JSON strings (e.g. "11", "-45"), so those use
// a lenient numeric type that accepts both strings and numbers.
type envelope struct {
	Raw      string   `json:"raw"`
	SNR      optFloat `json:"SNR"`
	RSSI     optFloat `json:"RSSI"`
	Origin   string   `json:"origin"`    // friendly observer name
	OriginID string   `json:"origin_id"` // observer public key
	Region   string   `json:"region"`
}

// optFloat is a float that unmarshals from a JSON number or a quoted numeric
// string, tracking whether a usable value was present.
type optFloat struct {
	set bool
	val float64
}

func (o *optFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil // tolerate non-numeric values rather than dropping the packet
	}
	o.set, o.val = true, v
	return nil
}

func (o optFloat) ptr() *float64 {
	if !o.set {
		return nil
	}
	v := o.val
	return &v
}

// Ingestor subscribes to the broker and writes decoded packets to the store.
type Ingestor struct {
	cfg   config.MQTT
	store *store.Store
	log   *slog.Logger

	// OnObservation, if set, is called for each successfully stored
	// observation — used to feed the live WebSocket.
	OnObservation func(store.Observation)

	client mqtt.Client

	// Packets from observers that have not yet reported a radio preset, held
	// until the verdict arrives. See holding_pen.go.
	penMu sync.Mutex
	pens  map[string]*pen
	// Connection-gap accounting. Packets published while disconnected are lost,
	// so the time spent disconnected is the loss.
	lostAt   atomic.Value // time.Time of the last connection loss
	downtime atomic.Int64 // cumulative nanoseconds disconnected
	// commitFn lets tests observe what the pen releases without standing up a
	// store. nil in production, where commit() is called directly.
	commitFn func(store.Observation) bool
	// sweepHook fires once the pen sweeper is running, so a test can assert the
	// goroutine actually started rather than that Start() returned.
	sweepHook func()
	done      chan struct{}
	once      sync.Once
}

// New creates an Ingestor.
func New(cfg config.MQTT, st *store.Store, log *slog.Logger) *Ingestor {
	return &Ingestor{cfg: cfg, store: st, log: log, done: make(chan struct{})}
}

// Start connects to the broker and subscribes. It returns once the
// subscriptions are established; the client reconnects automatically on drop.
func (in *Ingestor) Start() error {
	opts := mqtt.NewClientOptions().
		AddBroker(in.cfg.Broker).
		SetClientID(in.cfg.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		// CleanSession stays true, and the subscriptions stay at QoS 0, because
		// the alternative is worse. A persistent session with QoS 1 would have
		// the broker redeliver anything missed during a drop — but redelivery is
		// indistinguishable from a genuine repeat here: 64% of stored
		// observations already share (observer_id, message_hash) with another
		// row, one hash appearing 1058 times from a single observer, and MQTT
		// carries no message identity to dedupe on. Guaranteed delivery would
		// trade a measured ~0.5% loss for unbounded duplication.
		//
		// So the goal is short gaps, not no gaps.
		SetCleanSession(true).
		// paho's default reconnect backoff climbs to TEN MINUTES. Nothing here
		// benefits from backing off that far: the broker is a container on the
		// same host, and a ten-minute gap would lose far more than the blip that
		// caused it. Measured gaps were 0-98s; this caps the tail.
		SetMaxReconnectInterval(15 * time.Second).
		SetConnectRetryInterval(5 * time.Second).
		// The broker's auth plugin calls back into this process for every ACL
		// check, so a burst of publishes can delay a PINGRESP. The defaults
		// (30s keepalive, 10s ping timeout) turn that into a dropped connection
		// and a reconnect that loses packets. More tolerance costs only slower
		// detection of a genuinely dead link, which AutoReconnect then handles.
		SetKeepAlive(45 * time.Second).
		SetPingTimeout(15 * time.Second)

	// Until now a dropped connection was invisible: nothing logged it, and the
	// only trace was a second "subscribed" line appearing later. Packets
	// published while disconnected are gone — not queued, not retried — so the
	// loss must at least be visible and measurable.
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		in.lostAt.Store(time.Now())
		in.log.Warn("mqtt connection lost — packets published while disconnected are unrecoverable",
			"broker", in.cfg.Broker, "err", err)
	})
	opts.SetReconnectingHandler(func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		in.log.Info("mqtt reconnecting", "broker", in.cfg.Broker)
	})
	if in.cfg.Username != "" {
		opts.SetUsername(in.cfg.Username)
		opts.SetPassword(in.cfg.Password)
	}
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		// Report how long the gap was. A reconnect on its own says little; the
		// downtime is the part that maps to lost packets.
		if v := in.lostAt.Load(); v != nil {
			if t, ok := v.(time.Time); ok && !t.IsZero() {
				d := time.Since(t)
				in.downtime.Add(int64(d))
				in.log.Warn("mqtt reconnected after a gap",
					"broker", in.cfg.Broker, "downtime", d.Round(time.Second),
					"cumulativeDowntime", time.Duration(in.downtime.Load()).Round(time.Second))
				in.lostAt.Store(time.Time{})
			}
		}
		for _, topic := range in.cfg.Topics {
			if tok := c.Subscribe(topic, 0, in.handle); tok.Wait() && tok.Error() != nil {
				in.log.Error("subscribe failed", "topic", topic, "err", tok.Error())
			} else {
				in.log.Info("subscribed", "topic", topic)
			}
		}
	})

	in.client = mqtt.NewClient(opts)
	tok := in.client.Connect()
	// With ConnectRetry + AutoReconnect, paho keeps (re)connecting in the
	// background. Don't block the whole daemon — the HTTP server, /api/health and
	// the web UI — on the broker being reachable at startup. Wait a short bounded
	// time for a fast connect (so the common case still logs "subscribed" before
	// serving), then proceed regardless; the connection completes/retries later.
	// The pen sweeper is started here, before the connect is waited on, because
	// its lifetime is the Ingestor's and not the broker's. It previously lived
	// inside the timeout branch below, which meant that on every NORMAL startup —
	// broker reachable, connect fast — it never ran at all: pens never expired,
	// never got logged, and the grace-period quarantine could not fire. The bug
	// was invisible precisely because its only symptom was silence.
	go in.sweepPens()

	if !tok.WaitTimeout(10 * time.Second) {
		in.log.Warn("mqtt connect still pending; retrying in background", "broker", in.cfg.Broker)
		return nil
	}
	return tok.Error()
}

// Stop disconnects from the broker and stops the pen sweeper.
//
// Anything still held is dropped with the process. That is correct: it was never
// vouched for, and re-admitting unverified packets across a restart would defeat
// the hold.
func (in *Ingestor) Stop() {
	in.once.Do(func() { close(in.done) })
	if in.client != nil {
		in.client.Disconnect(250)
	}
}

func (in *Ingestor) handle(_ mqtt.Client, msg mqtt.Message) {
	// This callback runs in a paho goroutine on untrusted, attacker-influenceable
	// payloads (raw packet hex, observer status). An unrecovered panic here would
	// be fatal to the whole daemon, so contain it: log and drop the one message.
	defer func() {
		if rec := recover(); rec != nil {
			in.log.Error("recovered from panic in ingest handler", "topic", msg.Topic(), "panic", rec)
		}
	}()

	if strings.HasSuffix(msg.Topic(), "/status") {
		in.handleStatus(msg)
		return
	}
	var env envelope
	if err := json.Unmarshal(msg.Payload(), &env); err != nil {
		in.log.Debug("bad envelope", "topic", msg.Topic(), "err", err)
		return
	}
	if env.Raw == "" {
		return
	}

	packet, err := meshcore.DecodeHex(env.Raw)
	if err != nil {
		in.log.Debug("decode error", "err", err)
		return
	}

	observerKey, region := topicMeta(msg.Topic())
	if region == "" {
		region = env.Region
	}
	// The public key identifies the observer; the friendly name is a label the
	// operator can change at any time, so it must not be the identity. The topic
	// carries the key on every message, and env.OriginID repeats it; fall back to
	// the name only when neither is present.
	observerID, observerName := observerKey, env.Origin
	if observerID == "" {
		observerID = env.OriginID
	}
	if observerID == "" {
		observerID = env.Origin
	}

	// Drop blocklisted traffic (quarantined RF bridges / rogue MQTT publishers)
	// before it ever reaches the store.
	if in.store.ShouldDrop(packet, observerID) {
		in.log.Debug("dropped blocklisted packet", "observer", observerID)
		return
	}
	// Discard everything from an observer the operator has stood down. Checked
	// AFTER the blocklist so a blocked publisher is still reported as blocked
	// rather than as merely on standby, and BEFORE the store so a stand-down
	// leaves no trace in the data — which is the whole point of it. The /status
	// path is deliberately untouched: the receiver stays connected and keeps
	// reporting telemetry while none of what it hears is kept.
	if in.store.ObserverOnStandby(observerID) {
		// Counts the discard and keeps last_seen current — we DID hear from the
		// observer, we just aren't keeping what it said, and a frozen last_seen
		// would make it read as silent and get swept by retention.
		in.store.RecordStandbyDrop(observerID, time.Now().UTC().Format(time.RFC3339Nano))
		in.log.Debug("discarded packet from observer on standby", "observer", observerID)
		return
	}

	// Refuse what an observer on the wrong radio preset reports. It is hearing a
	// different network, and once stored its packets are indistinguishable from
	// this mesh's own. Checked AFTER standby so an operator's deliberate
	// stand-down is reported as that rather than as a config fault, and the
	// /status path is left alone on purpose: the observer stays connected and
	// keeps saying what it is set to, which is what makes the fault fixable.
	if in.store.ObserverRadioQuarantined(observerID) {
		// Counts the drop and keeps last_seen current — we heard from it, we
		// just refused the contents. A frozen last_seen would have retention
		// sweep the row away within the hour, taking the quarantine with it.
		in.store.RecordRadioQuarantineDrop(observerID, time.Now().UTC().Format(time.RFC3339Nano))
		in.log.Debug("dropped packet from observer on a foreign radio preset", "observer", observerID)
		return
	}

	obs := store.Observation{
		Packet:         packet,
		RawHex:         env.Raw,
		ObserverID:     observerID,
		ObserverName:   observerName,
		ObserverPubkey: env.OriginID,
		Region:         region,
		SNR:            env.SNR.ptr(),
		RSSI:           env.RSSI.ptr(),
		// Server clock owns ordering; the envelope timestamp is untrusted
		// (observers with skewed clocks would poison ordering).
		ReceivedAt: time.Now(),
	}

	// An observer that has not yet said what radio it is on gets held rather
	// than stored. Its first status decides whether any of this is kept, and
	// arrives within about five minutes. Only while the guard is configured:
	// with no presets to check against there is nothing to wait for.
	if in.store.AllowedRadioCount() > 0 && !in.store.ObserverRadioConfirmed(observerID) {
		if !in.holdObservation(observerID, obs) {
			in.log.Debug("held-packet cap reached, dropping", "observer", observerID)
		}
		return
	}

	in.commit(obs)
}

// commit stores an observation and runs everything that follows from storing
// one. Shared by the live path and by the holding pen's release, so a held
// packet is not a second-class citizen: it verifies claims and reaches the live
// feed exactly as it would have.
func (in *Ingestor) commit(obs store.Observation) bool {
	if in.commitFn != nil {
		return in.commitFn(obs)
	}
	packet := obs.Packet
	if err := in.store.Record(obs); err != nil {
		in.log.Error("store record failed", "err", err)
		return false
	}
	// Node-ownership claim verification: a signature-valid advert whose node has
	// a pending claim may carry the verification code in its name. The signature
	// check is essential — it proves the advert came from the node's own key, so
	// a rogue observer can't forge a claim by injecting the code. Gated by the
	// in-memory pending-claim set, so the overwhelming common case costs nothing.
	if a := packet.Advert; a != nil && a.SignatureValid && in.store.HasPendingClaim(a.PublicKey) {
		if verified, err := in.store.VerifyPendingClaims(a.PublicKey, a.Name); err != nil {
			in.log.Warn("claim verification failed", "node", a.PublicKey, "err", err)
		} else {
			for _, v := range verified {
				in.log.Info("node ownership claim verified", "node", a.PublicKey, "user", v.UserID)
			}
		}
	}
	if in.OnObservation != nil {
		in.OnObservation(obs)
	}
	return true
}

// statusEnvelope is the JSON an observer publishes on its /status topic: device
// identity, radio config ("freq,bw,sf,cr") and a stats block. Field names mirror
// the real MeshCore observer status messages.
type statusEnvelope struct {
	Status          string `json:"status"`
	Origin          string `json:"origin"`
	OriginID        string `json:"origin_id"`
	Region          string `json:"region"`
	Radio           string `json:"radio"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	ClientVersion   string `json:"client_version"`
	Stats           struct {
		BatteryMV  *int     `json:"battery_mv"`
		UptimeSecs *int64   `json:"uptime_secs"`
		NoiseFloor *float64 `json:"noise_floor"`
		TxAirSecs  *float64 `json:"tx_air_secs"`
		RxAirSecs  *float64 `json:"rx_air_secs"`
		RecvErrors *int     `json:"recv_errors"`
		Errors     *int     `json:"errors"` // some clients use "errors"
		QueueLen   *int     `json:"queue_len"`
	} `json:"stats"`
}

// handleStatus parses an observer /status message and stores its latest device
// telemetry. The observer is keyed by its friendly origin name (matching the
// packet path), so status attaches to the same observer row.
func (in *Ingestor) handleStatus(msg mqtt.Message) {
	var env statusEnvelope
	if err := json.Unmarshal(msg.Payload(), &env); err != nil {
		in.log.Debug("bad status envelope", "topic", msg.Topic(), "err", err)
		return
	}
	observerKey, region := topicMeta(msg.Topic())
	if region == "" {
		region = env.Region
	}
	// Keyed by public key like the packet path — the name is only a label.
	observerID, observerName := observerKey, env.Origin
	if observerID == "" {
		observerID = env.OriginID
	}
	if observerID == "" {
		observerID = env.Origin
	}
	if observerID == "" {
		return
	}

	st := store.ObserverStatus{
		State:         env.Status,
		Radio:         env.Radio,
		Model:         env.Model,
		Firmware:      env.FirmwareVersion,
		ClientVersion: env.ClientVersion,
		BatteryMV:     env.Stats.BatteryMV,
		UptimeSecs:    env.Stats.UptimeSecs,
		NoiseFloor:    env.Stats.NoiseFloor,
		TxAirSecs:     env.Stats.TxAirSecs,
		RxAirSecs:     env.Stats.RxAirSecs,
		RecvErrors:    firstNonNil(env.Stats.RecvErrors, env.Stats.Errors),
		QueueLen:      env.Stats.QueueLen,
	}
	parseRadio(env.Radio, &st)

	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	pubkey := env.OriginID
	if pubkey == "" {
		pubkey = observerKey
	}
	now := time.Now().UTC().Format(time.RFC3339)
	// A retained status is the broker replaying an observer's last known value on
	// every reconnect, not a live sighting — it may refresh an observer we already
	// know, but it must never create one, or decommissioned observers reappear.
	// Returning early also skips the telemetry append below: stamping a replayed
	// battery/noise reading with the reconnect time invents a data point that was
	// never measured, one per reconnect, for as long as the retained message lives.
	if msg.Retained() {
		found, err := in.store.UpdateObserverStatusIfPresent(observerID, observerName, region, pubkey, string(b), env.Radio, now)
		if err != nil {
			in.log.Error("update observer status failed", "err", err)
		} else if !found {
			in.log.Debug("ignored retained status for unknown observer", "observer", observerID)
		} else {
			// A retained status is stale, but it is still this observer saying
			// what it is set to. Evaluating it means a reconnecting observer
			// that was fixed while away is readmitted on reconnect rather than
			// waiting for its next live status.
			in.evaluateRadio(observerID, observerName, env.Radio, now)
		}
		return
	}
	if err := in.store.UpsertObserverStatus(observerID, observerName, region, pubkey, string(b), env.Radio, now); err != nil {
		in.log.Error("store observer status failed", "err", err)
	}
	in.evaluateRadio(observerID, observerName, env.Radio, now)
	// Append a point to the telemetry time series (rate-floored in the store) so
	// battery/noise/airtime can be trended — the observer row only keeps the latest.
	if err := in.store.RecordObserverTelemetry(observerID, now, st); err != nil {
		in.log.Error("store observer telemetry failed", "err", err)
	}
}

// parseRadio splits the "freq,bw,sf,cr" radio string into typed fields.
func parseRadio(radio string, st *store.ObserverStatus) {
	parts := strings.Split(strings.TrimSpace(radio), ",")
	if len(parts) >= 1 {
		if f, err := strconv.ParseFloat(parts[0], 64); err == nil {
			st.FreqMHz = &f
		}
	}
	if len(parts) >= 2 {
		if f, err := strconv.ParseFloat(parts[1], 64); err == nil {
			st.BandwidthKHz = &f
		}
	}
	if len(parts) >= 3 {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil {
			st.SpreadingFactor = &n
		}
	}
	if len(parts) >= 4 {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[3])); err == nil {
			st.CodingRate = &n
		}
	}
}

func firstNonNil(a, b *int) *int {
	if a != nil {
		return a
	}
	return b
}

// topicMeta extracts region and observer id from a meshcore/{region}/{observer}/packets
// topic. Missing segments yield empty strings.
func topicMeta(topic string) (observerID, region string) {
	parts := strings.Split(topic, "/")
	if len(parts) >= 2 {
		region = parts[1]
	}
	if len(parts) >= 3 {
		observerID = parts[2]
	}
	return observerID, region
}

// evaluateRadio applies the radio-preset guard to a reported config and logs
// only the transitions. Logging every status would bury the one line that
// matters — the moment an observer started or stopped being trusted.
func (in *Ingestor) evaluateRadio(observerID, observerName, reported, at string) {
	bad, changed := in.store.EvaluateObserverRadio(observerID, reported, at)
	if !changed {
		return
	}
	if bad {
		in.log.Warn("observer quarantined: radio preset is not on this mesh",
			"observer", observerID, "name", observerName, "radio", reported)
		// Anything held while we waited for this status was heard on that same
		// wrong preset. It is not this mesh's traffic and must not be stored.
		in.discardHeld(observerID, "radio preset is not on this mesh")
		return
	}
	in.log.Info("observer radio preset accepted",
		"observer", observerID, "name", observerName, "radio", reported)
	// Vouched for — commit whatever arrived before it identified itself.
	in.releaseHeld(observerID)
}
