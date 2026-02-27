# Minimal voice pipeline example

This example runs a **voice agent** pipeline: STT → LLM → TTS → transport.

- **STT**: Audio from the client (e.g. `AudioRawFrame`) is transcribed to text (`TranscriptionFrame`).
- **LLM**: User text is sent to the LLM; responses are streamed as `LLMTextFrame`.
- **TTS**: LLM text is converted to speech (`TTSAudioRawFrame`) and sent to the client.

## Config

Use a config that sets `provider` and `model` so the server builds the voice pipeline:

```json
{
  "host": "localhost",
  "port": 8080,
  "model": "gpt-3.5-turbo",
  "provider": "openai",
  "plugins": []
}
```

For Groq (faster, cheaper):

```json
{
  "host": "localhost",
  "port": 8080,
  "model": "llama-3.1-8b-instant",
  "provider": "groq",
  "plugins": []
}
```

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
