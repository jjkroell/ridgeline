# Changelog

Notable changes to Ridgeline. This project follows
[Semantic Versioning](https://semver.org/); tagging began at v0.1.0 (earlier
history lives in the git log).

## [v0.6.1] — 2026-08-02

### Changed
- **Channels and Map swapped places in the navigation.** Channels moves up to
  sit directly under Feed, and the static Map drops to where Channels was,
  between Topology and Identity. Applied to both the desktop sidebar (and its
  narrow-screen header nav, which renders the same list) and the mobile More
  sheet, so the two layouts stay in the same order.

## [v0.6.0] — 2026-07-28

### Added
- **Five colour themes, replacing the light/dark toggle.** Ridgeline (the
  default dark), Slate (deep blueprint navy, cyan accent), Graphite (warm
  near-black brown, brass accent), Paper (warm topographic light) and Mist (cool
  blue-grey light, indigo accent). A picker in the sidebar and the mobile More
  sheet previews each theme with its own ground and accent colour. Every palette
  clears WCAG AA for body and dimmed text, and 3:1 for the faint tier and
  accents, in both light and dark.
- **Ago/Clock timestamp toggle on the live feed.** Wall-clock time was
  previously reachable only by opening a row's detail modal. A segmented control
  in the toolbar switches every row between elapsed and clock time, on desktop
  and mobile, persisted under `ridgeline-time-mode`.

### Fixed
- **Relative timestamps no longer go stale.** `ago()` was computed during
  render, so a label only refreshed when something else re-rendered the feed —
  which arriving packets happened to do. Pausing the feed, or a quiet mesh, left
  every "2m" frozen. The store now owns a 10s clock.
- **Tooltips are anchored by their measured width, not `max-w-[250px]`.** Any
  tooltip whose trigger centred within 133px of a viewport edge was pushed to a
  fixed position; in the sidebar that collapsed a whole row of controls onto one
  shared spot. The edge-overflow protection it existed for is unchanged.
- **The feed's Time column widens for clock timestamps** — it was a fixed 34px,
  sized for "2m", and a wall clock overflowed into the Type badge.

### Internal
- Themes are selected by `data-theme` on `<html>` (values mutually exclusive, so
  blocks never compete on specificity). The `theme-light` class now only marks a
  light base and is what `map-util.isLight()` reads, so all basemap, hillshade
  and Leaflet call sites were untouched by going from two themes to five.
  `theme.mode` is gone: call sites use `theme.id` and `theme.isLight`.

## [v0.5.5] — 2026-07-20

### Fixed
- **A trace's header path is no longer counted as relay hops.** Trace packets
  are not shaped like the rest: their header path carries one signed SNR
  reading per hop rather than a list of relay hashes, and the route being
  traced lives in the payload, sized independently. Those SNR bytes are
  indistinguishable from 1-byte hop hashes, so every path walker was reading
  them as relays — on a live mesh, 40% of them coincidentally matched a known
  node's prefix, inventing a relay that never carried the packet and adjacency
  between nodes that were never neighbours. This fed relay counts, neighbour
  resolution, the activity heatmap, mesh topology, node history, observer
  direct-link detection and bridge detection; it also let a phantom hop keep a
  silent node alive through the retention sweep, and could drop a packet that
  never crossed a quarantined bridge. Trace is a small share of traffic, so
  mesh-wide conclusions are unchanged — the correction is per-node: a node no
  longer shows a relay it never performed.

## [v0.5.4] — 2026-07-20

### Fixed
- **Observers are shown by name again.** Keying observers by public key in
  v0.5.3 made the key their id, and the UI renders ids — so the observers list,
  both detail pages, the heard-by lists on node detail, the retired panel, and
  the analytics coverage list and direct-link graph all displayed 64 hex
  characters where a name belongs. The key is the right identity and the wrong
  label. Every observer surface now shows the name, falling back to the id only
  for an observer that never carried a key (where the id *is* its name). The
  detail pages keep the key visible but quiet beneath the heading — it is what
  the observer's MQTT topic is keyed by.

