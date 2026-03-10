// Package websocket provides a WebSocket transport for Voxray (one connection = one session).
package websocket

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/frames/serialize"
	"voxray-go/pkg/logger"
)

// checkOrigin allows same-origin, same-host, or localhost/127.0.0.1 for development.
// For production, restrict to your front-end origin to avoid cross-site WebSocket abuse.
func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := strings.ToLower(originURL.Hostname())
	if originHost == "localhost" || originHost == "127.0.0.1" {
		return true
	}
	reqHost := r.Host
	if idx := strings.Index(reqHost, ":"); idx != -1 {
		reqHost = reqHost[:idx]
	}
	return strings.ToLower(reqHost) == originHost
}

var upgrader = websocket.Upgrader{
	CheckOrigin: checkOrigin,
}

// Upgrade upgrades the HTTP connection to WebSocket and returns the connection.
// Used by server for custom handlers (e.g. telephony) that need to read handshake messages before creating ConnTransport.
func Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return upgrader.Upgrade(w, r, nil)
}

// ConnTransport handles a single WebSocket connection as a Voxray transport.
// It exposes Input (frames from client) and Output (frames to client) and closes when the connection ends or Close is called.
// Safe for multiple goroutines reading Input, writing to Output, or calling Done; Close is idempotent and must not be called concurrently with Send (sending on Output after Close may panic).
type ConnTransport struct {
	conn       *websocket.Conn
	serializer serialize.Serializer
	inCh       chan frames.Frame
	outCh      chan frames.Frame
	closed     chan struct{}
	once       sync.Once

	// lastActivity holds the last time we saw activity on this connection
	// (either a successfully read frame from the client or a successfully
	// written frame to the client), stored as Unix nano time.
	lastActivity atomic.Int64
}

// DefaultReadLimit is the maximum WebSocket message size in bytes (1MB). Prevents memory exhaustion from oversized frames.
const DefaultReadLimit = 1 << 20

// NewConnTransport builds a transport for an already-upgraded WebSocket connection.
// If serializer is nil, JSON text messages are used. inBuf and outBuf set channel sizes; zero or negative values default to 64.
// The caller must not use conn for reads or writes after passing it here.
func NewConnTransport(conn *websocket.Conn, inBuf, outBuf int, serializer serialize.Serializer) *ConnTransport {
	conn.SetReadLimit(DefaultReadLimit)
	if inBuf <= 0 {
		inBuf = 64
	}
	if outBuf <= 0 {
		outBuf = 64
	}
	if serializer == nil {
		serializer = serialize.JSONSerializer{}
	}
	t := &ConnTransport{
		conn:       conn,
		serializer: serializer,
		inCh:       make(chan frames.Frame, inBuf),
		outCh:      make(chan frames.Frame, outBuf),
		closed:     make(chan struct{}),
	}
	// Initialize last activity to now so that newly created transports
	// are considered active until we see the first message.
	t.touch()
	return t
}

// Input returns the channel of frames received from the client.
// The channel is closed when the transport is closed. Receive-only; safe to read from multiple goroutines.
func (t *ConnTransport) Input() <-chan frames.Frame { return t.inCh }

// Output returns the channel to send frames to the client.
// Do not send after calling Close; the channel is closed on Close.
func (t *ConnTransport) Output() chan<- frames.Frame { return t.outCh }

// Done returns a channel that is closed when the transport has shut down.
// Safe to select from multiple goroutines.
func (t *ConnTransport) Done() <-chan struct{} { return t.closed }

// LastActivity returns the last time a frame was successfully read from or written to the client.
// Returns zero time if no activity has been recorded. Used for session timeouts.
func (t *ConnTransport) LastActivity() time.Time {
	ns := t.lastActivity.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// touch records the current time as the last activity time.
func (t *ConnTransport) touch() {
	t.lastActivity.Store(time.Now().UnixNano())
}

// Start starts the read and write loops and returns immediately.
// The context drives shutdown: when it is canceled, the transport is closed.
// Returns an error if ctx is nil.
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

// Close closes the WebSocket and the Input/Output channels.
// Idempotent; safe to call from any goroutine. After Close, sending on Output may panic.
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
		f, err := t.serializer.Deserialize(data)
		if err != nil {
			logger.Error("decode frame: %v", err)
			continue
		}
		// Skip keepalive/handshake-only messages (serializer returns nil frame).
		if f == nil {
			t.touch()
			continue
		}
		// Optional: notify serializer of StartFrame for sample rate etc.
		if setup, ok := t.serializer.(serialize.SerializerWithSetup); ok {
			if start, ok := f.(*frames.StartFrame); ok {
				setup.Setup(start)
			}
		}
		t.touch()
		select {
		case <-t.closed:
			return
		case t.inCh <- f:
		}
	}
}

