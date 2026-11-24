package ingest

import (
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Receiving a message and handling it are separate, with a queue between.
//
// paho delivers messages in order and calls the handler from the goroutine that
// reads the socket. Handling one means waiting for the store's lock and its one
// connection, so anything slow there (a node purge, a cold disk) stopped the
// socket being read: the broker's PINGRESP sat unread behind the backlog, the
// client gave the link up as dead, and every message still unread was lost.
// Measured 2026-11-20 to 24: 36 drops, all but one traced to a purge.
//
// One worker, not one goroutine per message: order matters (a status decides
// the fate of the packets after it) and the store serializes anyway.
const (
	// queueLen is over half an hour of traffic at the busiest rate seen (~10/s).
	queueLen = 20000
	// maxQueueBytes bounds memory whatever publishers send.
	maxQueueBytes = 64 << 20
	// maxQueuedMessage is the largest payload any handler accepts.
	maxQueuedMessage = max(maxPacketEnvelope, maxStatusPayload, maxNeighborPayload)
	// queueLagWarn is how far behind the worker may fall before it is logged.
	queueLagWarn = 15 * time.Second
	// drainLimit is how long Stop spends handling what was already received.
	drainLimit = 3 * time.Second
)

// arrival is a received message and when it was received.
type arrival struct {
	msg mqtt.Message
	at  time.Time
}

// receive is paho's message callback. It must never block or touch the store.
func (in *Ingestor) receive(_ mqtt.Client, msg mqtt.Message) {
	now := time.Now()
	// Every message — even one we go on to drop — is proof the link was alive at
	// this instant. On the next reconnect this marks the last moment before the
	// silence began, which is how the gap is measured (see reconnectGap).
	in.lastMsg.Store(now.UnixNano())

	size := int64(len(msg.Payload()))
	if size > maxQueuedMessage {
		in.log.Debug("oversized message", "topic", msg.Topic(), "bytes", size)
		return
	}
	// Full means far behind already: drop the newest rather than block, which
	// would only bring back the dropped connections.
	if in.queuedBytes.Add(size) > maxQueueBytes {
		in.queuedBytes.Add(-size)
		in.queueFull.Add(1)
		return
	}
	select {
	case in.queue <- arrival{msg, now}:
	default:
		in.queuedBytes.Add(-size)
		in.queueFull.Add(1)
	}
}

// work handles queued messages in order until Stop, then whatever is left for up
// to drainLimit.
func (in *Ingestor) work() {
	defer close(in.worked)
	var lag lagReport
	for {
		select {
		case a := <-in.queue:
			in.take(a, &lag)
			continue
		case <-in.done:
		}
		break
	}
	deadline := time.Now().Add(drainLimit)
	for time.Now().Before(deadline) {
		select {
		case a := <-in.queue:
			in.take(a, &lag)
		default:
			return
		}
	}
	in.log.Warn("ingest stopped with messages still queued", "broker", in.cfg.Broker, "dropped", len(in.queue))
}

func (in *Ingestor) take(a arrival, lag *lagReport) {
	in.queuedBytes.Add(-int64(len(a.msg.Payload())))
	switch waited := time.Since(a.at); lag.observe(waited) {
	case lagStarted:
		in.log.Warn("ingest is behind; messages are queued", "broker", in.cfg.Broker,
			"behind", waited.Round(time.Second), "queued", len(in.queue))
	case lagEnded:
		in.log.Info("ingest caught up", "broker", in.cfg.Broker,
			"peakBehind", lag.lastPeak.Round(time.Second))
	}
	if n := in.queueFull.Swap(0); n > 0 {
		in.log.Error("ingest queue full, messages dropped", "broker", in.cfg.Broker, "dropped", n)
	}
	in.handle(a.msg, a.at)
}

// lagReport turns per-message waits into one log line when the worker falls
// behind and one when it has caught up.
type lagReport struct {
	behind         bool
	peak, lastPeak time.Duration
}

type lagChange int

const (
	lagNone lagChange = iota
	lagStarted
	lagEnded
)

func (l *lagReport) observe(waited time.Duration) lagChange {
	if l.behind {
		l.peak = max(l.peak, waited)
		if waited > time.Second {
			return lagNone
		}
		l.behind, l.lastPeak, l.peak = false, l.peak, 0
		return lagEnded
	}
	if waited < queueLagWarn {
		return lagNone
	}
	l.behind, l.peak = true, waited
	return lagStarted
}
