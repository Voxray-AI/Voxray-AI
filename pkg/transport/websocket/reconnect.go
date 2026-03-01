// Package websocket: WebsocketServiceBase provides reconnection, backoff, and send-with-retry
// for services that hold a long-lived WebSocket (e.g. realtime, Sarvam streaming).
package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/utils"
)

// WebSocketConnector is implemented by services that own a WebSocket connection.
// WebsocketServiceBase uses it to verify, reconnect, and run the receive loop.
type WebSocketConnector interface {
	// Conn returns the current WebSocket connection (may be nil).
	Conn() *websocket.Conn
	// SetConn sets the current connection (after Connect or reconnect).
	SetConn(conn *websocket.Conn)
	// Connect establishes a new connection and sets it via SetConn.
	Connect(ctx context.Context) error
	// Disconnect closes the current connection and clears it.
	Disconnect() error
	// ReceiveMessages runs the receive loop until error or connection close.
	// Called repeatedly by ReceiveLoop when reconnection is enabled.
	ReceiveMessages(ctx context.Context) error
}

// ReportErrorFunc is called to report connection errors (e.g. push ErrorFrame).
type ReportErrorFunc func(*frames.ErrorFrame)

// WebsocketServiceBase provides automatic reconnection with exponential backoff,
// connection verification (ping), and send-with-retry for WebSocket-based services.
// Embed or compose with a type that implements WebSocketConnector.
type WebsocketServiceBase struct {
	Connector         WebSocketConnector
	ReconnectOnError bool

	mu                 sync.Mutex
	reconnectInProgress bool
	disconnecting       bool
}

// NewWebsocketServiceBase returns a base with the given connector and reconnect policy.
func NewWebsocketServiceBase(connector WebSocketConnector, reconnectOnError bool) *WebsocketServiceBase {
	return &WebsocketServiceBase{
		Connector:         connector,
		ReconnectOnError:  reconnectOnError,
	}
}

// VerifyConnection pings the current connection. Returns false if conn is nil or closed.
func (b *WebsocketServiceBase) VerifyConnection() bool {
	conn := b.Connector.Conn()
	if conn == nil {
		return false
	}
	err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second))
	if err != nil {
		return false
	}
	return true
}

// Reconnect disconnects then connects and verifies. Returns true if connection is healthy after.
func (b *WebsocketServiceBase) Reconnect(ctx context.Context) bool {
	_ = b.Connector.Disconnect()
	if err := b.Connector.Connect(ctx); err != nil {
		logger.Error("websocket service reconnect connect: %v", err)
		return false
	}
	return b.VerifyConnection()
}

// TryReconnect attempts reconnection up to maxRetries with exponential backoff.
// If reportError is non-nil, it is called with an ErrorFrame on final failure.
// Returns true if reconnection and verification succeeded.
func (b *WebsocketServiceBase) TryReconnect(ctx context.Context, maxRetries int, reportError ReportErrorFunc) bool {
	b.mu.Lock()
	if b.reconnectInProgress {
		b.mu.Unlock()
		logger.Info("websocket service: reconnect already in progress")
		return false
	}
	b.reconnectInProgress = true
	b.mu.Unlock()
	defer func() { b.mu.Lock(); b.reconnectInProgress = false; b.mu.Unlock() }()

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.Info("websocket service: reconnecting, attempt %d", attempt)
		if b.Reconnect(ctx) {
			logger.Info("websocket service: reconnected on attempt %d", attempt)
			return true
		}
		lastErr = nil
		wait := utils.ExponentialBackoff(attempt)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(wait):
		}
	}
	msg := "websocket service: failed to reconnect after max retries"
	if lastErr != nil {
		msg += ": " + lastErr.Error()
	}
	logger.Error("%s", msg)
	if reportError != nil {
		reportError(frames.NewErrorFrame(msg, true, "websocket_service"))
	}
	return false
}

// SendWithRetry sends the message; on error tries reconnect once and retries send.
func (b *WebsocketServiceBase) SendWithRetry(ctx context.Context, messageType int, data []byte, reportError ReportErrorFunc) error {
	conn := b.Connector.Conn()
	if conn == nil {
		return nil
	}
	if err := conn.WriteMessage(messageType, data); err != nil {
		logger.Error("websocket service send failed: %v, will try reconnect", err)
		if b.TryReconnect(ctx, 3, reportError) {
			if c := b.Connector.Conn(); c != nil {
				return c.WriteMessage(messageType, data)
			}
		}
		return err
	}
	return nil
}

// MaybeTryReconnect decides whether to reconnect after an error or graceful close.
// Returns true if the caller should continue the receive loop (reconnect succeeded).
func (b *WebsocketServiceBase) MaybeTryReconnect(ctx context.Context, message string, reportError ReportErrorFunc, err error) bool {
	b.mu.Lock()
	disconnecting := b.disconnecting
	b.mu.Unlock()
	if disconnecting {
		if err != nil {
			logger.Info("websocket service error during disconnect: %v", err)
		}
		return false
	}
	logger.Info("%s", message)
	if b.ReconnectOnError {
		return b.TryReconnect(ctx, 3, reportError)
	}
	if reportError != nil {
		reportError(frames.NewErrorFrame(message, false, "websocket_service"))
	}
	return false
}

// ReceiveLoop runs ReceiveMessages in a loop, reconnecting on error when ReconnectOnError is true.
// reportError is called for connection errors when not reconnecting.
// Stops when context is canceled, Disconnect is called, or reconnection is disabled and an error occurs.
func (b *WebsocketServiceBase) ReceiveLoop(ctx context.Context, reportError ReportErrorFunc) {
	for {
		err := b.Connector.ReceiveMessages(ctx)
		if err != nil {
			// Check for normal closure
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.Debug("websocket service connection closed normally: %v", err)
				break
			}
			msg := "websocket service error receiving: " + err.Error()
			if !b.MaybeTryReconnect(ctx, msg, reportError, err) {
				break
			}
			continue
		}
		// ReceiveMessages returned nil (e.g. graceful close by server)
		if !b.MaybeTryReconnect(ctx, "websocket service connection closed by server", reportError, nil) {
			break
		}
	}
}

// SetDisconnecting sets whether the service is intentionally disconnecting (disables reconnect).
func (b *WebsocketServiceBase) SetDisconnecting(disconnecting bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.disconnecting = disconnecting
}

// Disconnect marks disconnecting and closes the connection.
func (b *WebsocketServiceBase) Disconnect() error {
	b.SetDisconnecting(true)
	return b.Connector.Disconnect()
}