func (t *ConnTransport) writeLoop() {
	defer func() { _ = t.Close() }()
	useBinaryDefault := false
	if _, ok := t.serializer.(serialize.ProtobufSerializer); ok {
		useBinaryDefault = true
	}
	for {
		select {
		case <-t.closed:
			return
		case f, ok := <-t.outCh:
			if !ok {
				return
			}
			var data []byte
			var err error
			msgType := websocket.TextMessage
			if withType, ok := t.serializer.(serialize.SerializerWithMessageType); ok {
				var binary bool
				data, binary, err = withType.SerializeWithType(f)
				if binary {
					msgType = websocket.BinaryMessage
				}
			} else {
				data, err = t.serializer.Serialize(f)
				if useBinaryDefault {
					msgType = websocket.BinaryMessage
				}
			}
			if err != nil {
				logger.Error("encode frame: %v", err)
				continue
			}
			if data == nil {
				// Serializer skipped this frame type (e.g. protobuf does not support it)
				continue
			}
			if err := t.conn.WriteMessage(msgType, data); err != nil {
				logger.Error("websocket write: %v", err)
				return
			}
			t.touch()
		}
	}
}

// DefaultSessionTimeout is the default inactivity duration before the server closes a WebSocket.
// Zero disables inactivity timeouts.
const DefaultSessionTimeout = 5 * time.Minute

// recoveryMiddleware wraps next and recovers panics, logging the error and stack then returning HTTP 500.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stack := string(buf[:n])
				logger.Error("panic recovered: %v\n%s", err, stack)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Default HTTP server timeouts for production hardening.
const (
	defaultReadHeaderTimeout = 10 * time.Second
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultShutdownTimeout   = 30 * time.Second
	defaultMaxHeaderBytes    = 1 << 20 // 1 MiB
)

// Server is an HTTP server that upgrades requests to /ws to WebSocket and creates a ConnTransport per connection.
// SessionTimeout is the idle timeout before closing a connection; zero or negative disables it.
// OnConn is called in a new goroutine for each connection. RegisterHandlers, if set, is called once with the mux before serving.
type Server struct {
	Host string
	Port int
	// SessionTimeout controls how long a connection may remain idle before
	// it is closed. If zero or negative, no inactivity timeout is enforced.
	SessionTimeout time.Duration
	// OnConn is called for each new connection; it receives the transport which should be linked to a pipeline.
	OnConn func(ctx context.Context, tr *ConnTransport)
	// RegisterHandlers, if non-nil, is called with the HTTP mux before the server
	// starts to allow registration of additional HTTP handlers (e.g. WebRTC signaling).
	RegisterHandlers func(mux *http.ServeMux)
	// ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout, MaxHeaderBytes, ShutdownTimeout set HTTP server timeouts.
	// Zero values use package defaults (10s, 30s, 30s, 60s, 1MiB, 30s).
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
	// TLS: when both are non-empty, ListenAndServe uses ListenAndServeTLS(certFile, keyFile).
	TLSCertFile string
	TLSKeyFile  string
	// CheckAuth if non-nil is called before upgrading to WebSocket. If it returns false, the handler returns (caller should have written 401).
	CheckAuth func(http.ResponseWriter, *http.Request) bool
	// GetSerializer if non-nil is called per request to choose the frame serializer (e.g. RTVI when query has rtvi=1).
	GetSerializer func(r *http.Request) serialize.Serializer
	// TryAcquireSlot if non-nil is called before upgrading to WebSocket. If it returns false, the handler responds 503 and returns.
	// Used to enforce max concurrent sessions.
	TryAcquireSlot func() bool
	// ReleaseSlot if non-nil is called when a connection ends (after tr.Done() is closed). Call from a goroutine that waits on tr.Done().
	ReleaseSlot func()
}