## [v0.5.3] — 2026-07-19

Observers are now identified by their public key, not their name.

### Changed
- **An observer's identity is its public key; the name is a label.** The name is
  something the operator changes at will, so using it as the identity meant a
  rename started a whole new observer: every packet and telemetry sample stayed
  under the old name and the renamed one began from nothing. The name is not
  reliably distinct either — a device publishing `"Foo "` and `"Foo"` was two
  observers. The MQTT topic carries the public key on every message, so it is
  always available and survives any number of renames.
- **Existing data is re-keyed once, on upgrade.** History recorded under each old
  name is repointed at that observer's key, rows that were separate identities
  only because of a rename are merged, and blocklist entries follow so a
  quarantined observer does not silently come off the list. Merging takes the
  observer's current identity — label, status, region, radio, and whether it is
  retired — from its most recent row, so a receiver retired under an old name and
  since returned to the air is not left hidden. Observers whose key is unknown
  stay keyed by name; there is nothing better, and dropping them would lose
  their history.
- **Names are resolved server-side** for the live feed, node history, per-node
  observer lists and mesh analytics, so a name renders even for an observer that
  has since been retired.

### Known issue
- Telemetry stranded by a rename whose old observer row was already deleted
  cannot be re-attached: nothing records which key that name belonged to. Renames
  from here on strand nothing.

## [v0.5.2] — 2026-07-19

### Fixed
- **Deleting an observer now removes its device telemetry too.** The
  battery/noise series is keyed by observer id and nothing else references it,
  so deleting the observer stranded every sample it had ever recorded — rows no
  page could reach and no sweep collected, for every observer ever deleted. The
  delete dialog said "all of its stored packets"; it now says what it does, and
  reports the telemetry rows removed.

### Known issue
- Observer identity is the friendly name, so **renaming an observer strands its
  telemetry** under the old name and starts a fresh series. This is separate
  from the fix above and is not addressed here — a rename is not a deletion, and
  the stranded samples are real measurements worth keeping until observers are
  keyed by something stable.

## [v0.5.1] — 2026-07-19

Decommissioned observers no longer come back from the dead.

### Fixed
- **A retired observer stays retired.** Observers publish their `/status` with
  the MQTT retain flag, so the broker keeps that message and replays it to the
  daemon on *every* reconnect — for as long as it exists, whether or not the
  device is still on the air. The daemon treated a replay as a live sighting and
  re-created the observer, which is why one deleted from the observers page
  reappeared after the next restart or redeploy. A retained status is a stale
  last-known value: it may now refresh an observer that already exists, but it
  can never create one.
- **No more invented telemetry.** The same replay appended a battery/noise
  sample stamped with the reconnect time — a reading that was never taken, one
  per reconnect, for as long as the retained message lived.

### Added
- **Retire an observer** instead of deleting it. Retiring withdraws a
  decommissioned receiver from the observers page and keeps every packet it
  reported, still attributed to it in history. Deleting an observer removes its
  packets too, which quietly rewrites the record — retiring is the right action
  for a receiver that has simply left the network. Reversible from the admin
  console, on desktop and mobile.

## [v0.5.0] — 2026-07-19

RF bridge detection, rebuilt. The previous detector could not find a live bridge
on the mesh it was written for; this release finds it, explains why it was
missed, and keeps the operator's own bridge from being reported as news forever.

### Added
- **A second detection signal: wired egress.** RF is broadcast, so which
  neighbour relays a packet next varies — a typical relay hands off to ~13
  different nodes. A relay whose next hop *never* varies is handing off over a
  cable. This finds a bridge however few nodes sit behind it; the old rule needed
  three and could not see a small far side at all. Both rules now run and every
  candidate is labelled with the signal(s) that produced it.
- **Moved behind a bridge.** Nodes that stopped being heard directly and now
  arrive through a bridge are reported in their own right. A node keeps its
  public key across a frequency change, so nothing else notices it moved.
- **Known bridges.** Mark a bridge you run on purpose: it moves to its own list
  and stops appearing as a candidate. Nothing is blocked or hidden — the
  opposite of Dismiss, which says a candidate is not a bridge.
