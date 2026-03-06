## 2. AI/ML Stack

This section covers **LLM, STT, TTS, and Realtime** capabilities, how they are wired in code, and the skills needed to work on them effectively.

---

### 2.1 Architectural Overview

- **Service interfaces (`pkg/services`)**
  - `LLMService` / `LLMServiceWithTools`, `STTService` / `STTStreamingService`, `TTSService` / `TTSStreamingService`, and `RealtimeService` define **provider‑agnostic contracts** for AI functionality.
  - Factories in `services.NewServicesFromConfig` and `services.NewFromConfig` construct these services from `config.Config`:
    - Global defaults: `provider`, `model`.
    - Per‑task overrides: `stt_provider`, `llm_provider`, `tts_provider`, `stt_model`, `tts_model`, `tts_voice`.
  - **Skill implication (Intermediate/Advanced Go + APIs)**: You must understand interface‑based design, streaming APIs, and how to surface provider differences through common abstractions.

- **Frames as the AI wiring glue (`pkg/frames`)**
  - `TranscriptionFrame` carries user text from STT; `LLMTextFrame` carries streamed model output; `TTSSpeakFrame` requests speech generation; `TTSAudioRawFrame` (and related audio frames) carry synthesized audio.
  - `LLMContextFrame`, `LLMMessagesUpdateFrame`, `LLMMessagesAppendFrame`, `LLMSetToolsFrame`, `LLMSetToolChoiceFrame`, and `FunctionCallResultFrame` express **prompt state** and **tool‑calling** results inside the pipeline.
  - **Skill implication (Advanced)**: To modify AI behavior you often work at the frame level, not just at HTTP/JSON level.

- **Voice pipeline (docs + `cmd/voxray/main.go`)**
  - When `cfg.Provider` and `cfg.Model` are set, `run` constructs a **voice pipeline**:
    - `TurnProcessor` (optional, VAD) → `STTProcessor` → `LLMProcessor` → `TTSProcessor` → `Sink`.
  - Each `voice.*Processor` wraps a concrete AI service and maps between frames and provider API calls.

---

### 2.2 LLM Providers & Patterns

All LLM providers implement a common **streaming chat** interface with **tool‑calling support** where available. Code lives under `pkg/services/<provider>/`.

- **OpenAI — Advanced**
  - **Files**: `pkg/services/openai/openai.go`, factory wiring via `services.NewLLMFromConfig`.
  - **Role**: Baseline LLM provider with rich streaming and tool‑calling (functions) support; also provides STT and TTS.
  - **Key behaviors**:
    - Maps `LLMContext` messages to OpenAI chat messages, preserving multi‑modal content (e.g. `AddImageMessage`, `AddAudioMessage` in `LLMContext`).
    - Registers tools when used with `LLMServiceWithTools` and MCP (`pkg/mcp/client.go`).
    - Handles streaming responses, surfacing them as `LLMTextFrame` and bracketing full responses with `LLMFullResponseStartFrame` / `LLMFullResponseEndFrame` for downstream aggregators and extensions.
  - **Skill implication**:
    - Deep understanding of OpenAI’s chat and tool‑calling APIs, including function schemas and streaming behaviors.
    - Ability to map internal frame structures to OpenAI requests/responses without losing semantics (e.g. tool IDs, arguments).

- **Groq — Intermediate/Advanced**
  - **Files**: `pkg/services/groq/llm.go`, `pkg/services/stt/groq.go`, `pkg/services/tts/groq.go`.
  - **Role**: High‑throughput, low‑latency provider with OpenAI‑style chat/STT/TTS endpoints; default models like `llama-3.1-8b-instant`.
  - **Key behaviors**:
    - Reuses OpenAI‑like request/response patterns but may have model‑specific rate limits and error codes.
    - Often configured as the default LLM (`model: "llama-3.1-8b-instant"`) with other providers (e.g. OpenAI, Sarvam) for STT/TTS.
  - **Skill implication**:
    - Familiarity with OpenAI‑compatible APIs and model families (LLama 3.*).
    - Ability to tune model choices and understand trade‑offs (latency vs cost vs quality).

