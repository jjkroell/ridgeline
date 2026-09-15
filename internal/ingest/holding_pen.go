package ingest

import (
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// Holding pen for observers that have not yet said what radio they are on.
//
// The radio-preset guard can only judge an observer once it has sent a /status,
// and observers send one roughly every five minutes. Without this, the first few
// minutes of a brand-new observer's traffic is stored unconditionally — which is
// exactly the traffic a misconfigured or hostile receiver publishes before it
// identifies itself, and once stored it is indistinguishable from this mesh's own.
//
// So a new observer's packets are held, not stored, until its first status
// arrives. Then they are either committed in full or thrown away, depending on
// the verdict.
//
// ⚠ A pen that can grow without limit is a denial-of-service vector: anyone able
// to publish can open an observer id, never send a status, and stream packets
// into the daemon's heap. Both caps below are load-bearing, not tidiness.

const (
	// holdPerObserver caps one observer's pen. At the observed advert rates this
	// is far more than five minutes' worth, so a legitimate new observer never
	// reaches it; a flooder reaches it immediately and is capped there.
	holdPerObserver = 500

	// holdTTL is how long packets wait for a status that may never come. Status
	// arrives every ~5 minutes and the worst gap observed in a day was ~25, so
	// this is generous. An observer that publishes packets but no status is
	// broken or lying, and either way its traffic cannot be vouched for.
	holdTTL = 30 * time.Minute

	// holdSweep is how often expired pens are collected.
	holdSweep = 5 * time.Minute
)

// heldObs is an observation captured at arrival and waiting for a verdict.
type heldObs struct {
	obs store.Observation
}

type pen struct {
	items   []heldObs
	opened  time.Time
	dropped int // packets refused because the pen was full
}

// holdObservation parks an observation until this observer's preset is known.
// Returns false when the pen is full, in which case the packet is discarded —
// an unvetted observer must never be able to grow this without bound.
func (in *Ingestor) holdObservation(observerID string, obs store.Observation) bool {
	in.penMu.Lock()
	defer in.penMu.Unlock()
	if in.pens == nil {
		in.pens = map[string]*pen{}
	}
	p := in.pens[observerID]
	if p == nil {
		p = &pen{opened: time.Now()}
		in.pens[observerID] = p
	}
	if len(p.items) >= holdPerObserver {
		p.dropped++
		return false
	}
	// The observation keeps the ReceivedAt stamped at arrival. Re-stamping on
	// release would file a packet under the moment we happened to believe the
	// observer, which is not when it was heard, and would reorder it against
	// everything that arrived while it waited.
	p.items = append(p.items, heldObs{obs: obs})
	return true
}

// releaseHeld commits everything held for an observer whose preset has just been
// accepted, in arrival order.
func (in *Ingestor) releaseHeld(observerID string) {
	in.penMu.Lock()
	p := in.pens[observerID]
	delete(in.pens, observerID)
	in.penMu.Unlock()
	if p == nil || len(p.items) == 0 {
		return
	}
	var stored int
	for _, h := range p.items {
		if in.commit(h.obs) {
			stored++
		}
	}
	in.log.Info("released held packets: observer preset confirmed",
		"observer", observerID, "stored", stored, "refusedWhileFull", p.dropped,
		"heldFor", time.Since(p.opened).Round(time.Second))
}

// discardHeld throws away everything held for an observer, used when the verdict
// goes against it or its pen expires.
func (in *Ingestor) discardHeld(observerID, reason string) {
	in.penMu.Lock()
	p := in.pens[observerID]
	delete(in.pens, observerID)
	in.penMu.Unlock()
	if p == nil || len(p.items) == 0 {
		return
	}
	in.log.Warn("discarded held packets", "observer", observerID, "reason", reason,
		"packets", len(p.items), "refusedWhileFull", p.dropped,
		"heldFor", time.Since(p.opened).Round(time.Second))
}

// sweepPens expires pens whose status never arrived. Runs until ctx-less stop:
// the Ingestor owns its lifetime and this exits when Stop closes in.done.
func (in *Ingestor) sweepPens() {
	t := time.NewTicker(holdSweep)
	defer t.Stop()
	for {
		select {
		case <-in.done:
			return
		case <-t.C:
			var expired []string
			in.penMu.Lock()
			for id, p := range in.pens {
				if time.Since(p.opened) > holdTTL {
					expired = append(expired, id)
				}
			}
			in.penMu.Unlock()
			for _, id := range expired {
				in.discardHeld(id, "no status within "+holdTTL.String())
			}
		}
	}
}

// HeldCount reports how many observations are parked, for diagnostics.
func (in *Ingestor) HeldCount() (observers, packets int) {
	in.penMu.Lock()
	defer in.penMu.Unlock()
	for _, p := range in.pens {
		observers++
		packets += len(p.items)
	}
	return observers, packets
}
