# RF bridge detection — design

How Ridgeline finds RF bridges: nodes that carry traffic between the mesh it
observes and a mesh on another frequency. Written 2026-07-19 after the original
detector failed to find a live, known bridge on the dev instance.

A bridge is not inherently hostile — the one this work was validated against is
the operator's own, deliberate and useful. The goal is to *identify* ingress
points so an operator can decide, not to auto-ban.

## Evidence base

Measured over 14 days on the dev instance (243 nodes, 16 observers, ~900k
observations). Known ground truth:

| Case | Truth |
|---|---|
| `KOD - Cokley 6` → `KOD - Cokley 4` | **bridge** — 910.425 ⇄ 910.625, serial-linked pair |
| `Dager-Mesh-Austin` | **not a bridge** — the original detector's top candidate |
| `Str8Peter SenseCap` | **not a bridge** — a relay serving an observer-less pocket; the captivity rule's core false-positive class |
| `Yaletown Repeater` | **migration** — moved .425 → .625, same pubkey |
| `KOD - Craig St` / "Craig st 6" | **migration** — renamed, same pubkey, now far side |
| `SteelHeadBC-1watt` | **migration** — another operator, temporary .625 test |

## Why the original detector failed

`DetectInjection` classified an origin as foreign when it was never heard at zero
hops, called it *captive* to a relay when ≥95% of its paths transited that relay,
and flagged a relay with ≥3 captive nodes forming ≥60% of its foreign traffic.

Four independent failures, each sufficient alone:

**1. "Never heard directly" carries almost no information.** 263 of 316 origins
(83%) qualify, and 90+ active relays with zero direct receptions are plainly
legitimate (Galiano, MERCS Sumas, Reginald Hill). With 16 observers over this
footprint, being out of direct earshot is the normal condition, not a signature.

**2. Adverts only.** The scan skips every packet where `pkt.Advert == nil`. A
companion that never adverts is structurally invisible — one far-side companion
was observed only through channel messages, with 13 observer copies carrying full
path evidence that the detector discarded. Most of a bridge's evidence is thrown
away before scoring.

**3. `minCaptiveNodes ≥ 3` fails on small far sides.** Only two far-side nodes
adverted in the window, so the bridge was below threshold by construction.

**4. Unverified adverts create phantom origins.** A corrupt public key invents a
node that never existed. These are rare — measured over 24h on the dev mesh, 8 of
7,857 adverts fail Ed25519 verification (0.1%) — but they concentrate in the
injector rule, where a handful of one-off phantom keys reported by a single
observer is exactly the "sole source of many origins" signature. Removing them
eliminated a standing injector false positive. `Advert.SignatureValid` is computed
by the decoder and was never consulted.

Note the signature covers the advert *payload*, not the path: the path is mutable
by design, since relays append to it. So signature validation cannot vouch for
route data, only for the originator's identity and payload.

## The signal that works: downstream determinism

Across 138 relays, the median hands off to **13 distinct next hops** with a **44%
top share**. RF is broadcast — whoever hears you first varies, so downstream
choice is high-entropy.

`KOD - Cokley 6` handed off **1,417 times to exactly one node, and never to
anything else** (entropy 0.00).

That is the physical signature of a bridge: its egress is a wire, not an antenna.

Score each relay on the entropy of its next-hop distribution, built from **all
packet types**, weighted by sample count:

    H(relay) = -Σ p(next) log₂ p(next)

Confidence scales with the number of samples observed without any alternative
appearing. 1,417 samples with zero alternatives is overwhelming; 100 is weak.

**Determinism alone is not proof.** A relay with exactly one physical neighbour
looks identical to a wire. Three other nodes on this mesh reach 100% determinism
(`Willoughby` 535 samples, `425.ve7gnr` 178, `NW Dublin at 14th` 154) and a fourth
sits at 98%. They are an order of magnitude below Cokley 6 in sample count, which
is what confidence weighting is for. The output is a ranked shortlist for review,
not a verdict.

### Corroborator: asymmetric pair

A bridge joins a zero-entropy node to a high-entropy one — the wire feeds a radio
that broadcasts normally. `Cokley 6 (H=0) → Cokley 4 (radiates into .425)` fits.
Two isolated relays chained by RF would not.

## Time-aware side classification

A pubkey survives a frequency change, so "is this node local?" is a property of a
node *during an interval*, not of the node. Yaletown's last direct .425 reception
was 18:13:21 and its first arrival via the bridge 21:21:40 — a clean transition
with no overlap. Under a window-wide boolean it reads "heard directly" and is
excluded from the far-side population by evidence that expired hours earlier.

Track **last direct reception** and **last bridge-entry reception** per node and
compare them. Yaletown then reads "currently far side" at any window size, with no
tuning. This is what the manual 1h/6h/24h/3d window selector was working around.

Surface the **changepoint** as a first-class event — an operator wants to know a
node moved. A clean changepoint means migration; continuous interleaving of both
modes means dual-homing or a spoofed key. Same statistic, different verdict.

## Hygiene

- Gate advert-derived facts on `SignatureValid` (phase 1).
- Treat unresolvable hops as **unknown**, not absent. 6,012 one-byte hops were
  unresolvable over 7 days; a node whose evidence is mostly 1-byte hops deserves
  lower confidence, not silent undercounting.
- Never rank on geography. An overlapping bridge is as real as a distant one.

## Registry

Let an operator mark a node as a sanctioned bridge — ideally tied to node claims,
since ownership is already provable over RF. A known bridge then renders as such
instead of recurring as a candidate. Same principle as the purge policy: a claim
beats a heuristic.

## Validation set

Any change is measured against the table at the top before shipping. Previously
there was no way to tell whether a tweak helped.

## Phasing

1. **Signature gate** — reject adverts failing Ed25519 verification.
2. **All-payload path extraction** — decouple path evidence from advert decoding.
3. **Determinism scoring** — behind the existing window selector, so old and new
   can be compared on identical data.
4. **Recency + changepoint** classification.
5. **Known-bridge registry.**

Findings surface in the admin console; no automatic notifications.
