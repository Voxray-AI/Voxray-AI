# What Can Connect to a Deployed Voila Server

When you deploy voila-go, **other applications** and **voice streams** connect over the transports you enable in config. Each connection is isolated (one goroutine per connection) and runs the same pipeline (e.g. voice: VAD → STT → LLM → TTS).

---

## 1. Entry points (what can connect)

| What connects | How | Endpoint(s) | Config |
|---------------|-----|-------------|--------|
| **Web / mobile apps (WebSocket)** | Any client that speaks JSON (or RTVI) over WebSocket | `GET /ws` (e.g. `wss://your-server/ws`) | `transport`: `websocket` or `both` |
| **Web / mobile apps (WebRTC)** | Browser or app that does SDP offer/answer | `POST /webrtc/offer` with JSON `{"offer": "<sdp>"}` | `transport`: `smallwebrtc` or `both` |
| **Runner-style clients** | Session then WebRTC | `POST /start` → then `POST /sessions/{id}/api/offer` with SDP | Same as WebRTC; needs `session_store` (memory or Redis) |
| **Daily.co room clients** | Users join a Daily room; room connects to your server via WebRTC | `GET /` (redirect to room), then session offer | `runner_transport`: `daily` |
| **Telephony (PSTN) voice** | Twilio/Telnyx/Plivo/Exotel send call to your webhook; media over WebSocket | `POST /` (XML webhook), `GET /telephony/ws` (media) | `runner_transport`: `twilio` \| `telnyx` \| `plivo` \| `exotel` |

So **other applications** = anything that can open a WebSocket to `/ws` or post an SDP offer to `/webrtc/offer` (or use `/start` + `/sessions/...`). **Voice streams** = that same WebSocket/WebRTC (for in-app voice) or telephony providers (for phone calls).

---

## 2. Wire format and compatibility

- **WebSocket `/ws`**
  - Default: **JSON** envelope (`type` + `data`) — any app that sends/receives these frames can connect.
  - Optional: **RTVI** protocol when connecting with `?rtvi=1` (or similar) — used by RTVI clients; see [ARCHITECTURE.md](./ARCHITECTURE.md) and FRAMEWORKS.md.
  - **Wire compatibility:** Connect with `?format=protobuf` to use binary frame format (same protobuf message names/fields). External clients can use this for interoperability. The server uses `ProtobufSerializer` from `pkg/frames/serialize`.
- **Telephony**  
  Provider-specific serializers (Twilio, Telnyx, Plivo, Exotel, etc.) — the server uses the right one when the connection is identified as that provider.

So "other applications" and "voice streams" use the same endpoints; the only difference is whether the client is a custom app (WebSocket/WebRTC), a Daily client, or a telephony carrier.

---

## 3. One connection = one voice session

From [SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md) and [ARCHITECTURE.md](./ARCHITECTURE.md):

- Each new connection (one WebSocket, one WebRTC session, or one telephony call) gets a new **Transport** and a new **Runner** (goroutine).
- The runner wires that transport to the **same pipeline** (voice or plugin-based) and pushes frames (e.g. audio in → STT → LLM → TTS → audio out).
- So: **multiple applications or voice streams** = multiple concurrent connections to the same server; each is independent.

---

## 4. Deployment choices that affect who can connect

- **Port and host:** Set `port` and `host` (e.g. `0.0.0.0`) in `config.json` (or `VOILA_PORT` / `VOILA_HOST`). Clients connect to `http(s)://host:port`.
- **Transport:** `transport: "websocket"` → only `/ws`; `"smallwebrtc"` → only `POST /webrtc/offer`; `"both"` → both. So "other applications" that use WebSocket need `websocket` or `both`.
- **Runner mode:** For runner-style or Daily you need WebRTC and optionally `runner_transport=daily`; for telephony you need `runner_transport=twilio|telnyx|plivo|exotel` (and telephony providers will hit `POST /` and `/telephony/ws`).
- **TLS:** Use TLS (or a reverse proxy with TLS) so clients can use `wss://` and HTTPS. See [DEPLOYMENT.md](./DEPLOYMENT.md).
- **CORS:** If browsers on other origins connect to `/ws` or your web UI, set `cors_allowed_origins` (or `VOILA_CORS_ORIGINS`) so the server allows those origins.
- **Auth:** Optional `server_api_key`; then clients must send the key (e.g. `Authorization: Bearer <key>` or `X-API-Key: <key>`) on WebSocket upgrade and on `/webrtc/offer`.

---

## 5. Summary

- **Other applications** can connect by:
  - **WebSocket:** open `wss://your-server/ws` and send/receive JSON (or RTVI with `?rtvi=1`, or binary protobuf with `?format=protobuf`).
  - **WebRTC:** POST SDP offer to `https://your-server/webrtc/offer` (or use `POST /start` then `POST /sessions/{id}/api/offer`).
- **Voice streams** are the same: each stream is one of those connections (WebSocket, WebRTC, or telephony WebSocket after provider webhook).
- Enable the right `transport` and optional `runner_transport` in config so the endpoints you need are active; use TLS, CORS, and optional API key so only intended applications and voice streams can connect.

---

## References

- [ARCHITECTURE.md](./ARCHITECTURE.md) — components and data flow.
- [SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md) — system view and entry-point table.
- [DEPLOYMENT.md](./DEPLOYMENT.md) — production deployment, TLS, scaling, security.
