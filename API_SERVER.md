# Voxray Server API Documentation

Server-side HTTP and WebSocket API for the Voxray voice pipeline. All behavior is inferred from the codebase.

---

## 1. Overview

The API serves the Voxray voice pipeline: WebSocket and WebRTC transports for real-time audio, plus optional telephony (Twilio, Telnyx, Plivo, Exotel) and Daily.co runner modes. Each connection runs the same pipeline (e.g. VAD → STT → LLM → TTS).

| Item | Description |
|------|-------------|
| **Base URL** | Configurable via `host` and `port`. Default bind: all interfaces, port `8080`. Example: `http://localhost:8080` or `https://your-host:8080`. |
| **Versioned base path** | `/api/v1`. Versioned routes are under this prefix; legacy paths (e.g. `/start`, `/health`) remain available. |
| **Versioning** | Versioned paths: `/api/v1/health`, `/api/v1/ready`, `/api/v1/webrtc/offer`, `/api/v1/start`, `/api/v1/sessions/{id}/offer`. Legacy paths (e.g. `/health`, `/start`, `/sessions/{id}/api/offer`) are still supported. |
| **Environment** | Configuration is env-based (no explicit dev/staging/prod). Key overrides: `VOXRAY_HOST`, `VOXRAY_PORT` (or `HOST`, `PORT`), `VOXRAY_TLS_ENABLE`, `VOXRAY_TLS_CERT_FILE`, `VOXRAY_TLS_KEY_FILE`, `VOXRAY_CORS_ORIGINS`, `VOXRAY_SERVER_API_KEY`, `VOXRAY_MAX_BODY_BYTES`, `VOXRAY_LOG_LEVEL`, `VOXRAY_JSON_LOGS`, plus recording/transcripts/session-store vars. Config can be loaded from JSON and then overridden by these env vars. |

---

## 2. Authentication

| Aspect | Detail |
|--------|--------|
| **Method** | Optional **API key**. No JWT, OAuth, or refresh flow. |
| **When required** | Only when `server_api_key` (config) or `VOXRAY_SERVER_API_KEY` (env) is set. When set, required for: `POST /start`, `POST /api/v1/start`, `POST`/`PATCH` `/sessions/{id}/api/offer` and `/api/v1/sessions/{id}/offer`, `POST /webrtc/offer`, `POST /api/v1/webrtc/offer`, and WebSocket `GET /ws`. |
| **How to pass** | `Authorization: Bearer <key>` or `X-API-Key: <key>`. |
| **Expiry / refresh** | None; key is static. |
| **Failure** | `401 Unauthorized` with standard error envelope and code `UNAUTHORIZED`. |

---

## 3. Request & Response Format

| Aspect | Detail |
|--------|--------|
| **Content type** | JSON request/response: `Content-Type: application/json`. |
| **Standard headers** | Client may send `X-Request-ID`; server includes it in response `meta.requestId` or `error.requestId`. CORS-enabled routes send `Access-Control-Allow-Origin`, `Access-Control-Allow-Methods`, `Access-Control-Allow-Headers: Content-Type, Authorization, X-API-Key`. |
| **Success body** | `{ "data": <payload>, "meta": { "requestId": "<uuid>" } }`. `meta` is omitted only when not applicable (e.g. cached idempotent response). |
| **Error body** | `{ "error": { "code": "<code>", "message": "<message>", "requestId": "<id>", "details": [ { "field": "<name>", "message": "<msg>" } ] } }`. `details` is optional. |
| **Date format** | No date fields are defined in the API. |
| **Pagination** | Not used; no paginated endpoints. |
| **Body size limit** | Request body is capped by `max_request_body_bytes` (config) or `VOXRAY_MAX_BODY_BYTES`; when unset or zero, a default of 256 KB is used for safety. |

---

## 4. Error Handling

