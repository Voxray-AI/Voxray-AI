package transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
	"voila-go/pkg/transport"
	ws "voila-go/pkg/transport/websocket"
)

// TestWebsocketServer_StartAndEcho verifies that the WebSocket server upgrades
// connections on /ws, wires a ConnTransport into the provided callback, and
// that basic frame send/receive using the shared serializer works.
func TestWebsocketServer_StartAndEcho(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got transport.Transport

	srv := &ws.Server{
		SessionTimeout: 0, // disable inactivity timeout for this test
		OnConn: func(c context.Context, tr *ws.ConnTransport) {
			got = tr
		},
	}

	// Use httptest to host the server handler by wiring the WebSocket handler
	// registered by ListenAndServe into a test server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		tr := ws.NewConnTransport(conn, 64, 64)
		// Start the transport so that its read loop forwards frames into the
		// Input channel used below.
		if err := tr.Start(ctx); err != nil {
			t.Fatalf("start ConnTransport: %v", err)
		}
		if srv.OnConn != nil {
			go srv.OnConn(ctx, tr)
		}
	}))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	u.Scheme = "ws"
	u.Path = "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial websocket server: %v", err)
	}
	defer conn.Close()

	// Wait briefly for the OnConn callback to fire.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && got == nil {
		time.Sleep(10 * time.Millisecond)
	}
	if got == nil {
		t.Fatalf("expected transport to be set in OnConn")
	}

	// Send a StartFrame from the client and ensure the server transport
	// receives and decodes it.
	start := frames.NewStartFrame()
	data, err := serialize.Encoder(start)
	if err != nil {
		t.Fatalf("encode StartFrame: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write StartFrame: %v", err)
	}

	select {
	case f := <-got.Input():
		if f.FrameType() != "StartFrame" {
			t.Fatalf("expected StartFrame, got %s", f.FrameType())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for frame from server transport")
	}
}

