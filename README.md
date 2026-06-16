# Ridgeline

**Mesh network observability for [MeshCore](https://meshcore.co.uk) LoRa networks.**

Ridgeline ingests packets from MeshCore observer nodes over MQTT, decodes them,
and turns them into a live picture of your mesh: nodes, links, hops, channels,
and the terrain-shaped RF reality in between.

Built for the Salish Sea / Southwest BC mesh, designed for any MeshCore region.

## Architecture

- **`cmd/ridgelined`** — single Go daemon: MQTT ingest → MeshCore packet
  decoder → SQLite → REST API + WebSocket live feed
- **`web/`** — SvelteKit (Svelte 5) single-page app, built static and served
  by the daemon; MapLibre GL maps, Tailwind CSS v4
- **SQLite** — one file, WAL mode, single writer (the daemon)

```
MeshCore observers ──MQTT──▶ ridgelined ──▶ SQLite
                                  │
                        REST + WebSocket
                                  │
                              web (Svelte)
```

## Development

```sh
# backend
go run ./cmd/ridgelined -config config.json

# frontend (dev server proxies /api to the daemon)
cd web && npm run dev
```

## Status

Early development. Written from scratch — protocol behavior referenced from
the MIT-licensed [MeshCore firmware](https://github.com/meshcore-dev/MeshCore).

## License

MIT — see [LICENSE](LICENSE).