### Standard error shape

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Missing or empty offer",
    "requestId": "550e8400-e29b-41d4-a716-446655440000",
    "details": [
      { "field": "offer", "message": "required" }
    ]
  }
}
```

### HTTP status codes used

| Status | Typical code | Meaning |
|--------|----------------|--------|
| 400 | `BAD_REQUEST` | Invalid JSON, invalid request. |
| 401 | `UNAUTHORIZED` | Missing or invalid API key / webhook secret. |
| 404 | `NOT_FOUND` | Route or session not found. |
| 405 | `BAD_REQUEST` | Method not allowed. |
| 422 | `VALIDATION_ERROR` | Validation failed (e.g. missing/empty required field). |
| 500 | `INTERNAL_ERROR` | Server error (e.g. panic recovered). |
| 503 | `SERVICE_UNAVAILABLE` | Dependency unavailable (e.g. WebRTC/encoder, Redis for `/ready`). |

> **Note:** Method-not-allowed (405) responses use error code `BAD_REQUEST`; there is no dedicated code for 405.

### Error code glossary

| Code | Description |
|------|-------------|
| `BAD_REQUEST` | Malformed or invalid request; also used for 405. |
| `CONFLICT` | Reserved. |
| `FORBIDDEN` | Reserved. |
| `INTERNAL_ERROR` | Internal server error. |
| `NOT_FOUND` | Resource or route not found. |
| `RATE_LIMIT_EXCEEDED` | Reserved (rate limiting not implemented). |
| `SERVICE_UNAVAILABLE` | Service or dependency unavailable. |
| `UNAUTHORIZED` | Authentication required or failed. |
| `UNPROCESSABLE_ENTITY` | Reserved (validation uses `VALIDATION_ERROR`). |
| `VALIDATION_ERROR` | Field validation failed. |

---

## 5. Rate Limiting

**Rate limiting is not implemented.** There are no per-endpoint or per-tier limits, no `X-RateLimit-*` headers, and no retry-after semantics. The error code `RATE_LIMIT_EXCEEDED` exists for future use.

---

## 6. Endpoints

### 6.1 Health and readiness

#### `GET /health` / `GET /api/v1/health`

| Item | Detail |
|------|--------|
| **Description** | Liveness: indicates server is running. |
| **Auth** | No. |
| **Path/query/body** | None. |
| **Success** | `200 OK`. Body: `{ "data": { "status": "ok" }, "meta": { "requestId": "..." } }`. |
| **Errors** | `405` for non-GET. |

**Example request**

```bash
curl -s http://localhost:8080/api/v1/health
```

**Example response**

```json
{
  "data": { "status": "ok" },
  "meta": { "requestId": "550e8400-e29b-41d4-a716-446655440000" }
}
```

---

#### `GET /ready` / `GET /api/v1/ready`

| Item | Detail |
|------|--------|
| **Description** | Readiness: 200 when server can accept work; 503 when Redis (session store) is unreachable. |
| **Auth** | No. |
| **Path/query/body** | None. |
| **Success** | `200 OK`. Body: `{ "data": { "status": "ok" }, "meta": { "requestId": "..." } }`. |
| **Errors** | `503` when session store is Redis and ping fails (`SERVICE_UNAVAILABLE`). `405` for non-GET. |

**Example request**

```bash
curl -s http://localhost:8080/api/v1/ready
```

**Example response**

```json
{
  "data": { "status": "ok" },
  "meta": { "requestId": "550e8400-e29b-41d4-a716-446655440000" }
}
```

---

### 6.2 Metrics and docs

#### `GET /metrics`

| Item | Detail |
|------|--------|
| **Description** | Prometheus metrics (text format). When metrics are disabled, endpoint still exists. |
| **Auth** | No. |
| **Path/query/body** | None. |
| **Success** | `200 OK` with Prometheus body when metrics enabled; `204 No Content` when metrics disabled. |
| **Errors** | `405` for non-GET. |

> **Note:** When metrics are disabled, GET `/metrics` returns 204 No Content, which is unusual for a GET resource.

---

#### `GET /swagger/` / `GET /swagger/doc.json`

| Item | Detail |
|------|--------|
| **Description** | Swagger UI and OpenAPI doc. Base path in spec is `/api/v1`. |
| **Auth** | No. |
| **Path/query/body** | None. |
| **Success** | `200` with HTML or JSON. |

---

### 6.3 WebRTC offer (smallwebrtc)

Available when `transport` is `smallwebrtc` or `both`.

#### `POST /webrtc/offer` / `POST /api/v1/webrtc/offer`

| Item | Detail |
|------|--------|
| **Description** | Submit a WebRTC SDP offer; server returns SDP answer and attaches a new transport to the pipeline. |
| **Auth** | Yes (when `server_api_key` is set). |
| **Path/query** | None. |
| **Request body** | `{ "offer": "<sdp string>" }`. `offer` required, non-empty after trim. |
| **Success** | `200 OK`. Body: `{ "data": { "answer": "<sdp>" }, "meta": { "requestId": "..." } }`. |
| **Errors** | `400` Invalid JSON (`BAD_REQUEST`). `401` Unauthorized. `405` Method not allowed. `422` Missing or empty offer (`VALIDATION_ERROR`, details for `offer`). `503` HandleOffer failed (`SERVICE_UNAVAILABLE`). `500` Start failed (`INTERNAL_ERROR`). |
| **CORS** | Allowed methods: `POST`, `OPTIONS`. Headers: `Content-Type`, `Authorization`, `X-API-Key`. |

> **Note:** Swagger defines the success response as `{ "answer": "..." }`. The actual response uses the envelope: `{ "data": { "answer": "..." }, "meta": { "requestId": "..." } }`.

**Example request**

```bash
curl -s -X POST http://localhost:8080/api/v1/webrtc/offer \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"offer":"v=0\r\n..."}'
```

**Example response**

```json
{
  "data": { "answer": "v=0\r\n..." },
  "meta": { "requestId": "550e8400-e29b-41d4-a716-446655440000" }
}
```

---

### 6.4 Runner: session start

Available when runner mode is enabled (WebRTC or Daily: `transport` is `smallwebrtc` or `both`, or `runner_transport` is `daily`).

#### `POST /start` / `POST /api/v1/start`

| Item | Detail |
|------|--------|
| **Description** | Create a session: either a Daily room (with URL/token) or a WebRTC session ID (and optional ICE config). Supports idempotency. |
| **Auth** | Yes (when `server_api_key` is set). |
| **Path/query** | None. |
| **Headers** | `Idempotency-Key` (optional): same key returns cached 201 response within TTL (24h). |
| **Request body** | `{ "createDailyRoom": boolean?, "enableDefaultIceServers": boolean?, "body": object? }`. All optional. `createDailyRoom` true → Daily room created. `enableDefaultIceServers` true → `iceConfig` with default STUN in response. `body` stored as session body for later use in session offer. |
| **Success** | `201 Created`. Body: `{ "data": { "sessionId": "<uuid>" } | { "sessionId", "iceConfig": { "iceServers": [...] } } | { "dailyRoom", "dailyToken", "sessionId" }, "meta": { "requestId": "..." } }`. Idempotent reply: same envelope returned from cache (cached response includes full envelope). |
| **Errors** | `400` Invalid JSON. `401` Unauthorized. `405` Method not allowed. `500` Daily configure or session store error. |
| **CORS** | `POST`, `OPTIONS`. |

**Example request**

```bash
curl -s -X POST http://localhost:8080/api/v1/start \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -H "Idempotency-Key: my-key-123" \
  -d '{"enableDefaultIceServers":true,"body":{"bot":"default"}}'
