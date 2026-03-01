# Voila-Go Deployment Guide

This document covers production deployment: environment variables, health checks, TLS, scaling, and security.

---

## Environment variables (12-factor)

After loading `config.json`, the following environment variables override config. Unset vars leave config unchanged.

| Variable | Description |
|----------|-------------|
| `VOILA_CONFIG` | Config file path (default for `-config` flag). |
| `PORT` or `VOILA_PORT` | Server port (e.g. `8080`). |
| `HOST` or `VOILA_HOST` | Bind address (e.g. `0.0.0.0` for all interfaces). |
| `VOILA_LOG_LEVEL` | Log level: `debug`, `info`, `error`. |
| `VOILA_JSON_LOGS` | Set to `true` or `1` for one-JSON-object-per-line logging. |
| `VOILA_TLS_ENABLE` | Set to `true` or `1` to enable TLS. |
| `VOILA_TLS_CERT_FILE` | Path to TLS certificate file. |
| `VOILA_TLS_KEY_FILE` | Path to TLS private key file. |
| `VOILA_CORS_ORIGINS` | Comma-separated allowed origins (e.g. `https://app.example.com`). Empty or unset = no CORS Origin header. |
| `VOILA_MAX_BODY_BYTES` | Max request body size in bytes for JSON endpoints (e.g. `1048576` for 1 MiB). |
| API keys | As in config: `OPENAI_API_KEY`, `DAILY_API_KEY`, etc., or via `api_keys` in config. |

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

- **On-server TLS:** Set `tls_enable: true` and `tls_cert_file` / `tls_key_file` in config (or `VOILA_TLS_*` env). The server will use `ListenAndServeTLS`.
- **Reverse proxy:** In production, TLS is often terminated at a reverse proxy (nginx, Ingress, load balancer). Run Voila with TLS disabled and bind to `127.0.0.1` or a private network; expose the proxy publicly with HTTPS.

---

## Horizontal scaling

- **Single instance:** Use default in-memory session store. No extra dependencies.
- **Multiple instances:** Set `session_store=redis` and `redis_url` (e.g. `redis://redis:6379/0`). All instances share session state via Redis. Use a load balancer in front; ensure WebSocket/HTTP affinity if required, or keep sessions in Redis so any instance can serve them.

---

## Observability

- **`GET /metrics`** returns Prometheus text format (e.g. `voila_up 1`). Point Prometheus at this endpoint for scraping. Consider restricting access (e.g. firewall, auth header) so `/metrics` is not public.

---

## Security checklist

- **CORS:** Set `cors_allowed_origins` (or `VOILA_CORS_ORIGINS`) to your front-end origins; avoid leaving CORS fully open in production.
- **TLS:** Use TLS in front of the server (on-server or reverse proxy).
- **Request body limit:** Set `max_request_body_bytes` (or `VOILA_MAX_BODY_BYTES`) to limit JSON body size (e.g. 1 MiB).
- **Secrets:** Provide API keys via environment variables or a secrets manager, not in config in repo.
- **Metrics:** Restrict access to `/metrics` (e.g. network policy or auth).

---

## Docker

- **Build:** `docker build -t voila-go .`
- **Run:** Mount config and set port, e.g.  
  `docker run -p 8080:8080 -v $(pwd)/config.json:/app/config.json voila-go`
- **docker-compose:** See root `docker-compose.yml`. Run with `docker compose up`; mount your `config.json`. Optionally enable the Redis service and set `session_store=redis` and `redis_url` for multi-instance sessions.

---

## References

- [SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md) — system view and deployment diagram.
- [ARCHITECTURE.md](./ARCHITECTURE.md) — components and data flow.
