# Voxray-Go Documentation

Documentation for the Voxray-Go real-time voice pipeline server.

---

## Reading order

1. **[ARCHITECTURE.md](ARCHITECTURE.md)** — Start here for the component view (CLI → Server → Transport → Pipeline → Processors), data flow, and file layout.
2. **[SYSTEM_ARCHITECTURE.md](SYSTEM_ARCHITECTURE.md)** — System view, entry points, runner modes, and deployment diagram. Or **[CONNECTIVITY.md](CONNECTIVITY.md)** if you need “what can connect” (WebSocket, WebRTC, telephony) first.
3. **As needed:** [DEPLOYMENT.md](DEPLOYMENT.md) (production), [EXTENSIONS.md](EXTENSIONS.md) (IVR, Voicemail), [FRAMEWORKS.md](FRAMEWORKS.md) (external chain, RTVI).
4. **[WEBSOCKET_SERVICES.md](WEBSOCKET_SERVICES.md)** — For implementers: WebSocket service base (reconnection, backoff).
5. **[skills/README.md](skills/README.md)** — Contributor onboarding: skills map and role-based reading order.

---

## Documentation index

| Document | Description |
|----------|-------------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Component view: CLI → Server → Transport → Pipeline → Processors; concurrency and performance (§5.2); Mermaid diagrams; data flow; file layout |
| [SYSTEM_ARCHITECTURE.md](SYSTEM_ARCHITECTURE.md) | System view: C4 context, entry points table, runner modes, deployment view, design decisions |
| [CONNECTIVITY.md](CONNECTIVITY.md) | What can connect (WebSocket, WebRTC, telephony), wire formats, deployment choices |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Production: env vars, health/ready, TLS, performance tuning, scaling, security, Docker |
| [EXTENSIONS.md](EXTENSIONS.md) | IVR and Voicemail extensions (usage, pipeline order, frames, hooks) |
| [FRAMEWORKS.md](FRAMEWORKS.md) | External chain (Langchain/Strands) and RTVI protocol |
| [WEBSOCKET_SERVICES.md](WEBSOCKET_SERVICES.md) | WebSocket service base: reconnection, backoff (for implementers) |
| [skills/README.md](skills/README.md) | Contributor onboarding: skills map and role-based reading order |

---

## API

- **[swagger.yaml](swagger.yaml)** / **[swagger.json](swagger.json)** — OpenAPI spec for the WebRTC offer endpoint (`POST /webrtc/offer`).
- Other endpoints (`/ws`, `/health`, `/ready`, `/start`, `/sessions`, etc.) are described in [SYSTEM_ARCHITECTURE.md](SYSTEM_ARCHITECTURE.md) and [DEPLOYMENT.md](DEPLOYMENT.md).
