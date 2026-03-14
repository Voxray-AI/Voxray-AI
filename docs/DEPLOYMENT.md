# Voxray-Go Deployment Guide

This document covers production deployment: environment variables, health checks, TLS, scaling, and security.

---

## Environment variables (12-factor)

After loading `config.json`, the following environment variables override config. Unset vars leave config unchanged.

| Variable | Description |
|----------|-------------|
| `VOXRAY_CONFIG` | Config file path (default for `-config` flag). |
| `PORT` or `VOXRAY_PORT` | Server port (e.g. `8080`). |
| `HOST` or `VOXRAY_HOST` | Bind address (e.g. `0.0.0.0` for all interfaces). |
| `VOXRAY_LOG_LEVEL` | Log level: `debug`, `info`, `error`. |
| `VOXRAY_JSON_LOGS` | Set to `true` or `1` for one-JSON-object-per-line logging. |
| `VOXRAY_TLS_ENABLE` | Set to `true` or `1` to enable TLS. |
| `VOXRAY_TLS_CERT_FILE` | Path to TLS certificate file. |
| `VOXRAY_TLS_KEY_FILE` | Path to TLS private key file. |
| `VOXRAY_CORS_ORIGINS` | Comma-separated allowed origins (e.g. `https://app.example.com`). Empty or unset = no CORS Origin header. |
| `VOXRAY_MAX_BODY_BYTES` | Max request body size in bytes for JSON endpoints (e.g. `1048576` for 1 MiB). |
| `VOXRAY_PIPELINE_INPUT_QUEUE_CAP` | Buffer size between transport read and pipeline push (default 256). Larger values absorb bursts; full buffer back-pressures the transport. |
| `VOXRAY_WS_WRITE_COALESCE_MS` | When > 0, WebSocket write coalescing: drain up to `VOXRAY_WS_WRITE_COALESCE_MAX_FRAMES` frames within this many ms before writing (reduces syscalls; adds latency). 0 = disabled. |
| `VOXRAY_WS_WRITE_COALESCE_MAX_FRAMES` | Max frames to coalesce per write window when coalescing is enabled (e.g. 10). |
| `VOXRAY_RECORDING_QUEUE_CAP` | Recording upload job queue capacity (default 32). Tune with worker count for S3 throughput. |
| `VOXRAY_RECORDING_MAX_RETRIES` | Number of S3 upload retries with exponential backoff (default 3). |
| API keys | As in config: `OPENAI_API_KEY`, `DAILY_API_KEY`, etc., or via `api_keys` in config. Resolved keys are cached to avoid repeated env lookups. |

---

## Health and readiness

- **`GET /health`** (liveness): Returns 200 when the process is up. Use for liveness probes.
- **`GET /ready`** (readiness): Returns 200 when the server is ready to accept traffic. When `session_store=redis`, returns 503 if Redis is unreachable. Use for readiness probes.

Example (Kubernetes):

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 2
  periodSeconds: 5
```

---

## TLS

- **On-server TLS:** Set `tls_enable: true` and `tls_cert_file` / `tls_key_file` in config (or `VOXRAY_TLS_*` env). The server will use `ListenAndServeTLS`.
- **Reverse proxy:** In production, TLS is often terminated at a reverse proxy (nginx, Ingress, load balancer). Run Voxray with TLS disabled and bind to `127.0.0.1` or a private network; expose the proxy publicly with HTTPS.

---

## Performance and scaling (tuning)

- **Pipeline input queue**: `pipeline_input_queue_cap` (or `VOXRAY_PIPELINE_INPUT_QUEUE_CAP`) sets the buffer between transport read and pipeline push. When full, the reader blocks so the transport does not consume unbounded memory; default 256. Increase under very bursty input if needed.
- **WebSocket write coalescing**: When `ws_write_coalesce_ms` > 0, the WebSocket writer drains multiple frames in a short window before writing, reducing syscalls at the cost of a small latency budget. Disabled by default (0).
- **Recording**: `recording.queue_cap` and `recording.worker_count` control the upload job queue and worker pool; `recording.max_retries` enables exponential backoff on S3 failures. Uploads stream from temp files to S3 (no full WAV in memory). See [ARCHITECTURE.md](ARCHITECTURE.md) for concurrency notes.
- **Metrics**: Prometheus metric names are stable for dashboards and alerts. Per-chunk metric sampling may be added in a future release.

---

## Horizontal scaling

- **Single instance:** Use default in-memory session store. No extra dependencies.
- **Multiple instances:** Set `session_store=redis` and `redis_url` (e.g. `redis://redis:6379/0`). All instances share session state via Redis. Use a load balancer in front; ensure WebSocket/HTTP affinity if required, or keep sessions in Redis so any instance can serve them.

---

## Observability

- **`GET /metrics`** returns Prometheus text format (e.g. `voxray_up 1`). Point Prometheus at this endpoint for scraping. Consider restricting access (e.g. firewall, auth header) so `/metrics` is not public.

---

## Security checklist

- **CORS:** Set `cors_allowed_origins` (or `VOXRAY_CORS_ORIGINS`) to your front-end origins; avoid leaving CORS fully open in production.
- **TLS:** Use TLS in front of the server (on-server or reverse proxy).
- **Request body limit:** Set `max_request_body_bytes` (or `VOXRAY_MAX_BODY_BYTES`) to limit JSON body size (e.g. 1 MiB).
- **Secrets:** Provide API keys via environment variables or a secrets manager, not in config in repo.
- **Metrics:** Restrict access to `/metrics` (e.g. network policy or auth).

---

## Docker

- **Build:** `docker build -t voxray-go .`
- **Run:** Mount config and set port, e.g.  
  `docker run -p 8080:8080 -v $(pwd)/config.json:/app/config.json voxray-go`
- **docker-compose:** See root `docker-compose.yml`. Run with `docker compose up`; mount your `config.json`. Optionally enable the Redis service and set `session_store=redis` and `redis_url` for multi-instance sessions.

---

## References

- [CONNECTIVITY.md](./CONNECTIVITY.md) — what can connect (WebSocket, WebRTC, telephony) and wire formats.
- [SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md) — system view and deployment diagram.
- [ARCHITECTURE.md](./ARCHITECTURE.md) — components and data flow.