// ListenAndServe starts the HTTP server and blocks until ctx is canceled.
// It registers /ws and, if set, RegisterHandlers. Port 0 is treated as 8080.
func (s *Server) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if s.CheckAuth != nil && !s.CheckAuth(w, r) {
			return
		}
		if s.TryAcquireSlot != nil && !s.TryAcquireSlot() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"server at capacity"}`))
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if s.ReleaseSlot != nil {
				s.ReleaseSlot()
			}
			logger.Error("upgrade: %v", err)
			return
		}
		var ser serialize.Serializer = serialize.JSONSerializer{}
		if s.GetSerializer != nil {
			if custom := s.GetSerializer(r); custom != nil {
				ser = custom
			}
		}
		tr := NewConnTransport(conn, 64, 64, ser)
		if s.ReleaseSlot != nil {
			go func() {
				<-tr.Done()
				s.ReleaseSlot()
			}()
		}
		// Start monitoring this connection for inactivity if a session timeout
		// has been configured.
		if s.SessionTimeout > 0 {
			go s.monitorSession(ctx, tr, s.SessionTimeout)
		}
		if s.OnConn != nil {
			go s.OnConn(ctx, tr)
		}
	})
	if s.RegisterHandlers != nil {
		s.RegisterHandlers(mux)
	}
	handler := recoveryMiddleware(mux)

	port := s.Port
	if port == 0 {
		port = 8080
	}
	addr := fmt.Sprintf(":%d", port)
	if s.Host != "" {
		host := s.Host
		// Bind to IPv4 loopback when host is "localhost" so browsers connecting to
		// http://localhost (often resolved to 127.0.0.1) can reach the server on Windows.
		if host == "localhost" {
			host = "127.0.0.1"
		}
		addr = fmt.Sprintf("%s:%d", host, port)
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
		MaxHeaderBytes:    defaultMaxHeaderBytes,
	}
	if s.ReadHeaderTimeout > 0 {
		srv.ReadHeaderTimeout = s.ReadHeaderTimeout
	}
	if s.ReadTimeout > 0 {
		srv.ReadTimeout = s.ReadTimeout
	}
	if s.WriteTimeout > 0 {
		srv.WriteTimeout = s.WriteTimeout
	}
	if s.IdleTimeout > 0 {
		srv.IdleTimeout = s.IdleTimeout
	}
	if s.MaxHeaderBytes > 0 {
		srv.MaxHeaderBytes = s.MaxHeaderBytes
	}
	shutdownTimeout := defaultShutdownTimeout
	if s.ShutdownTimeout > 0 {
		shutdownTimeout = s.ShutdownTimeout
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("websocket server shutdown: %v", err)
		}
	}()
	if s.TLSCertFile != "" && s.TLSKeyFile != "" {
		if err := srv.ListenAndServeTLS(s.TLSCertFile, s.TLSKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("websocket server listen tls: %w", err)
		}
	} else {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("websocket server listen: %w", err)
		}
	}
	return nil
}

// ServeWithListener runs the same handler logic as ListenAndServe but on the given listener.
// The listener is not closed when the server shuts down. Used for tests with dynamic ports.
func (s *Server) ServeWithListener(ctx context.Context, listener net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if s.CheckAuth != nil && !s.CheckAuth(w, r) {
			return
		}
		if s.TryAcquireSlot != nil && !s.TryAcquireSlot() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"server at capacity"}`))
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if s.ReleaseSlot != nil {
				s.ReleaseSlot()
			}
			logger.Error("upgrade: %v", err)
			return
		}
		var ser serialize.Serializer = serialize.JSONSerializer{}
		if s.GetSerializer != nil {
			if custom := s.GetSerializer(r); custom != nil {
				ser = custom
			}
		}
		tr := NewConnTransport(conn, 64, 64, ser)
		if s.ReleaseSlot != nil {
			go func() {
				<-tr.Done()
				s.ReleaseSlot()
			}()
		}
		if s.SessionTimeout > 0 {
			go s.monitorSession(ctx, tr, s.SessionTimeout)
		}
		if s.OnConn != nil {
			go s.OnConn(ctx, tr)
		}
	})
	if s.RegisterHandlers != nil {
		s.RegisterHandlers(mux)
	}
	handler := recoveryMiddleware(mux)

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
		MaxHeaderBytes:    defaultMaxHeaderBytes,
	}
	if s.ReadHeaderTimeout > 0 {
		srv.ReadHeaderTimeout = s.ReadHeaderTimeout
	}
	if s.ReadTimeout > 0 {
		srv.ReadTimeout = s.ReadTimeout
	}
	if s.WriteTimeout > 0 {
		srv.WriteTimeout = s.WriteTimeout
	}
	if s.IdleTimeout > 0 {
		srv.IdleTimeout = s.IdleTimeout
	}
	if s.MaxHeaderBytes > 0 {
		srv.MaxHeaderBytes = s.MaxHeaderBytes
	}
	shutdownTimeout := defaultShutdownTimeout
	if s.ShutdownTimeout > 0 {
		shutdownTimeout = s.ShutdownTimeout
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("websocket server serve: %w", err)
	}
	return nil
}

// monitorSession periodically checks the last-activity time of the given
// connection transport and closes it if it has been idle for at least
// timeout. It exits when the context is canceled or the transport is closed.
func (s *Server) monitorSession(ctx context.Context, tr *ConnTransport, timeout time.Duration) {
	// Poll at half the timeout interval to balance responsiveness and overhead.
	interval := timeout / 2
	if interval <= 0 {
		interval = timeout
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tr.Done():
			return
		case <-ticker.C:
			last := tr.LastActivity()
			if last.IsZero() {
				continue
			}
			if time.Since(last) >= timeout {
				logger.Info("websocket session timeout after %s; closing connection", timeout)
				_ = tr.Close()
				return
			}
		}
	}
}
