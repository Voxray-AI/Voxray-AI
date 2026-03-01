# Frameworks: External chain and RTVI

This document describes the **frameworks** processors ported from upstream frameworks: external chain (Langchain/Strands-style backends) and the RTVI protocol.

---

## 1. External chain (Langchain / Strands)

Langchain and Strands are Python-only. In Go, **external_chain** calls an HTTP endpoint (e.g. a Python sidecar running Langchain or Strands) with the last user message from `LLMContextFrame` and streams the response back as `LLMTextFrame` with `LLMFullResponseStartFrame` / `LLMFullResponseEndFrame`.

### Config

Add `external_chain` to `plugins` and set `plugin_options["external_chain"]`:

```json
{
  "plugins": ["external_chain", "..."],
  "plugin_options": {
    "external_chain": {
      "url": "http://localhost:8765/chain",
      "method": "POST",
      "stream": true,
      "timeout_sec": 60,
      "transcript_key": "input",
      "headers": { "X-Custom": "value" }
    }
  }
}
```

- **url** (required): HTTP endpoint. When empty, the processor no-ops (forwards frames).
- **method**: `POST` (default) or `GET`.
- **stream**: if true, response is parsed as SSE or line-delimited JSON chunks (`data: {"text":"..."}` or `{"content":"..."}`).
- **timeout_sec**: request timeout; default 30.
- **transcript_key**: JSON key for the user message in the request body; default `"input"`. Request body is `{ "<transcript_key>": "<last user message text>" }`.
- **headers**: optional HTTP headers.

### Running a Langchain/Strands sidecar

1. Run your Python service that exposes an HTTP endpoint (e.g. `/chain`) accepting JSON `{"input": "user message"}` and responding with JSON `{"text": "..."}` or streaming SSE.
2. Point `external_chain.url` at that endpoint (e.g. `http://localhost:8765/chain`).
3. Ensure the pipeline feeds `LLMContextFrame` into the chain (e.g. use an aggregator that produces context and then this processor).

---

## 2. RTVI (Real-Time Voice Interface)

RTVI is the client–server messaging protocol (bot-ready, send-text, errors, etc.). The Go implementation provides:

- **RTVIProcessor** (`plugins`: `rtvi`): Handles `StartFrame` (sends bot-ready), `RTVIClientMessageFrame` (client-ready, send-text → `TranscriptionFrame`), and `ErrorFrame` (sends RTVI error).
- **RTVI serializer**: When the WebSocket connection uses `?rtvi=1`, the server uses the RTVI serializer so that client messages are parsed into `RTVIClientMessageFrame` and pipeline frames are converted to RTVI server messages (bot-ready, bot-output, user-transcription, error, etc.).

### Enabling RTVI

1. **WebSocket**: Connect to `/ws?rtvi=1` so the server selects the RTVI serializer.
2. **Plugins**: Include `rtvi` in `plugins` (typically first, so client messages are handled before other processors).
3. **plugin_options** (optional): `"rtvi": { "protocol_version": "1.2.0" }`.

### Client flow

1. Client connects to `ws://host:port/ws?rtvi=1`.
2. Server pushes `StartFrame`; RTVIProcessor sends a **bot-ready** RTVI message.
3. Client sends RTVI messages, e.g. **client-ready** (version, about) and **send-text** (`{"content": "hello"}`). The serializer turns these into `RTVIClientMessageFrame`; RTVIProcessor turns send-text into `TranscriptionFrame` and pushes it downstream (e.g. to LLM → TTS).
4. Pipeline output (e.g. `LLMTextFrame`, `TranscriptionFrame`, `ErrorFrame`) is serialized as RTVI **bot-output**, **user-transcription**, **error**, etc., and sent to the client.

### RTVI message types (Phase 1–2)

- **Client → server**: `client-ready`, `send-text` (data: `content`, optional `options`).
- **Server → client**: `bot-ready` (version, about), `bot-output` (text), `user-transcription` (text, final), `error` (error, fatal), `bot-started-speaking`, `bot-stopped-speaking`.

For full compatibility with RTVI clients, see the upstream RTVI protocol. Deprecated actions/config are not implemented in the initial phases.