```

**Example response**

```json
{
  "data": {
    "sessionId": "550e8400-e29b-41d4-a716-446655440000",
    "iceConfig": {
      "iceServers": [ { "urls": [ "stun:stun.l.google.com:19302" ] } ]
    }
  },
  "meta": { "requestId": "660e8400-e29b-41d4-a716-446655440001" }
}
```

---

### 6.5 Runner: session offer

Same availability as `/start`. Legacy path: `/sessions/{id}/api/offer`. Versioned: `/api/v1/sessions/{id}/offer`.

#### `POST /sessions/{id}/api/offer` / `POST /api/v1/sessions/{id}/offer`

| Item | Detail |
|------|--------|
| **Description** | Submit SDP offer for session `id`; returns SDP answer and wires transport to pipeline. |
| **Auth** | Yes (when `server_api_key` is set). |
| **Path params** | `id`: session ID (UUID). |
| **Query** | None. |
| **Request body** | `{ "sdp": "<string>", "type": string?, "pc_id": string?, "restart_pc": boolean?, "request_data": object?, "requestData": object? }`. `sdp` required, non-empty after trim. `request_data` / `requestData` override session body for this request if provided. |
| **Success** | `200 OK`. Body: `{ "data": { "answer": "<sdp>", "type": "answer" }, "meta": { "requestId": "..." } }`. |
| **Errors** | `400` Invalid session ID format (not UUID). `401` Unauthorized. `404` Session or path not found. `405` Method not allowed. `422` Missing or empty SDP. `503` HandleOffer failed. `500` Start or store error. |
| **CORS** | `POST`, `PATCH`, `OPTIONS`. |

**Example request**

```bash
curl -s -X POST http://localhost:8080/api/v1/sessions/550e8400-e29b-41d4-a716-446655440000/offer \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"sdp":"v=0\r\n..."}'
```

**Example response**

```json
{
  "data": { "answer": "v=0\r\n...", "type": "answer" },
  "meta": { "requestId": "660e8400-e29b-41d4-a716-446655440001" }
}
```

---

#### `PATCH /sessions/{id}/api/offer` / `PATCH /api/v1/sessions/{id}/offer`

| Item | Detail |
|------|--------|
| **Description** | ICE/trickle acknowledgment; fire-and-forget. |
| **Auth** | Yes (when `server_api_key` is set). |
| **Path params** | `id`: session ID (UUID). |
| **Success** | `204 No Content`. |
| **Errors** | Same as POST for 400/401/404/405 (e.g. invalid ID, session not found, wrong method). |

---

### 6.6 WebSocket

#### `GET /ws`

| Item | Detail |
|------|--------|
| **Description** | WebSocket upgrade for real-time voice pipeline. Default: JSON frames (`type` + `data`). Query params change serialization. |
| **Auth** | Yes (when `server_api_key` is set): checked before upgrade; 401 on failure. |
| **Query params** | `rtvi=1`: use RTVI protocol. `format=protobuf`: binary protobuf frames. |
| **Success** | HTTP 101 upgrade; then WebSocket frames. |
| **Errors** | `401` Unauthorized. |

---

### 6.7 Root and telephony

Behavior of `GET /` and `POST /` depends on `runner_transport`.

#### Default (no telephony, not Daily)

- If `web/` directory exists: `GET /` serves static files (no API envelope).
- Otherwise: `GET /` returns `200` with `{ "data": { "status": "ok" }, "meta": { "requestId": "..." } }`. Any other path → `404`.

#### Telephony (`runner_transport`: twilio, telnyx, plivo, exotel)

| Method | Path | Description | Auth | Response |
|--------|------|-------------|------|----------|
| GET | `/` | Status. | No | `200`. `{ "data": { "status": "Bot started with <provider>" }, "meta": { "requestId": "..." } }`. |
| POST | `/` | Webhook: Twilio/Telnyx/Plivo return XML with `<Stream url="wss://{host}/telephony/ws"/>`. Exotel returns JSON with `error`, `websocketUrl`, `note` (informational). | No | Twilio/Telnyx/Plivo: `200`, `Content-Type: application/xml`. Exotel: `200`, JSON. |

> **Note:** Root `/` behavior and response type (JSON vs XML) depend on `runner_transport`. Exotel POST returns 200 with an `error` key in JSON (informational, not a failure).

**Example (Twilio-style) POST / response (XML)**

```xml
<?xml version="1.0" encoding="UTF-8"?><Response><Connect><Stream url="wss://your-host/telephony/ws"/></Connect></Response>
```

---

#### `GET /telephony/ws`

| Item | Detail |
|------|--------|
| **Description** | WebSocket for telephony media. Provider detected from first message(s). |
| **Auth** | No. |
| **Availability** | When `runner_transport` is twilio, telnyx, plivo, or exotel. |
| **Success** | 101 upgrade; then provider-specific frame format. |
| **Errors** | `405` for non-GET. |

---

### 6.8 Daily.co

#### `GET /` (Daily mode)

When `runner_transport` is `daily`:

| Item | Detail |
|------|--------|
| **Description** | Creates a Daily room and redirects to the room URL. |
| **Auth** | No. |
| **Success** | `302 Found` to Daily room URL. |
| **Errors** | `404` for non-GET. `500` if Daily configure fails. |

---

#### `POST /daily-dialin-webhook`

| Item | Detail |
|------|--------|
| **Description** | Incoming webhook for Daily PSTN dial-in. If `test` is true, responds OK; else validates payload and creates room; returns room URL, token, and session ID. |
| **Auth** | When `daily_dialin_webhook_secret` (or `VOXRAY_DAILY_DIALIN_WEBHOOK_SECRET`) is set: header `X-Webhook-Secret` must match. |
| **Availability** | When `runner_transport=daily` and `dialin=true`. |
| **Request body** | `{ "test": boolean?, "From": string?, "To": string?, "callId": string?, "callDomain": string? }`. For non-test: `From`, `To`, `callId`, `callDomain` required. |
| **Success** | Test: `200 OK`, `{ "data": { "status": "OK" }, "meta": { "requestId": "..." } }`. Non-test: `201 Created`, `{ "data": { "dailyRoom", "dailyToken", "sessionId" }, "meta": { "requestId": "..." } }`. |
| **Errors** | `400` Invalid JSON. `401` Invalid or missing webhook secret. `405` Method not allowed. `422` Missing required fields (with `details`). `500` Daily configure error. |

**Example request (test)**

```bash
curl -s -X POST http://localhost:8080/daily-dialin-webhook \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: your-secret" \
  -d '{"test":true}'
