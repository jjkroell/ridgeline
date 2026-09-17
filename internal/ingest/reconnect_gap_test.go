package ingest

import (
	"testing"
	"time"
)

// The real 04:04–04:05 prod incident: the broker severed the connection at
// 04:04:21, the last message arrived just before, paho did not notice until the
// 45s keepalive + 15s ping timeout elapsed (04:05:22), then reconnected in ~3ms.
// The old code measured detected→reconnect and reported ~0s. The gap that maps
// to lost packets is lastMsg→reconnect ≈ 61s.
func TestReconnectGapMeasuresFromLastMessage(t *testing.T) {
	lastMsg := time.Date(2026, 9, 17, 4, 4, 21, 0, time.UTC)
	detected := time.Date(2026, 9, 17, 4, 5, 22, 0, time.UTC)
	now := detected.Add(3 * time.Millisecond)

	gap, lag := reconnectGap(now, detected, lastMsg)
	if got := gap.Round(time.Second); got != 61*time.Second {
		t.Errorf("gap = %s, want ~61s (the true outage), not the near-zero detect→reconnect", got)
	}
	if got := lag.Round(time.Second); got != 61*time.Second {
		t.Errorf("detectionLag = %s, want ~61s (the previously-invisible pre-detection silence)", got)
	}
}

// With no message ever received (startup, or an idle link), fall back to the
// detection time rather than reporting a gap since the zero time.
func TestReconnectGapFallsBackToDetection(t *testing.T) {
	detected := time.Date(2026, 9, 17, 4, 5, 22, 0, time.UTC)
	now := detected.Add(5 * time.Second)

	gap, lag := reconnectGap(now, detected, time.Time{})
	if gap != 5*time.Second {
		t.Errorf("gap = %s, want 5s measured from detection when no message is known", gap)
	}
	if lag != 0 {
		t.Errorf("detectionLag = %s, want 0 when there is no earlier last-message mark", lag)
	}
}

// A message that arrived AFTER detection (e.g. one slipped in between loss and
// the handler running) must not pull the start forward past detection.
func TestReconnectGapIgnoresLaterMessage(t *testing.T) {
	detected := time.Date(2026, 9, 17, 4, 5, 22, 0, time.UTC)
	lastMsg := detected.Add(1 * time.Second)
	now := detected.Add(5 * time.Second)

	gap, lag := reconnectGap(now, detected, lastMsg)
	if gap != 5*time.Second {
		t.Errorf("gap = %s, want 5s from detection when the last message is later", gap)
	}
	if lag != 0 {
		t.Errorf("detectionLag = %s, want 0", lag)
	}
}
