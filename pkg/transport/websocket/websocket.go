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

	"voxray-go/pkg/api"
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
// THREAD SAFETY: single reader goroutine (readLoop) and single writer goroutine (writeLoop); only they touch conn. Close is idempotent; do not send on Output after Close.
type ConnTransport struct {
	conn       *websocket.Conn
	serializer serialize.Serializer
	inCh       chan frames.Frame
	outCh      chan frames.Frame
	closed     chan struct{}
	once       sync.Once

	// writeCoalesceMs when > 0 enables write coalescing: drain up to writeCoalesceMaxFrames frames within this many ms before writing (reduces syscalls; adds latency). Set via ConnTransportOptions.
	writeCoalesceMs     int
	writeCoalesceMaxFrames int

	// WriteMessageFunc, when non-nil, is used instead of conn.WriteMessage in writeOne (e.g. for tests to count or capture writes). When nil, conn.WriteMessage is used.
	WriteMessageFunc func(messageType int, data []byte) error

	// lastActivity holds the last time we saw activity on this connection
	// (either a successfully read frame from the client or a successfully
	// written frame to the client), stored as Unix nano time.
	lastActivity atomic.Int64

	// Max-duration enforcement (optional).
	maxDurationAfterFirstAudio time.Duration
	onMaxDurationTimeout       func()
	maxDurationOnce            sync.Once
}

// DefaultReadLimit is the maximum WebSocket message size in bytes (1MB). Prevents memory exhaustion from oversized frames.
const DefaultReadLimit = 1 << 20

// ConnTransportOptions optionally configures write coalescing when creating a ConnTransport.
// Pass nil to NewConnTransport for default behaviour (no coalescing).
type ConnTransportOptions struct {
	// WriteCoalesceMs when > 0 enables write coalescing (drain up to WriteCoalesceMaxFrames within this many ms).
	WriteCoalesceMs int
	// WriteCoalesceMaxFrames is the max frames per coalesced batch; 0 means default 10.
	WriteCoalesceMaxFrames int

	// MaxDurationAfterFirstAudio when > 0 starts a one-shot timer on the
	// first inbound *frames.AudioRawFrame and invokes OnMaxDurationTimeout
	// when the duration elapses.
	MaxDurationAfterFirstAudio time.Duration
	OnMaxDurationTimeout       func()
}

// NewConnTransport builds a transport for an already-upgraded WebSocket connection.
// If serializer is nil, JSON text messages are used. inBuf and outBuf set channel sizes; zero or negative values default to 64.
// opts may be nil; when non-nil and WriteCoalesceMs > 0, coalescing is enabled so callers need not set fields after construction.
// The caller must not use conn for reads or writes after passing it here.
func NewConnTransport(conn *websocket.Conn, inBuf, outBuf int, serializer serialize.Serializer, opts *ConnTransportOptions) *ConnTransport {
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

	if opts != nil {
		if opts.WriteCoalesceMs > 0 {
			t.writeCoalesceMs = opts.WriteCoalesceMs
			if opts.WriteCoalesceMaxFrames > 0 {
				t.writeCoalesceMaxFrames = opts.WriteCoalesceMaxFrames
			}
		}
		t.maxDurationAfterFirstAudio = opts.MaxDurationAfterFirstAudio
		t.onMaxDurationTimeout = opts.OnMaxDurationTimeout
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
	// CONCURRENCY: single reader goroutine; single writer goroutine; only they touch conn.
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

		t.maybeStartMaxDuration(f)

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

func (t *ConnTransport) maybeStartMaxDuration(f frames.Frame) {
	if t.maxDurationAfterFirstAudio <= 0 || t.onMaxDurationTimeout == nil {
		return
	}
	if _, ok := f.(*frames.AudioRawFrame); !ok {
		return
	}
	t.maxDurationOnce.Do(func() {
		dur := t.maxDurationAfterFirstAudio
		cb := t.onMaxDurationTimeout
		// One-shot timer: when it fires, close/cancel will be handled by the
		// callback, and Close() will stop future reads/writes.
		time.AfterFunc(dur, func() {
			select {
			case <-t.closed:
				return
			default:
			}
			if cb != nil {
				cb()
			}
		})
	})
}

func (t *ConnTransport) writeLoop() {
	defer func() { _ = t.Close() }()
	useBinaryDefault := false
	if _, ok := t.serializer.(serialize.ProtobufSerializer); ok {
		useBinaryDefault = true
	}
	coalesceMs := t.writeCoalesceMs
	maxFrames := t.writeCoalesceMaxFrames
	if maxFrames <= 0 {
		maxFrames = 10
	}
	for {
		// PERF: coalescing reduces syscalls at the cost of added latency budget; when coalesceMs > 0, drain up to maxFrames within the window.
		if coalesceMs > 0 {
			select {
			case <-t.closed:
				return
			case f, ok := <-t.outCh:
				if !ok {
					return
				}
				batch := []frames.Frame{f}
				timer := time.NewTimer(time.Duration(coalesceMs) * time.Millisecond)
			drainLoop:
				for len(batch) < maxFrames {
					select {
					case <-t.closed:
						timer.Stop()
						return
					case f, ok := <-t.outCh:
						if !ok {
							timer.Stop()
							break drainLoop
						}
						batch = append(batch, f)
					case <-timer.C:
						break drainLoop
					}
				}
				timer.Stop()
				for _, fr := range batch {
					if err := t.writeOne(fr, useBinaryDefault); err != nil {
						return
					}
				}
			}
			continue
		}
		select {
		case <-t.closed:
			return
		case f, ok := <-t.outCh:
			if !ok {
				return
			}
			if err := t.writeOne(f, useBinaryDefault); err != nil {
				return
			}
		}
	}
}

// writeOne serializes and writes a single frame. Returns non-nil error to stop the write loop.
func (t *ConnTransport) writeOne(f frames.Frame, useBinaryDefault bool) error {
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
		return nil
	}
	if data == nil {
		return nil
	}
	write := t.WriteMessageFunc
	if write == nil {
		write = t.conn.WriteMessage
	}
	if err := write(msgType, data); err != nil {
		logger.Error("websocket write: %v", err)
		return err
	}
	t.touch()
	return nil
}

// DefaultSessionTimeout is the default inactivity duration before the server closes a WebSocket.
// Zero disables inactivity timeouts.
const DefaultSessionTimeout = 5 * time.Minute

// recoveryMiddleware wraps next and recovers panics, logging the error and stack then returning HTTP 500 with standard error envelope.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stack := string(buf[:n])
				logger.Error("panic recovered: %v\n%s", err, stack)
				api.RespondError(w, r, &api.APIError{
					StatusCode: http.StatusInternalServerError,
					Code:       api.CodeInternalError,
					Message:    "Internal server error",
				})
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
	// WriteCoalesceMs when > 0 enables write coalescing on each ConnTransport (drain up to WriteCoalesceMaxFrames within this many ms).
	WriteCoalesceMs     int
	WriteCoalesceMaxFrames int

	// MaxDurationAfterFirstAudio when > 0 starts a one-shot max-duration
	// timer on each connection after the first inbound *frames.AudioRawFrame.
	// When the timer fires, the per-connection context is canceled, which
	// terminates the transport and associated pipeline session.
	MaxDurationAfterFirstAudio time.Duration
}

