// Package websocket provides WebSocket transport (server and client) for Voila.
package websocket

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"

	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
	"voila-go/pkg/logger"
)

// ClientTransport is an outbound WebSocket transport that connects to a pipeline server.
// It implements transport.Transport: Input receives frames from the server, Output sends frames to the server.
type ClientTransport struct {
	url    *url.URL
	conn   *websocket.Conn
	inCh   chan frames.Frame
	outCh  chan frames.Frame
	closed chan struct{}
	once   sync.Once
}

// ClientConfig configures the WebSocket client transport.
type ClientConfig struct {
	InBufSize  int
	OutBufSize int
}

// NewClientTransport creates a client transport that will connect to the given WebSocket URL (e.g. ws://host/ws or wss://host/ws).
// Connect is performed when Start is called. If inBufSize or outBufSize are <= 0, 64 is used.
func NewClientTransport(wsURL string, cfg *ClientConfig) (*ClientTransport, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("websocket client: invalid URL: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("websocket client: URL scheme must be ws or wss, got %q", u.Scheme)
	}
	inBuf, outBuf := 64, 64
	if cfg != nil {
		if cfg.InBufSize > 0 {
			inBuf = cfg.InBufSize
		}
		if cfg.OutBufSize > 0 {
			outBuf = cfg.OutBufSize
		}
	}
	return &ClientTransport{
		url:    u,
		inCh:   make(chan frames.Frame, inBuf),
		outCh:  make(chan frames.Frame, outBuf),
		closed: make(chan struct{}),
	}, nil
}

// Input returns the channel of frames received from the server.
func (t *ClientTransport) Input() <-chan frames.Frame { return t.inCh }

// Output returns the channel to send frames to the server.
func (t *ClientTransport) Output() chan<- frames.Frame { return t.outCh }

// Start dials the WebSocket URL and starts read/write loops. Returns an error if the connection fails.
// The provided context is used for the dial and for shutdown; when it is canceled, the transport is closed.
func (t *ClientTransport) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("websocket client: nil context passed to Start")
	}
	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, t.url.String(), nil)
	if err != nil {
		return fmt.Errorf("websocket client: dial %s: %w", t.url.String(), err)
	}
	t.conn = conn
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
func (t *ClientTransport) Close() error {
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

func (t *ClientTransport) readLoop() {
	defer func() { _ = t.Close() }()
	for {
		_, data, err := t.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.Error("websocket client read: %v", err)
			}
			return
		}
		f, err := serialize.Decoder(data)
		if err != nil {
			logger.Error("websocket client decode frame: %v", err)
			continue
		}
		select {
		case <-t.closed:
			return
		case t.inCh <- f:
		}
	}
}

func (t *ClientTransport) writeLoop() {
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
				logger.Error("websocket client encode frame: %v", err)
				continue
			}
			if err := t.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				logger.Error("websocket client write: %v", err)
				return
			}
		}
	}
}
