// Package websocket provides a WebSocket transport for Voila (one connection = one session).
package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
	"voila-go/pkg/logger"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ConnTransport handles a single WebSocket connection as a Voila transport.
// It manages the bidirectional flow of frames between the server and the client,
// handling serialization and deserialization automatically.
type ConnTransport struct {
	conn   *websocket.Conn
	inCh   chan frames.Frame
	outCh  chan frames.Frame
	closed chan struct{}
	once   sync.Once
}

// NewConnTransport creates a transport for an already-upgraded WebSocket connection.
func NewConnTransport(conn *websocket.Conn, inBuf, outBuf int) *ConnTransport {
	if inBuf <= 0 {
		inBuf = 64
	}
	if outBuf <= 0 {
		outBuf = 64
	}
	return &ConnTransport{
		conn:   conn,
		inCh:   make(chan frames.Frame, inBuf),
		outCh:  make(chan frames.Frame, outBuf),
		closed: make(chan struct{}),
	}
}

// Input returns the channel of frames received from the client.
func (t *ConnTransport) Input() <-chan frames.Frame { return t.inCh }

// Output returns the channel to send frames to the client.
func (t *ConnTransport) Output() chan<- frames.Frame { return t.outCh }

// Start starts the read and write loops. It returns once the connection is set up (no error).
// The provided context is used to drive shutdown; when it is canceled, the transport is closed.
func (t *ConnTransport) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("websocket: nil context passed to ConnTransport.Start")
	}

	go t.readLoop()
	go t.writeLoop()
	go func() {
		select {
		case <-ctx.Done():
			_ = t.Close()
		case <-t.closed:
		}
	}()
	return nil
}

// Close closes the connection and channels.
func (t *ConnTransport) Close() error {
	var err error
	t.once.Do(func() {
		close(t.closed)
		if t.conn != nil {
			err = t.conn.Close()
		}
		close(t.inCh)
		close(t.outCh)
	})
	return err
}

func (t *ConnTransport) readLoop() {
	defer func() { _ = t.Close() }()
	for {
		_, data, err := t.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.Error("websocket read: %v", err)
			}
			return
		}
		f, err := serialize.Decoder(data)
		if err != nil {
			logger.Error("decode frame: %v", err)
			continue
		}
		select {
		case <-t.closed:
			return
		case t.inCh <- f:
		}
	}
}

func (t *ConnTransport) writeLoop() {
	defer func() { _ = t.Close() }()
	for {
		select {
		case <-t.closed:
			return
		case f, ok := <-t.outCh:
			if !ok {
				return
			}
			data, err := serialize.Encoder(f)
			if err != nil {
				logger.Error("encode frame: %v", err)
				continue
			}
			if err := t.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				logger.Error("websocket write: %v", err)
				return
			}
		}
	}
}

// Server is a specialized HTTP server that upgrades incoming connections to WebSockets
// and initializes a ConnTransport for each session.
type Server struct {
	Host string
	Port int
	// OnConn is called for each new connection; it receives the transport which should be linked to a pipeline.
	OnConn func(ctx context.Context, tr *ConnTransport)
}

// ListenAndServe starts the HTTP server and blocks.
func (s *Server) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("upgrade: %v", err)
			return
		}
		tr := NewConnTransport(conn, 64, 64)
		if s.OnConn != nil {
			go s.OnConn(ctx, tr)
		}
	})

	port := s.Port
	if port == 0 {
		port = 8080
	}
	addr := fmt.Sprintf(":%d", port)
	if s.Host != "" {
		addr = fmt.Sprintf("%s:%d", s.Host, port)
	}

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			logger.Error("websocket server shutdown: %v", err)
		}
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("websocket server listen: %w", err)
	}
	return nil
}