- **Anthropic (direct + via AWS Bedrock) — Advanced**
  - **Files**: `pkg/services/anthropic/llm.go`, `pkg/services/aws/llm.go`.
  - **Role**: Access to Claude models, either via native Anthropic API or Bedrock ConverseStream, including streaming.
  - **Key behaviors**:
    - Different API surface than OpenAI; internal adapters translate into `LLMContext` semantics (roles, messages, tool usage).
    - Bedrock path requires AWS credential/configuration understanding (`aws-sdk-go-v2`).
  - **Skill implication**:
    - Comfort with multiple vendor APIs and AWS’s auth/config primitives.
    - Ability to debug subtle behavioral differences (e.g. tokenization, stop sequences, system prompts) across Anthropic vs OpenAI.

- **Mistral, Qwen/DashScope, DeepSeek, Cerebras — Intermediate**
  - **Files**: `pkg/services/mistral/llm.go`, `pkg/services/qwen/llm.go`, `pkg/services/deepseek/llm.go`, `pkg/services/cerebras/llm.go`.
  - **Role**: Additional LLM providers with varying strengths (e.g. long‑context, code focus, low‑latency).
  - **Key behaviors**:
    - Use provider‑specific SDKs or HTTP clients; adapt them to `LLMService` interface.
    - Some may not support tools or multi‑modal features; adapters stub or ignore unsupported capabilities gracefully.
  - **Skill implication**:
    - Ability to integrate new providers by following existing patterns (config, factory, service implementation, tests).
    - Awareness that feature parity (tools, streaming) is not uniform across providers.

- **Google Gemini / Vertex AI — Intermediate/Advanced**
  - **Files**: `pkg/services/google/llm.go`, `pkg/services/google/llm_vertex.go`.
  - **Role**: Access to Gemini via public API and via Vertex AI, often used for multi‑modal LLM tasks and enterprise integration.
  - **Key behaviors**:
    - Uses `google.golang.org/genai` and `google.golang.org/api` clients.
    - Adapts Gemini’s multi‑modal message structures into `LLMContext` with image/audio parts.
  - **Skill implication**:
    - Familiarity with Google Cloud authentication and project configuration.
    - Comfort reading and mapping between Google’s generative APIs and Voxray’s internal frame model.

- **Ollama — Intermediate**
  - **Files**: `pkg/services/ollama/llm.go`.
  - **Role**: Local or remote self‑hosted models via an OpenAI‑compatible HTTP API.
  - **Key behaviors**:
    - Enables testing and development without external cloud providers.
    - Often used to validate pipeline behavior or run on air‑gapped environments.
  - **Skill implication**:
    - Understanding of local model lifecycle (pulling images, memory/VRAM constraints).
    - Ability to reason about performance and prompt design when operating entirely locally.

---

### 2.3 STT (Speech‑to‑Text) Providers

STT services convert `AudioRawFrame` segments into `TranscriptionFrame` messages, typically orchestrated by `voice.STTProcessor`.

- **OpenAI Whisper (API) — Intermediate**
  - **Files**: `pkg/services/stt/openai.go`.
  - **Role**: General‑purpose STT for voice pipelines and telephony.
  - **Key behaviors**:
    - Supports streaming or chunked audio; adapters decide how to accumulate and send buffers.
    - Produces `TranscriptionFrame` with `Finalized` flag, which the `TranscriptObserver` uses to log user messages.
  - **Skill implication**:
    - Understanding of audio encoding requirements (e.g. PCM vs encoded formats) and Whisper’s latency/accuracy trade‑offs.

- **Groq Whisper — Intermediate**
  - **Files**: `pkg/services/stt/groq.go`.
  - **Role**: Uses Groq’s Whisper models to reduce STT latency and cost.
  - **Key behaviors**:
    - Similar interface to OpenAI Whisper, but with Groq‑specific models and endpoints.
  - **Skill implication**:
    - Same as OpenAI STT, with an added need to understand provider‑specific rate‑limits and error modes.

- **Sarvam — Advanced (domain‑specific)**
  - **Files**: `pkg/services/sarvam/stt.go`, `pkg/services/sarvam/stt_streaming.go`.
  - **Role**: Streaming STT specialized for certain languages/regions; often paired with Sarvam TTS.
  - **Key behaviors**:
    - WebSocket and HTTP streaming STT; uses provider‑specific message formats and timeouts.
    - Models like `saarika:v2.5` configured via `stt_model`.
  - **Skill implication**:
    - Ability to work with provider‑specific streaming protocols and recover from partial or out‑of‑order messages.
    - Enough domain understanding to tune parameters (e.g. language, dialect, latency vs completeness).

- **ElevenLabs STT — Intermediate**
  - **Files**: `pkg/services/elevenlabs/stt.go`.
  - **Role**: STT for scenarios where ElevenLabs is already used for TTS, easing key management and integration.
  - **Key behaviors**:
    - HTTP‑based STT; may trade off latency vs quality depending on configuration.
  - **Skill implication**:
    - Familiarity with ElevenLabs APIs and authentication.

