# WebSocket service base (reconnection and backoff)

Services that hold a long-lived WebSocket connection (e.g. OpenAI Realtime, Sarvam streaming) can use the **WebsocketServiceBase** helper for automatic reconnection, exponential backoff, connection verification (ping), and send-with-retry. This mirrors the upstream Pipecat [websocket_service.py](https://github.com/pipecat-ai/pipecat/blob/main/src/pipecat/services/websocket_service.py).

## Location

- **Reconnect logic:** [pkg/transport/websocket/reconnect.go](../pkg/transport/websocket/reconnect.go)
- **Backoff:** [pkg/utils/backoff.go](../pkg/utils/backoff.go)

## Usage

1. Implement **WebSocketConnector** in your service:
   - `Conn()` / `SetConn(conn)` – current WebSocket connection
   - `Connect(ctx)` – establish a new connection (e.g. dial)
   - `Disconnect()` – close and clear the connection
   - `ReceiveMessages(ctx)` – run the receive loop until error or close

2. Create a **WebsocketServiceBase** with your connector and reconnect policy:
   - `NewWebsocketServiceBase(connector, reconnectOnError)`

3. Use the base methods:
   - **VerifyConnection()** – ping the current connection
   - **TryReconnect(ctx, maxRetries, reportError)** – reconnect with exponential backoff
   - **SendWithRetry(ctx, messageType, data, reportError)** – send, retry after reconnect on failure
   - **ReceiveLoop(ctx, reportError)** – run ReceiveMessages in a loop, reconnecting on error when enabled
   - **SetDisconnecting(true)** before shutdown so reconnection is skipped

## Integration

- **OpenAI Realtime** ([pkg/realtime/openairealtime.go](../pkg/realtime/openairealtime.go)) and **Sarvam streaming** ([pkg/services/sarvam/stt_streaming.go](../pkg/services/sarvam/stt_streaming.go), [tts_streaming.go](../pkg/services/sarvam/tts_streaming.go)) can be refactored to embed or compose WebsocketServiceBase for resilient reconnection. Currently they connect once without automatic reconnect.
