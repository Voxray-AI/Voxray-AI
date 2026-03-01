package main

import (
	"context"
	"log"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/transport/websocket"
)

// Simple example client that connects to a running voxray-go server over
// WebSocket and sends a StartFrame followed by a TTSSpeakFrame, then logs
// any frames received from the server.
func main() {
	tr, err := websocket.NewClientTransport("ws://localhost:8080/ws", nil)
	if err != nil {
		log.Fatalf("new client transport: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tr.Start(ctx); err != nil {
		log.Fatalf("start client transport: %v", err)
	}

	// Send a basic StartFrame so the server-side pipeline can initialize.
	start := frames.NewStartFrame()
	select {
	case tr.Output() <- start:
	case <-time.After(5 * time.Second):
		log.Fatalf("timeout sending StartFrame")
	}

	// Ask TTS to speak a simple phrase.
	speak := frames.NewTTSSpeakFrame("Hello from WebSocket client")
	select {
	case tr.Output() <- speak:
	case <-time.After(5 * time.Second):
		log.Fatalf("timeout sending TTSSpeakFrame")
	}

	// Read a few frames from the server and log them.
	timeout := time.After(30 * time.Second)
	for {
		select {
		case f, ok := <-tr.Input():
			if !ok {
				log.Printf("input channel closed")
				return
			}
			log.Printf("received frame type=%s id=%d", f.FrameType(), f.ID())
		case <-timeout:
			log.Printf("timeout waiting for frames; exiting")
			return
		}
	}
}

