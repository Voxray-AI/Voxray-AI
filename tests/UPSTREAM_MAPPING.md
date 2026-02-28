### Upstream test mapping

This document tracks which upstream Python tests from the reference test suite have
been conceptually ported into the Go test suite.

- **VAD / speech activity**
  - Upstream: `tests/test_vad_controller.py`
  - Go components: `pkg/audio/vad` analyzer + `pkg/processors/voice/turn.go`
  - Go tests:
    - `pkg/audio/vad/vad_test.go` (basic analyzer behavior)
    - `pkg/processors/voice/turn_test.go` (turn processing and VAD-driven user frames)

- **User turn controller / strategies**
  - Upstream: `tests/test_user_turn_controller.py`
  - Go components: `pkg/audio/turn` (user turn controller + strategies),
    `pkg/processors/voice/turn.go` (integration)
  - Go tests:
    - `pkg/processors/voice/turn_test.go` (user turn start events and frames)

- **WebSocket transport**
  - Upstream: `tests/test_websocket_transport.py`
  - Go components: `pkg/transport/websocket/websocket.go`
  - Go tests:
    - `tests/pkg/transport/websocket_server_test.go` (server, connection, basic frame IO)

- **Frames / serialization**
  - Upstream: `tests/test_protobuf_serializer.py`
  - Go components: `pkg/frames`, `pkg/frames/serialize`
  - Go tests:
    - `tests/pkg/frames/frames_test.go` (frame constructors and types)
    - `tests/pkg/frames/serialize_test.go` (JSON envelope encode/decode round-trips)

- **Pipeline behavior**
  - Upstream: `tests/test_pipeline.py`
  - Go components: pipeline and processors where applicable (subset)
  - Go tests:
    - `tests/pkg/pipeline/pipeline_flow_test.go` and related files

