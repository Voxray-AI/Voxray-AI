# Services layer

This package provides LLM, STT, TTS, and realtime service abstractions aligned with the [Pipecat Python services](https://github.com/pipecat-ai/pipecat/tree/main/src/pipecat/services). Use the factory and `config.Config` to construct implementations by provider name.

## Interfaces

- **LLMService** — chat completion with optional streaming (`Chat(ctx, messages, onToken)`).
- **STTService** — transcription (`Transcribe(ctx, audio, sampleRate, numChannels)`). Optional **STTStreamingService** adds `TranscribeStream`.
- **TTSService** — text-to-speech (`Speak(ctx, text, sampleRate)`). Optional **TTSStreamingService** adds `SpeakStream`.
- **RealtimeService** — creates **RealtimeSession** (SendText, SendAudio, Events, Close). Use `realtime.NewFromConfig(cfg, provider)` to construct (lives in `pkg/realtime` to avoid import cycles).

## Supported providers

| Provider   | LLM | STT | TTS | Realtime |
|-----------|-----|-----|-----|----------|
| openai    | ✓   | ✓   | ✓   | ✓        |
| groq      | ✓   | ✓   | ✓   | —        |
| sarvam    | —   | ✓   | ✓   | —        |
| grok      | ✓   | —   | —   | —        |
| cerebras  | ✓   | —   | —   | —        |
| elevenlabs| —   | ✓   | ✓   | —        |
| aws       | ✓   | ✓   | ✓   | —        |
| mistral   | ✓   | —   | —   | —        |
| deepseek  | ✓   | —   | —   | —        |

Constants: `ProviderOpenAI`, `ProviderGroq`, `ProviderSarvam`, `ProviderGrok`, `ProviderCerebras`, `ProviderElevenLabs`, `ProviderAWS`, `ProviderMistral`, `ProviderDeepSeek`. Realtime: `SupportedRealtimeProviders` (currently `"openai"`).

## Configuration

Use **config.Config** (JSON or env):

- **provider** — default for all tasks.
- **stt_provider**, **llm_provider**, **tts_provider** — override per task.
- **model** — chat/LLM model (e.g. `gpt-3.5-turbo`, `mistral-small-latest`, `deepseek-chat`).
- **stt_model**, **tts_model**, **tts_voice** — task-specific when supported.
- **api_keys** — map of service name to API key; otherwise keys are read from environment.

### Environment variables (fallback when not in `api_keys`)

| Provider   | Env var |
|-----------|---------|
| openai    | OPENAI_API_KEY |
| groq      | GROQ_API_KEY |
| sarvam    | SARVAM_API_KEY |
| grok (xai) | XAI_API_KEY |
| cerebras  | CEREBRAS_API_KEY |
| elevenlabs| ELEVENLABS_API_KEY |
| aws       | AWS_SECRET_ACCESS_KEY, AWS_REGION (optional, default us-east-1) |
| mistral   | MISTRAL_API_KEY |
| deepseek  | DEEPSEEK_API_KEY |

## Usage

```go
cfg, _ := config.LoadConfig("config.json")
// Or build manually:
cfg := &config.Config{
    LlmProvider: services.ProviderMistral,
    Model:       "mistral-small-latest",
}

llm := services.NewLLMFromConfig(cfg, cfg.LLMProvider(), cfg.Model)
stt := services.NewSTTFromConfig(cfg, cfg.STTProvider())
tts := services.NewTTSFromConfig(cfg, cfg.TTSProvider(), cfg.TTSModel, cfg.TTSVoice)

// Realtime (e.g. OpenAI Realtime WebSocket API):
realtimeSvc, err := realtime.NewFromConfig(cfg, "openai")
```

One-shot construction for all three:

```go
llm, stt, tts := services.NewServicesFromConfig(cfg)
```

## Tests

- `tests/pkg/services/` — factory construction tests for all supported providers; Sarvam integration test (requires `SARVAM_API_KEY`).
- `tests/pkg/realtime/` — realtime.NewFromConfig for openai and unsupported provider.
