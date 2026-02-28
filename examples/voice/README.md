# Minimal voice pipeline example

This example runs a **voice agent** pipeline: STT → LLM → TTS → transport.

- **STT**: Audio from the client (e.g. `AudioRawFrame`) is transcribed to text (`TranscriptionFrame`).
- **LLM**: User text is sent to the LLM; responses are streamed as `LLMTextFrame`.
- **TTS**: LLM text is converted to speech (`TTSAudioRawFrame`) and sent to the client.

## Config

Use a config that sets `provider` and `model` so the server builds the voice pipeline. By default,
the network transport is a WebSocket server on `/ws`:

```json
{
  "host": "localhost",
  "port": 8080,
  "model": "gpt-3.5-turbo",
  "provider": "openai",
  "plugins": [],
  "transport": "websocket"
}
```

For Groq (faster, cheaper), with optional task-specific models:

```json
{
  "host": "localhost",
  "port": 8080,
  "model": "llama-3.1-8b-instant",
  "provider": "groq",
  "stt_model": "whisper-large-v3-turbo",
  "tts_model": "canopylabs/orpheus-v1-english",
  "tts_voice": "alloy",
  "plugins": [],
  "transport": "websocket"
}
```

You can mix providers per task (e.g. OpenAI for STT, Groq for LLM and TTS) by setting `stt_provider`, `llm_provider`, and `tts_provider`; put the corresponding keys in `api_keys` (e.g. `openai`, `groq`).

To enable SmallWebRTC signaling (mirroring the `small_webrtc` network transport), set:

```json
{
  "transport": "both",
  "webrtc_ice_servers": ["stun:stun.l.google.com:19302"]
}
```

This will keep the WebSocket endpoint at `/ws` and also expose a WebRTC offer/answer endpoint at
`/webrtc/offer` that accepts a JSON body like:

```json
{
  "offer": "{ \"type\": \"offer\", \"sdp\": \"...\" }"
}
```

The response contains an `answer` field with the JSON SDP to use as the remote description on the client.

### WebRTC + Sarvam STT/TTS + Groq LLM

For a WebRTC-based real-time voice pipeline with **Sarvam** (STT and TTS) and **Groq** (LLM):

```json
{
  "host": "localhost",
  "port": 8080,
  "transport": "both",
  "stt_provider": "sarvam",
  "llm_provider": "groq",
  "tts_provider": "sarvam",
  "model": "llama-3.1-8b-instant",
  "stt_model": "saarika:v2.5",
  "tts_model": "bulbul:v2",
  "webrtc_ice_servers": ["stun:stun.l.google.com:19302"],
  "api_keys": {
    "sarvam": "<SARVAM_API_KEY>",
    "groq": "<GROQ_API_KEY>"
  }
}
```

- **Run the server**: `go run ./cmd/voila -config config.json` (from repo root).
- **Run the WebRTC client**: Open `tests/frontend/webrtc-voice.html` in a browser (or serve it), set the server URL to `http://localhost:8080`, click **Start**, allow the microphone, then speak. The synthesized reply is played back over the remote audio track.

See `tests/frontend/README.md` for the JavaScript WebRTC client details.

## Run

1. Set API keys:
   - OpenAI: `OPENAI_API_KEY`
   - Groq: `GROQ_API_KEY`

2. Start the server:
   ```bash
   cd ../..
   go run ./cmd/Voila --config config.json
   ```

3. Connect a WebSocket client to `ws://localhost:8080/ws`.

4. Send a `StartFrame`, then send `AudioRawFrame` (or for testing, `TranscriptionFrame` with user text). The pipeline will run STT (if audio), LLM, TTS and send audio back.

## Frame flow

- Client → `StartFrame`, then `AudioRawFrame` (PCM 16-bit mono, e.g. 16 kHz) or `TranscriptionFrame` (text).
- Server → `TranscriptionFrame` (if STT), `LLMTextFrame` (streamed), `TTSAudioRawFrame` (output audio).

See `pkg/frames/serialize` for JSON envelope format.
