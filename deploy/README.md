# Ridgeline VM deployment & CoreScope cutover

Move Ridgeline onto the VM (`ve7kod@lnuvm159.ubc.bcwarn.net`), replacing CoreScope.

## Target architecture

All public traffic arrives through the VM's existing **Cloudflare Tunnel**
(`cloudflared`, token-managed — routes live in the Cloudflare dashboard, not a
local file). Cloudflare terminates TLS at the edge; the origin serves plain HTTP.

```
ridgeline.ve7kod.ca  → CF edge (TLS) → tunnel → localhost:8088 → caddy → ridgelined:8080
mqtt-dev.ve7kod.ca   → CF edge (TLS) → tunnel → localhost:9101 → mosquitto (websockets, anonymous)
```

Ridgeline binds **8088** (web) and **9101** (broker) so it runs alongside CoreScope
(80/443/1883/9001) with no port clash — a parallel transition, not a hard cut.
CoreScope is torn down only in step 7, after Ridgeline is confirmed healthy.

Compose stack (`docker compose`, this directory): `mosquitto` + `ridgelined` + `caddy`.
- Broker is **open / anonymous** (no observer auth), matching the `ve7kod-dev`
  observer config (websockets, edge TLS, no `[broker.auth]`).
- `ridgelined` runs as **uid 1000** so the bind-mounted DB dir is writable with
  no chown. Confirm the VM user is uid 1000 (`id -u`); adjust `user:` in
  `docker-compose.yml` if not.

## Image build

Multi-stage `Dockerfile` (repo root): node builds the SvelteKit SPA, golang
builds the pure-Go binary (CGO off — modernc sqlite), distroless runtime. Build
context is the repo root. Validated to build + run end-to-end locally.

---

## Cutover runbook

Prep (steps 1–3) is non-destructive. The destructive cut is step 5.

### 1. Get the source onto the VM
From the local repo (excludes git/node_modules/build/secrets via rsync filters):
```bash
rsync -az --delete \
  --exclude '.git' --exclude '**/node_modules' --exclude 'web/build' \
  --exclude 'web/.svelte-kit' --exclude 'data' --exclude '*.db' \
  --exclude '*.db-wal' --exclude '*.db-shm' \
  --exclude 'config.dev.json' --exclude 'config.prod.json' --exclude '/ridgelined' \
  --exclude 'deploy/config.json' --exclude 'deploy/.env' --exclude 'deploy/data' \
  /home/jesse/ridgeline/ ve7kod@lnuvm159.ubc.bcwarn.net:ridgeline/
```
> Two non-obvious excludes, both learned the hard way:
> - `/ridgelined` is anchored (leading slash) so it skips only the root build
>   binary — an unanchored `ridgelined` also matches the `cmd/ridgelined/` dir
>   and breaks the Go build.
> - `deploy/config.json`, `deploy/.env`, `deploy/data` are VM-only runtime files;
>   without excluding them, `--delete` wipes them on every re-sync.

### 2. Create the prod config on the VM
```bash
ssh ve7kod@lnuvm159.ubc.bcwarn.net
cd ~/ridgeline/deploy
cp config.example.json config.json
# set "adminToken" to a real secret (reuse the local config.prod.json token if you want parity)
mkdir -p data
# capture this box's uid/gid so the container can write the bind-mounted DB dir
printf 'RIDGELINE_UID=%s\nRIDGELINE_GID=%s\n' "$(id -u)" "$(id -g)" > .env
```
> The `.env` is required: `ve7kod` is uid **1001**, and the container must run as
> that id to write `./data`. The compose `user:` reads `RIDGELINE_UID/GID` from it.

### 3. Migrate the database safely (no writes during copy)
On the **local** machine — stop the daemon so the WAL checkpoints into the .db,
copy the quiesced files (the -wal/-shm may be absent after a clean stop; copy
whatever exists), then **restart the local daemon** so this box stays an intact,
live fallback:
```bash
systemctl --user stop ridgeline
rsync -az /home/jesse/ridgeline/data/ridgeline.db* \
  ve7kod@lnuvm159.ubc.bcwarn.net:~/ridgeline/deploy/data/
systemctl --user start ridgeline   # local stays a clean rollback copy
```
The copy is read-only against the local DB — the local box is never mutated.

### 4. Build the image on the VM (still safe — old stack keeps running)
```bash
cd ~/ridgeline/deploy
docker compose build
```

### 5. Bring Ridgeline up — alongside CoreScope (no downtime)
Ridgeline's ports (8088/9101) don't clash with CoreScope (80/443/1883/9001), so
both run in parallel. CoreScope keeps serving until you're satisfied (step 7).
```bash
cd ~/ridgeline/deploy && docker compose up -d
docker compose ps
curl -s localhost:8088/api/stats             # expect your migrated node/observer counts
curl -s localhost:8088/ | grep -o '<title>[^<]*</title>'
docker compose logs -f ridgelined            # watch for ingest once the tunnel + observers point here
```

### 6. Add the Cloudflare Tunnel routes — **DASHBOARD (manual)**
Cloudflare dashboard → Zero Trust → Networks → Tunnels → the VM's tunnel →
**Public Hostnames** (these are NEW hostnames, so adding them doesn't disturb the
running CoreScope routes):
- **Add** `ridgeline.ve7kod.ca` → `HTTP` → `localhost:8088`
  (this hostname currently routes to the *local machine's* tunnel — remove it
  there, or it will keep resolving to the old box).
- **Add** `mqtt-dev.ve7kod.ca` → `HTTP` → `localhost:9101`

Then verify publicly: `https://ridgeline.ve7kod.ca` loads, `/api/live` WS connects,
and observers (pointed at `mqtt-dev.ve7kod.ca`) start landing packets in the logs.
Migrate observers to `mqtt-dev.ve7kod.ca` at your own pace — CoreScope's broker
stays up until step 7.

### 7. Decommission CoreScope (after Ridgeline is confirmed healthy)
```bash
cd ~/CoreScope && docker compose down -v      # drop CoreScope volumes (umami db etc.)
docker image rm corescope:latest 2>/dev/null; docker image prune -f
rm -rf ~/CoreScope ~/meshcore-data            # reclaims ~5.5G — only after you're satisfied
```
Cloudflare dashboard: **remove** the retired CoreScope hostnames
(`analyzer.ve7kod.ca`, `mqtt.ve7kod.ca`, `425.ve7kod.ca`).
Local box: keep it intact as a rollback until you're fully confident; when ready,
`systemctl --user disable --now ridgeline` and remove `ridgeline.ve7kod.ca` from
the local cloudflared tunnel.

## Deploying updates later
```bash
# from local: rsync the repo (step 1), then on the VM:
cd ~/ridgeline/deploy && docker compose up -d --build
```

## Rollback (before step 7)
`cd ~/ridgeline/deploy && docker compose down`, `cd ~/CoreScope && docker compose up -d`,
point the tunnel hostnames back, and `systemctl --user start ridgeline` locally.
Nothing is destroyed until step 7.
