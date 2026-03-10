### Upstream test mapping

This document tracks which upstream Python tests (see [upstream test tree](https://github.com/pipecat-ai/pipecat/tree/main/tests) — upstream Pipecat-AI repo) have been conceptually ported into the Voxray-Go test suite. This project is **Voxray** / **Voxray-AI**; the link is retained for historical test mapping.

**Status legend:** **Ported** = Go test(s) exist and cover the same behavior; **Partial** = some scenarios ported, extend Go tests; **To port** = Go code exists, add/extend tests; **N/A** = no Go equivalent (Python-only SDK/feature).

---

#### VAD / speech activity

- **test_vad_controller.py** — **Ported**. Go: `pkg/audio/vad`, `pkg/processors/voice/turn.go`. Tests: `pkg/audio/vad/vad_test.go`, `pkg/processors/voice/turn_test.go`.
- **test_vad_processor.py** — **Ported**. Go: `pkg/processors/audio` VAD processor. Tests: `tests/pkg/processors/audio/vad_processor_test.go` (start/stop/forwards, STARTING/STOPPING, quiet-only, StartingThenSpeaking).

#### User turn / strategies

- **test_user_turn_controller.py** — **Ported**. Go: `pkg/audio/turn`, `pkg/processors/voice/turn.go`. Tests: `pkg/processors/voice/turn_test.go`.
- **test_user_turn_processor.py**, **test_user_turn_start_strategy.py**, **test_user_turn_stop_strategy.py**, **test_user_turn_completion_mixin.py** — **Partial**. Same Go components; covered by turn_test.go.
- **test_user_idle_controller.py**, **test_user_idle_processor.py** — **Partial**. Go: `pkg/audio/turn`. Documented; add tests for idle behavior if exposed.
- **test_user_mute_strategy.py** — **Partial**. Go: `pkg/audio/turn` strategies. Documented.

#### Aggregators

- **test_aggregators.py** (Sentence + Gated) — **Ported**. Sentence: `tests/pkg/processors/aggregator/aggregator_test.go`. Gated: `tests/pkg/processors/aggregators/gated/gated_test.go`.
- **test_simple_text_aggregator.py** — **Ported**. Go: `pkg/processors/aggregator`, `pkg/utils/textaggregator/sentence`. Covered by aggregator_test.go.
- **test_pattern_pair_aggregator.py** — **Ported**. Go: `pkg/utils/patternaggregator`. Tests: `tests/pkg/utils/patternaggregator/patternaggregator_test.go`.
- **test_dtmf_aggregator.py** — **Ported**. Go: `pkg/processors/aggregators/dtmf`. Tests: `tests/pkg/processors/aggregators/dtmf/dtmf_test.go`.
- **test_skip_tags_aggregator.py** — **N/A** (or **Partial** if Go has skip-tags logic in aggregators; no dedicated processor found).
- **test_context_aggregators.py**, **test_context_aggregators_universal.py**, **test_context_summarization.py** — **Ported**. Go: `pkg/processors/aggregators/llmcontextsummarizer`, `pkg/processors/aggregators/gatedcontext`. Tests: `tests/pkg/processors/aggregators/llmcontextsummarizer/summarizer_test.go`, `tests/pkg/processors/aggregators/gatedcontext/gatedcontext_test.go`.
- **test_llm_context_summarizer.py** — **Ported**. Go: `pkg/processors/aggregators/llmcontextsummarizer`. Tests: `tests/pkg/processors/aggregators/llmcontextsummarizer/summarizer_test.go`.

#### Filters

- **test_filters.py** (Identity, FrameFilter, FunctionFilter, WakeCheck) — **Ported**. Identity/FrameFilter: `tests/pkg/processors/filters/filter_test.go`. WakeCheck: `tests/pkg/processors/filters/wake_check_filter_test.go`. FunctionFilter: **N/A** (no function-based filter in Go).
- **test_aic_filter.py**, **test_aic_vad.py** — **N/A**. Python AIC-specific; no Go AIC implementation.
- **test_markdown_text_filter.py**, **test_stt_mute_filter.py** — **N/A** (or add tests if Go gains these processors).
- **test_noisereduce_filter.py** — **Partial**. Go: `pkg/audio/filters`, `pkg/processors/audio/audio_filter_processor.go`. Tests: `tests/pkg/processors/audio/audio_filter_processor_test.go` (basic gain-style filter chain; no external noisereduce dependency).

#### Pipeline / frame processing

- **test_pipeline.py** — **Ported**. Go: `pkg/pipeline`. Tests: `tests/pkg/pipeline/pipeline_test.go`, `pipeline_flow_test.go` (PipelineTask queue, Run, HasFinished, Cancel, CancelFrame propagation).
- **test_frame_processor.py** — **Partial**. Base processor behavior covered by pipeline and filter tests.
- **test_producer_consumer.py** — **Partial**. Pipeline flow tests cover producer/consumer style.

#### Audio

- **test_audio_buffer_processor.py** — **Ported**. Go: `pkg/processors/audio`. Tests: `tests/pkg/processors/audio/audio_buffer_processor_test.go`.
- **test_rnnoise_cancellation.py**, **test_rnnoise_filter.py**, **test_rnnoise_resampling.py** — **N/A**. RNNoise is Python/native; no Go equivalent.

#### Observers

- **test_turn_trace_observer.py**, **test_turn_tracking_observer.py** — **Ported**. Go: `pkg/observers`. Tests: `tests/pkg/observers/observer_test.go`, `metrics_test.go` (turn tracking, multiple turns).
- **test_user_bot_latency_observer.py** — **Ported**. Go: `pkg/observers/user_bot_latency.go`. Tests: `tests/pkg/observers/observer_test.go` (OnLatencyMeasured, no callback when bot starts without user stop).

#### Transports

- **test_websocket_transport.py** — **Ported**. Go: `pkg/transport/websocket`. Tests: `tests/pkg/transport/websocket_server_test.go`.
- **test_daily_transport_service.py** — **N/A**. Daily SDK is Python-specific.
- **test_fastapi_websocket.py** — **N/A**. FastAPI is Python-specific.
- **test_livekit_transport.py** — **N/A**. LiveKit transport is Python-specific; Go has no equivalent in this repo.

#### Frames / serialization

- **test_protobuf_serializer.py** — **Ported**. Go: `pkg/frames`, `pkg/frames/serialize`, `pkg/frames/proto/wire`. Tests: `tests/pkg/frames/frames_test.go`, `serialize_test.go`, `tests/pkg/frames/proto/wire/wire_test.go`.

#### LLM / services

- **test_google_llm_openai.py**, **test_google_utils.py** — **N/A**. Google LLM helpers are Python-specific; Go has different client structure.
- **test_get_llm_invocation_params.py**, **test_llm_response.py**, **test_openai_llm_timeout.py** — **N/A** (or **Partial** if Go has equivalent invocation/response/timeout logic; add tests in service packages).
- **test_sambanova_llm.py** — **N/A**. Sambanova is not in Go pkg/services.
- **test_service_switcher.py** — **Ported**. Go: `pkg/pipeline/service_switcher.go`. Tests: `tests/pkg/pipeline/service_switcher_test.go` (initial active, switch to B, invalid switch, LLMSwitcher).

#### Adapters / tools

- **test_direct_functions.py**, **test_function_calling_adapters.py** — **Ported**. Go: `pkg/adapters/schemas`. Tests: `tests/pkg/adapters/schemas/toolschema_test.go` (schema round-trip, parameters, CustomTools by AdapterType).

#### Config / utils

- **test_settings.py** — **Ported**. Go: `pkg/config`. Tests: `tests/pkg/config/config_test.go`.
- **test_utils_network.py**, **test_utils_string.py** — **N/A** (or add under pkg/utils if Go has equivalent helpers).

#### Other

- **test_tracing_context.py** — **N/A**. Tracing context is Python-specific.
- **test_transcript_processor.py** — **N/A** (or **Partial** if Go has transcript processing).
- **test_run_inference.py**, **test_runner_utils.py** — **N/A**. Python runner/inference utilities.
- **test_piper_tts.py** — **N/A**. Piper TTS is Python-specific.
- **test_krisp_sdk_manager.py**, **test_krisp_viva_filter.py** — **N/A**. Krisp SDK is Python-specific.
- **test_langchain.py** — **N/A**. LangChain is Python-specific.
- **test_ivr_navigation.py** — **Ported**. Go: `pkg/extensions/ivr`. Tests: `tests/pkg/extensions/ivr/ivr_test.go` (DTMF, status completed, navigation multiple DTMF).
- **test_interruption_strategies.py** — **Partial**. Go: `pkg/audio/turn`, `pkg/audio/interruptions`, `pkg/processors/voice/interruption_controller.go`. Tests: `tests/pkg/audio/interruptions/interruptions_test.go` (min-words strategy behavior).

#### Subdirs

- **genesys/test_genesys_serializer.py** — **N/A**. Genesys serializer is Python-specific; Go has no Genesys serializer in this repo.
- **integration/test_integration_unified_function_calling.py** — **N/A** (or **Partial** if Go has unified function-calling integration tests).