- **AWS Transcribe Streaming — Advanced**
  - **Files**: `pkg/services/aws/stt.go`.
  - **Role**: Enterprise‑grade, region‑aware STT for AWS‑centric deployments.
  - **Key behaviors**:
    - Uses `aws-sdk-go-v2/service/transcribestreaming` streaming APIs; must handle AWS credential resolution and region selection.
  - **Skill implication**:
    - Strong AWS SDK experience, especially with streaming RPCs and error handling (throttling, timeouts).

- **Google Cloud Speech‑to‑Text — Intermediate/Advanced**
  - **Files**: `pkg/services/google/stt.go`.
  - **Role**: STT with support for various languages, sample rates, and streaming modes, backed by `cloud.google.com/go/speech`.
  - **Skill implication**:
    - Comfortable configuring Google Cloud projects and service accounts.
    - Understanding of streaming gRPC and client lifecycle in Go.

---

### 2.4 TTS (Text‑to‑Speech) Providers

TTS services turn `TTSSpeakFrame` text into `TTSAudioRawFrame` or Opus audio, typically via `voice.TTSProcessor`.

- **OpenAI TTS — Intermediate**
  - **Files**: `pkg/services/tts/openai.go`.
  - **Role**: Converts model output text into audio for WebSocket or WebRTC clients.
  - **Key behaviors**:
    - May stream audio in chunks; the pipeline routes them to appropriate transports.
    - Voice/model names (e.g. `alloy`) configured via `tts_model` and `tts_voice`.
  - **Skill implication**:
    - Familiarity with OpenAI TTS endpoints and audio formats.

- **Groq TTS — Intermediate**
  - **Files**: `pkg/services/tts/groq.go`.
  - **Role**: Complements Groq LLM/STT options, keeping all AI services under one provider.
  - **Skill implication**:
    - Same as OpenAI TTS, with attention to model options and performance characteristics.

- **Sarvam TTS — Advanced**
  - **Files**: `pkg/services/sarvam/tts.go`, `pkg/services/sarvam/tts_streaming.go`.
  - **Role**: Regional/language‑specific TTS optimized for certain use cases (e.g. contact‑center voice flows).
  - **Key behaviors**:
    - Streaming TTS over WebSocket; interplay with STT for echo cancellation and latency control.
  - **Skill implication**:
    - Ability to debug streaming audio issues across languages and network conditions.

- **ElevenLabs TTS — Advanced (voice design)**
  - **Files**: `pkg/services/elevenlabs/tts.go`.
  - **Role**: High‑quality, expressive voice TTS with custom voices and emotions.
  - **Skill implication**:
    - Understanding of ElevenLabs voice/voice‑ID concepts; mapping custom voices into config.
    - Balancing expressiveness against latency and cost.

- **AWS Polly — Intermediate**
  - **Files**: `pkg/services/aws/tts.go`.
  - **Role**: AWS‑native TTS, often paired with AWS Transcribe and Bedrock.
  - **Skill implication**:
    - Familiarity with AWS Polly voices and regions.

- **Google Cloud Text‑to‑Speech — Intermediate**
  - **Files**: `pkg/services/google/tts.go`.
  - **Role**: Google‑native TTS; often used where Gemini or GCP STT is already configured.
  - **Skill implication**:
    - Understanding of GCP TTS voice/model selection.

---

### 2.5 Realtime Services

- **OpenAI Realtime (abstraction) — Advanced**
  - **Files**: `pkg/realtime/openai.go`, interfaces in `pkg/services/interfaces.go`.
  - **Role**: Encapsulates future use of OpenAI’s Realtime API, while currently leveraging existing STT/LLM/TTS services behind a WebSocket‑like `RealtimeSession`.
  - **Key behaviors**:
    - `RealtimeService`/`RealtimeSession` provide an event‑driven API (`SendText`, `SendAudio`, `Events`).
    - Current implementation uses existing services; can be swapped later for true Realtime APIs without changing higher‑level pipeline logic.
  - **Skill implication**:
    - Experience designing for **evolution**: building abstractions that can pivot from legacy chains to single‑provider realtime backends.
    - Comfort working with event streams and translating between transport events and internal frames.

---

### 2.6 Tools, MCP, and Prompt Engineering Patterns