- **Scan summary and per-candidate evidence** in the admin console: packets
  scanned, paths, unresolved hops, adverts rejected, and for each candidate the
  traffic it carried, how many distinct next hops it had, and whether an observer
  ever received its own transmission.

### Fixed
- **Path evidence now comes from every packet type, not just adverts.** A route
  is in the clear whatever the payload; only the *origin* needs an advert. A
  companion that never adverts previously contributed nothing at all despite its
  messages crossing a bridge with a full path attached — on the reference mesh
  this raised the evidence base from 1,699 adverts to 5,667 packets per window.
- **Side membership is judged on recent evidence.** A single direct reception
  anywhere in the window used to mark a node local for the whole window, so a
  node that moved kept being excused by evidence that had expired hours earlier.
- **Adverts whose signature does not verify are rejected.** A corrupt public key
  invents a node that never existed; those phantoms were surfacing as injector
  candidates.
- **Ordinary repeaters no longer flagged as bridges.** A single unvarying next
  hop is also what a repeater with exactly one reachable neighbour looks like;
  a candidate must now actually carry a far side.
- **Admin console** only renders sections that hold something, and the Known
  action is no longer adjacent to the show/hide toggle.

## [v0.4.0] — 2026-07-19

### Added
- **Claimed filter on the Nodes list**, showing your own nodes first and other
  operators' after. It joins the role and favorites filters in a new filter
  modal, replacing the row of pills that competed with the table: the header is
  now a search field and one control that names the active filters
  ("Repeaters · Claimed") so the constraint is legible at a glance.
- **Member management on mobile.** `/m/admin` gained the MEMBERS panel — promote
  or demote admins, block, unblock and remove — matching the desktop console.
  Both now share one component.
- **Dormant claims and shares.** A claim or location share outlives its node when
  the retention sweep prunes a node that has gone quiet. Those rows now render
  un-linked with a "Dormant" pill explaining the claim is kept and reconnects if
  the node advertises again, instead of linking to a "Node not found" page.

### Fixed
- **Scrubbing a node now removes the data attached to it** — the ownership claim,
  notes, private location and location shares. Previously they were orphaned: the
  claim still showed in "Claimed Nodes" pointing at a node that no longer existed,
  and it would have blocked the node from ever being re-claimed. Re-scrubbing a
  key cleans up leftovers from earlier scrubs.
- **The automatic retention sweep no longer deletes that data.** A node pruned for
  going silent is expected to come back, so an operator who takes a repeater down
  for a week keeps their claim and private location.
- **Heuristic sweeps skip claimed nodes.** Neither the corruption-artifact scrub nor
  the detector-driven bridge purge will delete a node someone has claimed — a claim
  means the heuristic misfired. Purge still blocks (reversible); only the delete
  holds back, and the console reports what it skipped.
- **The live feed backfills after a reconnect.** A dropped WebSocket (redeploy,
  tunnel blip, laptop sleep, backgrounded PWA) left a permanent hole: the page
  looked connected while silently omitting everything from the outage. Channel
  conversations would simply stop updating until reload.
- **Nodes lists sort by the time they display.** Ordering used the last advert
  while the "Heard" column showed the most recent advert *or* relay, so rows could
  appear out of order.
- **The mobile "Companions" filter matched nothing** — it filtered on a role value
  no node reports.

## [v0.3.2] — 2026-07-17

### Added
- **Password reset.** A "Forgot password?" flow: request a reset link by email
  (`POST /api/auth/forgot`, always responds 200 so it never reveals whether an
  address has an account), then set a new password from the emailed single-use
  link (`/reset-password`, 1-hour expiry). Completing a reset revokes the
  account's other sessions, confirms the email address, and signs you in.
  Available on desktop and mobile.

### Security
- **Login brute-force protection.** `POST /api/auth/login` is now rate-limited per
  client IP and per target account (429 when exceeded, returned before the account
  lookup so it reveals nothing). Bursts stay generous enough for a mistyped
  password but bound sustained guessing. The reset endpoint is IP-limited too.

