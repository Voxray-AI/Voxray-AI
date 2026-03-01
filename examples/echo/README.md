# Echo example

Run the Voxray server with echo pipeline (frames received are echoed back):

```bash
cd ../..
go run ./cmd/voxray --config config.json
```

Then connect a WebSocket client to `ws://localhost:8080/ws` and send JSON frame envelopes (see pkg/frames/serialize). The server will push frames through the echo processor and send them back.