- **LLM Context & Prompt Management — Advanced**
  - **Files**: `pkg/frames/llm.go`, `pkg/processors/aggregators/llmfullresponse`, `pkg/processors/aggregators/llmcontextsummarizer`, `pkg/processors/aggregators/userresponse`.
  - **Patterns**:
    - **Context as frames**: `LLMContextFrame` carries the full conversation, tools, and tool choice; other frames request partial updates (`LLMMessagesUpdateFrame`, `LLMMessagesAppendFrame`) or summarization (`LLMContextSummaryRequestFrame`/`ResultFrame`).
    - **Summarization for long‑running sessions**: `llmcontextsummarizer` monitors context size and asks the LLM to summarize older messages, replacing them with summaries to stay under provider token limits.
    - **Turn‑aware user prompts**: `userresponse` aggregates STT outputs until user stops speaking, emitting a single aggregated transcription for the LLM.
  - **Skill implication**:
    - Ability to design prompts that interact well with automated summarization and tool use, while respecting token budgets.
    - Understanding how context frames are built and updated so you can safely modify or extend prompt flows.

- **Tool calling via MCP — Advanced**
  - **Files**: `pkg/mcp/client.go`, `pkg/adapters/schemas`, OpenAI LLM service implementation.
  - **Patterns**:
    - `mcp.Client` connects to an MCP server via stdio (`CommandTransport`), lists tools, and converts schemas to internal `FunctionSchema`.
    - For `LLMServiceWithTools`, tools are registered with handlers that call back to MCP on demand; results are returned to the LLM as tool outputs.
    - Tool outputs can be post‑processed with per‑tool output filters to simplify or normalize responses.
  - **Skill implication**:
    - Strong understanding of **function‑calling LLMs** and how to design tool schemas (JSON Schema properties, required args, descriptions).
    - Ability to debug multi‑hop flows: LLM → tool call → MCP server → external system → MCP response → LLM.

- **Framework integrations (external_chain, RTVI) — Intermediate/Advanced**
  - **Files**: `pkg/processors/frameworks/*`, `docs/FRAMEWORKS.md`.
  - **Patterns**:
    - **external_chain**: Treats an HTTP endpoint (e.g. LangChain or Strands) as the LLM; sends the user’s last message as JSON (configurable key), and streams text back as `LLMTextFrame` while bracketing with full‑response frames.
    - **rtvi**: Implements the Real‑Time Voice Interface protocol; maps RTVI client messages into frames (e.g. `send-text` → `TranscriptionFrame`), and maps server frames back into RTVI messages.
  - **Skill implication**:
    - Comfortable designing and debugging **agent‑like** flows that may include asynchronous events, multiple tools, and external orchestration runtimes.
    - Understanding how to surface errors and state transitions clearly over protocols like RTVI.

---

### 2.7 Evaluation & Quality

- **LLM eval runner — Intermediate**
  - **Files**: `cmd/evals`, `scripts/evals/README.md`, `scripts/evals/config/scenarios.json`.
  - **Role**: Go‑native evaluation harness for **LLM‑only** scenarios: sends prompts, validates responses via substrings or regex, and writes structured results.
  - **Patterns**:
    - Reuses Voxray config (`-voxray-config`) so provider/model settings match production.
    - Encodes scenarios in JSON, allowing simple, repeatable tests for correctness and regressions.
  - **Skill implication**:
    - Ability to design meaningful eval scenarios and interpret results, especially across different providers/models.
    - Understanding that these evals do not cover STT/TTS or transport, so additional testing is needed for full voice flows.

---

### 2.8 Onboarding Guidance (AI/ML Stack)

- **Recommended ramp‑up path**:
  - Start with `docs/ARCHITECTURE.md` and `docs/SYSTEM_ARCHITECTURE.md` to understand how STT → LLM → TTS is wired.
  - Read `pkg/services/README.md` (if present) and `services/factory.go` plus a few representative provider implementations (e.g. OpenAI, Groq, Sarvam).
  - Explore `pkg/frames/llm.go` and aggregator processors (`llmfullresponse`, `llmcontextsummarizer`, `userresponse`) to see how prompts, context, and summarization work in practice.
  - For tools/agents, study `pkg/mcp/client.go` and the OpenAI LLM service’s tool‑calling logic.
- **Difficulty assessment**:
  - This section is **High complexity** for onboarding: many providers, streaming patterns, and context/tool abstractions interact at once.
  - New contributors should budget **several days** to feel comfortable reading and modifying AI‑related code without breaking provider invariants. 

