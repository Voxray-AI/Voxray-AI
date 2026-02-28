# WebRTC Voice Client

A minimal JavaScript WebRTC client that captures live microphone input and streams it to the Voila backend over WebRTC. The server runs the voice pipeline (Sarvam STT → Groq LLM → Sarvam TTS) and returns synthesized audio; the client plays it back.

## Prerequisites

- Voila server running with **WebRTC** and **Sarvam + Groq** config (see [examples/voice/README.md](../../examples/voice/README.md)).
- Browser with WebRTC and `getUserMedia` support (Chrome, Firefox, Edge, Safari).

## Building with voice (Opus)

The WebRTC outbound path sends TTS as Opus over the peer connection. That requires the Opus encoder, which is only built when **CGO** is enabled and a C compiler is available.

- **Windows (PowerShell):** Use the voice scripts from repo root:
  - **Build:** `.\scripts\build-voice.ps1` → produces `voila.exe` with Opus. Run with `.\voila.exe -config config.json`.
  - **Run (no build):** `.\scripts\run-voice.ps1 -config config.json`
  - You need **gcc** on PATH: install [MSYS2](https://www.msys2.org/) (e.g. `pacman -S mingw-w64-ucrt-x86_64-toolchain`) or [Winlibs](https://winlibs.com/) MinGW-w64, then add the toolchain `bin` to PATH. Verify with `gcc --version`.
- **Linux/macOS:** From repo root, `make build-voice` then run the binary, or `make run-voice ARGS="-config config.json"`.

If the Opus encoder is not available, the server **returns 503** and the client shows the error (e.g. *"opus encoder unavailable (build without cgo); TTS audio cannot be sent..."*). Use the voice build/run above to fix it.

## Usage

1. **Start the server** (from repo root). For **voice (TTS)** use the CGO build:
   - **With voice:** `.\scripts\run-voice.ps1 -config config.json` (Windows) or `make run-voice ARGS="-config config.json"` (Unix)
   - **Without voice:** `go run ./cmd/voila -config config.json` (WebRTC connect will get 503)
   Use a `config.json` with `"transport": "both"`, `stt_provider`/`llm_provider`/`tts_provider` set to `sarvam`/`groq`/`sarvam`, and the required API keys.

2. **Open the client**  
   - **Recommended:** Serve the client so the browser doesn’t block requests (e.g. CORS or `file://` restrictions). From repo root: `cd tests/frontend && python -m http.server 3000`, then open **http://localhost:3000/webrtc-voice.html** in your browser.
   - Or open `webrtc-voice.html` directly; the server now sends CORS headers so the client can reach it from any origin.

3. **Connect**  
   - Set **Server URL** to the server base URL (e.g. `http://localhost:8080`).
   - Click **Start**, allow microphone access when prompted.
   - Speak; the assistant reply is played through the **remote audio** element.

4. **Disconnect**  
   - Click **Stop** to close the peer connection and release the microphone.

## Flow

- **Start**: `getUserMedia` → create `RTCPeerConnection` → add local audio track → `createOffer` → `setLocalDescription` → `POST /webrtc/offer` with `{ "offer": "<SDP>" }` → receive `{ "answer": "<SDP>" }` → `setRemoteDescription` → wait for `ontrack`.
- **Remote audio**: On `ontrack`, the remote stream is attached to the `<audio>` element and played.
- **Stop**: Peer connection and local tracks are closed.

## Troubleshooting

- **"Cannot reach http://localhost:8080"** – The Voila server is not running or not reachable. Start it from the repo root: `go run ./cmd/voila -config config.json`. Ensure `config.json` has `"transport": "both"` (or `"smallwebrtc"`). When the server is up you should see: `starting server on localhost:8080 (transport=both)`.
- **`cgo: C compiler "gcc" not found`** – CGO needs a C compiler. **Quick fix (winget):** run `winget install BrechtSanders.WinLibs.POSIX.UCRT --accept-package-agreements --accept-source-agreements`; then **close and reopen** your terminal so PATH is updated (or in the same session add the WinLibs `bin` folder to PATH, e.g. under `%LOCALAPPDATA%\Microsoft\WinGet\Packages\...\mingw64\bin`). **Alternative:** Install [MSYS2](https://www.msys2.org/) (or `winget install MSYS2.MSYS2`), open **MSYS2 UCRT64**, run `pacman -S mingw-w64-ucrt-x86_64-toolchain`, then add `C:\msys64\ucrt64\bin` to your user PATH. Verify with `gcc --version`.
- **503 Service Unavailable / "opus encoder unavailable"** – The server is built without the Opus encoder (no CGO). **Fix:** Use the voice build: Windows: `.\scripts\build-voice.ps1` then run `.\voila.exe -config config.json`, or `.\scripts\run-voice.ps1 -config config.json`. See [Building with voice (Opus)](#building-with-voice-opus).
- **Port already in use** – If you see `bind: address already in use`, change `port` in `config.json` and use that URL in the client (e.g. `http://localhost:8081`).
- **Connecting from another machine** – Use the server machine’s IP in the client and set `"host": "0.0.0.0"` in `config.json` so the server listens on all interfaces.

## Files

- **webrtc-voice.html** – Single-file client (HTML + CSS + JS). No build step.