// ListenAndServe starts the HTTP server and blocks until ctx is canceled.
// It registers /ws and, if set, RegisterHandlers. Port 0 is treated as 8080.
func (s *Server) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if s.CheckAuth != nil && !s.CheckAuth(w, r) {
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("upgrade: %v", err)
			return
		}
		var ser serialize.Serializer = serialize.JSONSerializer{}
		if s.GetSerializer != nil {
			if custom := s.GetSerializer(r); custom != nil {
				ser = custom
			}
		}
		var opts *ConnTransportOptions
		// Per-connection context so timers can cancel only this connection's session.
		connCtx, cancelConn := context.WithCancel(ctx)
		if s.WriteCoalesceMs > 0 || s.MaxDurationAfterFirstAudio > 0 {
			opts = &ConnTransportOptions{
				WriteCoalesceMs:           s.WriteCoalesceMs,
				WriteCoalesceMaxFrames:   s.WriteCoalesceMaxFrames,
				MaxDurationAfterFirstAudio: s.MaxDurationAfterFirstAudio,
				OnMaxDurationTimeout:     cancelConn,
			}
		}
		tr := NewConnTransport(conn, 64, 64, ser, opts)
		go func() {
			<-tr.Done()
			cancelConn()
		}()
		// Start monitoring this connection for inactivity if a session timeout
		// has been configured.
		if s.SessionTimeout > 0 {
			go s.monitorSession(connCtx, tr, s.SessionTimeout)
		}
		if s.OnConn != nil {
			go s.OnConn(connCtx, tr)
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
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("upgrade: %v", err)
			return
		}
		var ser serialize.Serializer = serialize.JSONSerializer{}
		if s.GetSerializer != nil {
			if custom := s.GetSerializer(r); custom != nil {
				ser = custom
			}
		}
		var opts *ConnTransportOptions
		connCtx, cancelConn := context.WithCancel(ctx)
		var tr *ConnTransport
		if s.WriteCoalesceMs > 0 || s.MaxDurationAfterFirstAudio > 0 {
			opts = &ConnTransportOptions{
				WriteCoalesceMs:           s.WriteCoalesceMs,
				WriteCoalesceMaxFrames:   s.WriteCoalesceMaxFrames,
				MaxDurationAfterFirstAudio: s.MaxDurationAfterFirstAudio,
				OnMaxDurationTimeout:     cancelConn,
			}
		}
		tr = NewConnTransport(conn, 64, 64, ser, opts)
		go func() {
			<-tr.Done()
			cancelConn()
		}()
		if s.SessionTimeout > 0 {
			go s.monitorSession(connCtx, tr, s.SessionTimeout)
		}
		if s.OnConn != nil {
			go s.OnConn(connCtx, tr)
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