## [v0.3.1]

Version reserved for a public-repo-only release (an installer prompt in the
self-hostable build); no private-repo changes. See the public CHANGELOG.

## [v0.3.0] — 2026-07-17

### Added
- Account deletion now opens a prominent confirmation modal that spells out
  exactly what is removed (account, notes, private locations, shares, sessions)
  versus what remains (nodes you own are released, not deleted — kept public and
  marked "previously owned by …"). The delete button activates only after you
  re-type the account's registered email (case-insensitive) in addition to the
  password.
- Config-gated non-production banner: set `environment` (e.g. `"dev"` or
  `"staging"`) in an instance's config and the UI shows a persistent, obvious
  "not the live site" banner. It is reported via `/api/health`; unset (the
  default) shows nothing, so production instances are unaffected.

### Fixed
- The claimed-node ownership badge now recolours immediately when a claim
  verifies or a node is released, instead of only after a full page reload.
- Long values in confirmation dialogs no longer overflow the modal: dialog
  title/message wrap, and a public key (e.g. in the "scrub node" prompt) is
  shown centred in a dedicated one-line monospace slot sized to fit.

### Changed
- The daemon logs the effective email `baseURL` at startup and warns if it is
  empty while email is enabled, so a misconfigured link origin is obvious.
- Removed the hardcoded default email `baseURL`; each instance must set its own
  public origin. A hardcoded default could silently send an instance's
  verification links to the wrong origin.

## [v0.2.1] — 2026-07-17

### Security
- Cap request bodies at 64 KB via a `MaxBytesReader` middleware on all routes
  (except the `/api/live` WebSocket). Endpoint length limits were previously
  enforced only after fully decoding the JSON body, so an unbounded POST could
  buffer arbitrary memory before any check ran; the cap keeps memory bounded and
  the handler returns its usual 400.

### Tests
- `TestBodyLimit`: an oversized request body is refused while a normal-sized one
  still succeeds.

## [v0.2.0] — 2026-07-17

Security hardening (from an endpoint-authorization / email self-audit).

### Security
- Rate-limit the unauthenticated email endpoints (`POST /api/auth/register`,
  `POST /api/auth/resend-verification`), keyed by client IP and target address —
  bounds mass verification-email sends, quota burn, inbox-bombing, and account
  enumeration.
- Strip CR/LF from outgoing email headers (`To`/`From`/`Subject`) to block email
  header injection.
- Enforce same-origin on the `/api/live` WebSocket (was allow-all) to block
  cross-site WebSocket hijacking.
- Remove the dead `adminToken` config field and its misleading framing. Admin
  access is the account `is_admin` flag; the first registered account is the
  protected owner/admin. A legacy `adminToken` in an old config is ignored.

### Tests
- Rate limiter, client-IP extraction, same-origin WebSocket, and email
  header-injection tests.

## [v0.1.0] — 2026-07-17

First tagged release — the baseline of the running deployment.

### Added
- Container healthcheck: a `-healthcheck` self-probe (distroless has no shell) wired
  as a Docker `HEALTHCHECK`, so `docker ps` reports `(healthy)`.
- Build version stamping: `Dockerfile ARG VERSION` → `-ldflags -X main.version`,
  surfaced by `-version`, the startup log, and `/api/health`.
- `mqtt.username` / `mqtt.password` in the example configs.

### Fixed
- The daemon no longer blocks indefinitely when the MQTT broker is unreachable at
  startup — it serves the API/UI immediately and retries the broker in the
  background.

### Baseline
The Go daemon (MQTT ingest → MeshCore decode → SQLite → REST + WebSocket live feed)
serving the SvelteKit SPA: node/observer directories, live feed, coverage / live /
topology maps, analytics, channels; user accounts with email verification, node
claiming (rename-code or private-key signature), notes, private locations, sharing,
and self-service deletion; customizable dashboard; GDPR/PIPEDA cookie consent; and a
Docker Compose deploy (mosquitto + ridgelined + caddy). See the git history for the
pre-tag detail.
