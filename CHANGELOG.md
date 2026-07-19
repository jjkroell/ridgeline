# Changelog

Notable changes to Ridgeline. This project follows
[Semantic Versioning](https://semver.org/); tagging began at v0.1.0 (earlier
history lives in the git log).

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
