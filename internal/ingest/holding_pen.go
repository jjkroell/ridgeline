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

	// holdTTL is how long packets wait for a status that may never come, after
	// which the publisher is quarantined outright. Status arrives every ~5
	// minutes on this mesh, so twenty is four missed cycles: long enough that a
	// slow or briefly-offline receiver is never caught, short enough that a
	// publisher which will never identify itself is dealt with the same hour.
	//
	// Reaching this is not a wrong answer, it is no answer — and a publisher
	// that streams packets for twenty minutes without once saying what radio it
	// is on cannot be vouched for by anything.
	holdTTL = 20 * time.Minute

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
	dropped int    // packets refused because the pen was full
	name    string // last friendly name seen, for the log and the quarantine row
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
	if obs.ObserverName != "" {
		p.name = obs.ObserverName
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
			type waiting struct {
				id   string
				name string
				n    int
				age  time.Duration
			}
			var expired, still []waiting
			in.penMu.Lock()
			for id, p := range in.pens {
				w := waiting{id: id, name: p.name, n: len(p.items), age: time.Since(p.opened)}
				if w.age > holdTTL {
					expired = append(expired, w)
				} else {
					still = append(still, w)
				}
			}
			in.penMu.Unlock()

			// Anything still inside its grace period is reported rather than left
			// invisible: until a publisher is quarantined it has no observer row,
			// so this log line is the only trace it exists.
			for _, w := range still {
				in.log.Info("observer awaiting its first status; packets held",
					"observer", w.id, "name", w.name, "held", w.n,
					"waiting", w.age.Round(time.Second), "deadline", holdTTL)
			}

			for _, w := range expired {
				now := time.Now().UTC().Format(time.RFC3339Nano)
				if err := in.store.QuarantineSilentObserver(w.id, w.name, now); err != nil {
					in.log.Error("quarantine silent observer failed", "observer", w.id, "err", err)
				}
				in.discardHeld(w.id, "no status within "+holdTTL.String()+" — quarantined")
				in.log.Warn("observer quarantined: published for the whole grace period without ever reporting a radio preset",
					"observer", w.id, "name", w.name, "discarded", w.n, "grace", holdTTL)
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
