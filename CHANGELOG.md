# Changelog

Notable changes to Ridgeline. This project follows
[Semantic Versioning](https://semver.org/); tagging began at v0.1.0 (earlier
history lives in the git log).

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
