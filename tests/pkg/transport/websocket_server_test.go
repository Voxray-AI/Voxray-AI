package transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/frames/serialize"
	"voxray-go/pkg/transport"
	ws "voxray-go/pkg/transport/websocket"
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
		tr := ws.NewConnTransport(conn, 64, 64, nil, nil)
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

// TestWebsocketServer_ServerToClient verifies that frames sent via the
// ConnTransport.Output channel are serialized and delivered to the WebSocket
// client, mirroring the server-to-client direction covered in the upstream
// Python tests for the websocket transport.
func TestWebsocketServer_ServerToClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var tr *ws.ConnTransport

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		tr = ws.NewConnTransport(conn, 64, 64, nil, nil)
		if err := tr.Start(ctx); err != nil {
			t.Fatalf("start ConnTransport: %v", err)
		}
	}))
	defer ts.Close()

	conn, closeConn := newTestWebSocketClient(t, ts)
	defer closeConn()

	// Wait briefly for transport to be initialized.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && tr == nil {
		time.Sleep(10 * time.Millisecond)
	}
	if tr == nil {
		t.Fatalf("expected ConnTransport to be initialized")
	}

	// Send a TextFrame from the server side and verify the client receives it.
	tf := frames.NewTextFrame("from server")
	data, err := serialize.Encoder(tf)
	if err != nil {
		t.Fatalf("encode TextFrame: %v", err)
	}

	tr.Output() <- tf

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("client ReadMessage: %v", err)
	}
	if string(msg) != string(data) {
		t.Fatalf("unexpected payload from server: got %q, want %q", string(msg), string(data))
	}
}

// newTestWebSocketClient dials the given httptest.Server over WebSocket and
// returns the live connection and a cancel func to close it. This is a small
// helper used to express additional websocket transport scenarios succinctly.
func newTestWebSocketClient(t *testing.T, ts *httptest.Server) (*websocket.Conn, func()) {
	t.Helper()
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
	cancel := func() { _ = conn.Close() }
	return conn, cancel
}

