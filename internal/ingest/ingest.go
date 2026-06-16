// Package ingest connects to a MeshCore observer MQTT broker, decodes incoming
// packets, and persists them via the store.
package ingest

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/jjkroell/ridgeline/internal/config"
	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// envelope is the JSON wrapper an observer publishes for each received packet.
// Field names mirror the MeshCore observer convention.
type envelope struct {
	Raw       string   `json:"raw"`
	SNR       *float64 `json:"SNR"`
	RSSI      *float64 `json:"RSSI"`
	Origin    string   `json:"origin"`
	Region    string   `json:"region"`
	Timestamp string   `json:"timestamp"`
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
}

// New creates an Ingestor.
func New(cfg config.MQTT, st *store.Store, log *slog.Logger) *Ingestor {
	return &Ingestor{cfg: cfg, store: st, log: log}
}

// Start connects to the broker and subscribes. It returns once the
// subscriptions are established; the client reconnects automatically on drop.
func (in *Ingestor) Start() error {
	opts := mqtt.NewClientOptions().
		AddBroker(in.cfg.Broker).
		SetClientID(in.cfg.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetCleanSession(true)
	if in.cfg.Username != "" {
		opts.SetUsername(in.cfg.Username)
		opts.SetPassword(in.cfg.Password)
	}
	opts.SetOnConnectHandler(func(c mqtt.Client) {
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
	tok.Wait()
	return tok.Error()
}

// Stop disconnects from the broker.
func (in *Ingestor) Stop() {
	if in.client != nil {
		in.client.Disconnect(250)
	}
}

func (in *Ingestor) handle(_ mqtt.Client, msg mqtt.Message) {
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

	observerID, region := topicMeta(msg.Topic())
	if region == "" {
		region = env.Region
	}
	if observerID == "" {
		observerID = env.Origin
	}

	obs := store.Observation{
		Packet:     packet,
		ObserverID: observerID,
		Region:     region,
		SNR:        env.SNR,
		RSSI:       env.RSSI,
		// Server clock owns ordering; the envelope timestamp is untrusted
		// (observers with skewed clocks would poison ordering).
		ReceivedAt: time.Now(),
	}
	if err := in.store.Record(obs); err != nil {
		in.log.Error("store record failed", "err", err)
		return
	}
	if in.OnObservation != nil {
		in.OnObservation(obs)
	}
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
