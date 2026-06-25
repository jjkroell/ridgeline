# Ridgeline VM deployment & CoreScope cutover

Move Ridgeline onto the VM (`ve7kod@lnuvm159.ubc.bcwarn.net`), replacing CoreScope.

## Target architecture

All public traffic arrives through the VM's existing **Cloudflare Tunnel**
(`cloudflared`, token-managed — routes live in the Cloudflare dashboard, not a
local file). Cloudflare terminates TLS at the edge; the origin serves plain HTTP.

```
ridgeline.ve7kod.ca  → CF edge (TLS) → tunnel → localhost:8088 → caddy → ridgelined:8080
mqtt-dev.ve7kod.ca   → CF edge (TLS) → tunnel → localhost:9001 → mosquitto (websockets, anonymous)
```

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
  --exclude 'web/.svelte-kit' --exclude 'data' --exclude '*.db*' \
  --exclude 'config.dev.json' --exclude 'config.prod.json' \
  /home/jesse/ridgeline/ ve7kod@lnuvm159.ubc.bcwarn.net:~/ridgeline/
```

### 2. Create the prod config on the VM
```bash
ssh ve7kod@lnuvm159.ubc.bcwarn.net
cd ~/ridgeline/deploy
cp config.example.json config.json
# set "adminToken" to a real secret (reuse the local config.prod.json token if you want parity)
mkdir -p data
```

### 3. Migrate the database safely (no writes during copy)
On the **local** machine — stop the daemon so the WAL checkpoints into the .db,
then copy the quiesced files (the -wal/-shm may be absent after a clean stop;
copy whatever exists):
```bash
systemctl --user stop ridgeline
rsync -az /home/jesse/ridgeline/data/ridgeline.db* \
  ve7kod@lnuvm159.ubc.bcwarn.net:~/ridgeline/deploy/data/
```
(Leave the local daemon stopped — it's being retired. Restart it only to roll back.)

### 4. Build the image on the VM (still safe — old stack keeps running)
```bash
cd ~/ridgeline/deploy
docker compose build
```

### 5. Cut over — stop CoreScope, free the ports, start Ridgeline
CoreScope's stack holds 80/443/1883/9001; taking it down frees 9001 + the rest
for Ridgeline. Brief observer gap until the tunnel route is switched (step 6).
```bash
cd ~/CoreScope && docker compose down        # stops corescope-master/umami/db/mosquitto
cd ~/ridgeline/deploy && docker compose up -d
docker compose ps
curl -s localhost:8088/api/stats             # expect your migrated node/observer counts
curl -s localhost:8088/ | grep -o '<title>[^<]*</title>'
docker compose logs -f ridgelined            # watch for ingest once the tunnel is switched
```

### 6. Switch the Cloudflare Tunnel routes — **DASHBOARD (manual)**
Cloudflare dashboard → Zero Trust → Networks → Tunnels → the VM's tunnel →
**Public Hostnames**:
- **Add** `ridgeline.ve7kod.ca` → `HTTP` → `localhost:8088`
  (this hostname currently routes to the *local machine's* tunnel — remove it
  there, or it will keep resolving to the old box).
- **Add** `mqtt-dev.ve7kod.ca` → `HTTP` → `localhost:9001`
- **Remove** the retired CoreScope hostnames (`analyzer.ve7kod.ca`,
  `mqtt.ve7kod.ca`, `425.ve7kod.ca`) once you've confirmed Ridgeline is healthy.

Then verify publicly: `https://ridgeline.ve7kod.ca` loads, `/api/live` WS connects,
and observers (pointed at `mqtt-dev.ve7kod.ca`) start landing packets in the logs.

### 7. Decommission CoreScope (after Ridgeline is confirmed healthy)
```bash
cd ~/CoreScope && docker compose down -v      # drop CoreScope volumes (umami db etc.)
docker image rm corescope:latest 2>/dev/null; docker image prune -f
rm -rf ~/CoreScope ~/meshcore-data            # reclaims ~5.5G — only after you're satisfied
```
Local box: `systemctl --user disable --now ridgeline`, and remove the
`ridgeline.ve7kod.ca` hostname from the local cloudflared tunnel.

## Deploying updates later
```bash
# from local: rsync the repo (step 1), then on the VM:
cd ~/ridgeline/deploy && docker compose up -d --build
```

## Rollback (before step 7)
`cd ~/ridgeline/deploy && docker compose down`, `cd ~/CoreScope && docker compose up -d`,
point the tunnel hostnames back, and `systemctl --user start ridgeline` locally.
Nothing is destroyed until step 7.