```

**Example response (201)**

```json
{
  "data": {
    "dailyRoom": "https://example.daily.co/room",
    "dailyToken": "eyJ...",
    "sessionId": "550e8400-e29b-41d4-a716-446655440000"
  },
  "meta": { "requestId": "660e8400-e29b-41d4-a716-446655440001" }
}
```

---

## 7. Middleware

Applied in this order (outer to inner):

| Order | Middleware | Purpose |
|-------|------------|--------|
| 1 | **Recovery** | Wraps entire mux; recovers panics, logs error and stack, returns `500` with standard error envelope (`INTERNAL_ERROR`). |
| 2 | **Metrics** | Per-route (when metrics enabled). Records request count, duration, status, active connections (Prometheus). Disabled when `metrics_enabled` is false. |
| 3 | **CORS** | Not global; applied inside handlers for `/webrtc/offer`, `/start`, `/sessions/`. Sets `Access-Control-Allow-Origin` (from config or `*`), `Access-Control-Allow-Methods`, `Access-Control-Allow-Headers: Content-Type, Authorization, X-API-Key`. |
| — | **Auth** | Per-handler: `requireAPIKey` called inside handlers that need it; not a separate middleware. |

---

## 8. Database Models

There is no relational database for API resources. Persistence relevant to the API:

| Concept | Storage | Key fields / types | Notes |
|--------|---------|--------------------|--------|
| **Session** | Memory or Redis (`session_store`: `memory` \| `redis`). | `Body`: `map[string]interface{}` (optional payload from POST /start). `EnableDefaultIceServers`: `bool`. | Created by POST /start. Redis: key `voxray:session:{id}`, TTL from `session_ttl_secs` (default 3600). |
| **Idempotency** | In-memory only. | Key: `Idempotency-Key` header value. Value: cached 201 response body. | TTL 24 hours. Used only for POST /start. |
| **Transcripts / recording** | External (SQL, S3) via config. | Not exposed as REST resources; config only (`transcripts`, `recording`). | |

---

## 9. Webhooks

### Incoming

| Endpoint | Event | Payload | Auth | Delivery |
|----------|--------|---------|------|----------|
| `POST /daily-dialin-webhook` | Daily PSTN dial-in | `test`, `From`, `To`, `callId`, `callDomain`. | Optional `X-Webhook-Secret`. | Synchronous HTTP response; no retry or delivery guarantee documented in code. |

### Outgoing

None. The server does not send webhooks to external URLs.

---

## 10. Changelog (inferred from code comments)

No formal `CHANGELOG` file exists. The following breaking or notable behaviors are documented in the code:

- **POST /start** returns `201 Created` with envelope `{ "data": { "sessionId", ... }, "meta": ... }` and supports optional `Idempotency-Key` (24h TTL).
- **Session offer path**: Versioned path is `/api/v1/sessions/:id/offer`; legacy `/sessions/:id/api/offer` still supported.
- **Session offer PATCH** returns `204 No Content` (ICE/trickle ack).
- **Session offer POST** returns `200` with envelope `{ "data": { "answer", "type": "answer" }, "meta": ... }`.
- **POST /webrtc/offer** response is envelope `{ "data": { "answer" }, "meta": ... }`; validation failure returns `422` with `VALIDATION_ERROR` and optional `details`.

---

*Generated from the Voxray server codebase. Base URL and auth behavior depend on config and environment.*
