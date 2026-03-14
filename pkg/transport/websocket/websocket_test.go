package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"voxray-go/pkg/frames"
)

// TestWriteCoalescing_Disabled verifies that when WriteCoalesceMs is 0, each frame results in exactly one write.
func TestWriteCoalescing_Disabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		_ = conn
		// Server holds conn open; readLoop will block until client closes.
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	tr := NewConnTransport(conn, 64, 64, nil, nil)
	const N = 5
	var writeCount int32
	done := make(chan struct{})
	var once sync.Once
	tr.WriteMessageFunc = func(messageType int, data []byte) error {
		n := atomic.AddInt32(&writeCount, 1)
		if n == N {
			once.Do(func() { close(done) })
		}
		return nil
	}

	if err := tr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tr.Close()

	for i := 0; i < N; i++ {
		tr.Output() <- frames.NewTextFrame("x")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for writes")
	}
	if n := atomic.LoadInt32(&writeCount); n != N {
		t.Errorf("with coalescing disabled: expected %d writes, got %d", N, n)
	}
}

// TestWriteCoalescing_Enabled_NoPanic is a smoke test: when WriteCoalesceMs > 0, the coalescing path runs,
// all N frames are written (current implementation does one write per frame, batched drain only), and no panic.
// If a future change batches multiple frames into a single WriteMessage, update this test to assert n < N.
func TestWriteCoalescing_Enabled_NoPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		_ = conn
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	tr := NewConnTransport(conn, 64, 64, nil, &ConnTransportOptions{WriteCoalesceMs: 10, WriteCoalesceMaxFrames: 10})
	const N = 8
	var writeCount int32
	done := make(chan struct{})
	var once sync.Once
	tr.WriteMessageFunc = func(messageType int, data []byte) error {
		n := atomic.AddInt32(&writeCount, 1)
		if n == N {
			once.Do(func() { close(done) })
		}
		return nil
	}

	if err := tr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tr.Close()

	for i := 0; i < N; i++ {
		tr.Output() <- frames.NewTextFrame("y")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for writes")
	}
	if n := atomic.LoadInt32(&writeCount); n != N {
		t.Errorf("with coalescing enabled: expected %d writes (one per frame), got %d", N, n)
	}
}
