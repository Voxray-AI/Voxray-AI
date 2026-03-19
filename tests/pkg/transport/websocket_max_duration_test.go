package transport_test

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/frames/serialize"
	ws "voxray-go/pkg/transport/websocket"
)

func TestWebsocketMaxDurationAfterFirstAudio(t *testing.T) {
	const maxDur = 50 * time.Millisecond

	startedCh := make(chan struct{})
	connCtxDone := make(chan struct{})
	trDone := make(chan struct{})

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()

	srv := &ws.Server{
		SessionTimeout: 0,
		MaxDurationAfterFirstAudio: maxDur,
		OnConn: func(c context.Context, tr *ws.ConnTransport) {
			// Start read/write loops so the transport receives frames from the client.
			if err := tr.Start(c); err != nil {
				t.Errorf("transport start: %v", err)
				return
			}
			close(startedCh)
			go func() {
				<-c.Done()
				close(connCtxDone)
			}()
			go func() {
				<-tr.Done()
				close(trDone)
			}()
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ServeWithListener(srvCtx, l)
	}()

	u := url.URL{
		Scheme:   "ws",
		Host:     l.Addr().String(),
		Path:     "/ws",
		RawQuery: "format=protobuf",
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	select {
	case <-startedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for transport to start")
	}

	// 1) Send StartFrame: should not start the max-duration timer (timer starts on first AudioRawFrame).
	ps := serialize.ProtobufSerializer{}
	start := frames.NewStartFrame()
	startData, err := ps.Serialize(start)
	if err != nil {
		t.Fatalf("encode StartFrame: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, startData); err != nil {
		t.Fatalf("write StartFrame: %v", err)
	}

	select {
	case <-connCtxDone:
		t.Fatal("conn context canceled too early (before inbound AudioRawFrame)")
	case <-time.After(maxDur / 2):
		// ok: not canceled yet
	}

	// 2) Send first inbound audio frame: should trigger cancellation after maxDur.
	// Keep the payload small; we just need a deserializable AudioRawFrame for the timer start.
	pcm := make([]byte, 320)
	audioFrame := frames.NewAudioRawFrame(pcm, 16000, 1, 0)
	audioData, err := ps.Serialize(audioFrame)
	if err != nil {
		t.Fatalf("encode AudioRawFrame: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, audioData); err != nil {
		t.Fatalf("write AudioRawFrame: %v", err)
	}

	select {
	case <-connCtxDone:
		// ok
	case <-time.After(5 * maxDur):
		t.Fatalf("timeout waiting for conn context cancel after first AudioRawFrame (maxDur=%s)", maxDur)
	}

	select {
	case <-trDone:
		// ok
	case <-time.After(2 * maxDur):
		t.Fatalf("timeout waiting for transport Done() after cancellation (maxDur=%s)", maxDur)
	}

	cancelSrv()
	select {
	case err := <-errCh:
		// ServeWithListener returns only after ctx cancellation; ignore expected close errors.
		_ = err
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for server shutdown")
	}
}

