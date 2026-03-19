// Package server provides transport servers (e.g. WebSocket, SmallWebRTC) for Voxray.
package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swaggo/http-swagger"
	_ "voxray-go/docs" // register generated Swagger spec
	"voxray-go/pkg/api"
	"voxray-go/pkg/config"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/metrics"
	"voxray-go/pkg/runner"
	"voxray-go/pkg/transport"
	"voxray-go/pkg/runner/daily"
	"voxray-go/pkg/transport/smallwebrtc"
	ws "voxray-go/pkg/transport/websocket"
	"voxray-go/pkg/frames/serialize"
	rtvi "voxray-go/pkg/processors/frameworks/rtvi"
	"voxray-go/pkg/transcripts"
)

// webrtcOfferResponse is the JSON body for successful POST /webrtc/offer responses.
type webrtcOfferResponse struct {
	Answer string `json:"answer"`
}

// SessionMeta holds metadata for a registered session.
type SessionMeta struct {
	SessionID string
	CreatedAt time.Time
}

// sessionEntry holds session metadata and cancel for the registry.
type sessionEntry struct {
	meta   SessionMeta
	cancel func()
}

// SessionRegistry is an in-memory registry of active sessions with optional cancel.
// THREAD SAFETY: mu protects sess; Register/Unregister are single-writer per key; Get (via internal lookup) may run concurrently with other readers.
type SessionRegistry struct {
	mu   sync.RWMutex
	sess map[string]sessionEntry
}

// NewSessionRegistry returns a new in-memory session registry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sess: make(map[string]sessionEntry)}
}

// Register adds a session to the registry.
func (r *SessionRegistry) Register(sessionID string, meta SessionMeta, cancel func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sess[sessionID] = sessionEntry{meta: meta, cancel: cancel}
}

// Unregister removes a session from the registry.
func (r *SessionRegistry) Unregister(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sess, sessionID)
}

// PipelineStore stores pipeline (or runner) references by session ID for API access.
// THREAD SAFETY: mu protects m; Put/Delete are writers; Get is reader; safe for concurrent use from multiple goroutines.
type PipelineStore struct {
	mu sync.RWMutex
	m  map[string]interface{}
}

// NewPipelineStore returns a new in-memory pipeline store.
func NewPipelineStore() *PipelineStore {
	return &PipelineStore{m: make(map[string]interface{})}
}

// Put stores a value for the session ID.
func (s *PipelineStore) Put(sessionID string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[sessionID] = value
}

// Get returns the value for the session ID, and true if found.
func (s *PipelineStore) Get(sessionID string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[sessionID]
	return v, ok
}

// Delete removes the session ID from the store.
func (s *PipelineStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, sessionID)
}

// idempotencyEntry caches a POST /start response for Idempotency-Key.
type idempotencyEntry struct {
	body []byte
	exp  time.Time
}

// IdempotencyStore caches responses by idempotency key (e.g. for POST /start). Safe for concurrent use.
type IdempotencyStore struct {
	mu   sync.RWMutex
	m    map[string]idempotencyEntry
	ttl  time.Duration
}

// NewIdempotencyStore returns a store with the given TTL (e.g. 24*time.Hour). Entries are not proactively cleaned; they are ignored once expired.
func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &IdempotencyStore{m: make(map[string]idempotencyEntry), ttl: ttl}
}

func (s *IdempotencyStore) get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.m[key]
	if !ok || time.Now().After(e.exp) {
		return nil, false
	}
	return e.body, true
}

func (s *IdempotencyStore) set(key string, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = idempotencyEntry{body: body, exp: time.Now().Add(s.ttl)}
}

// WebrtcOfferDoc documents the WebRTC offer HTTP endpoint for Swagger.
// POST /webrtc/offer accepts a JSON body with an "offer" (SDP offer string), creates a new WebRTC transport, and returns an SDP answer.
// The endpoint returns 400 for invalid or missing offer, 405 for non-POST, and 503 when the server cannot complete the connection (e.g. Opus encoder unavailable).
// Each request gets its own transport; onTransport is invoked in a new goroutine.
//
// @Summary Submit WebRTC offer
// @Description Accepts a WebRTC SDP offer and returns an SDP answer. Available when transport is smallwebrtc or both.
// @Tags webrtc
// @Accept json
// @Produce json
// @Param body body object true "JSON body with 'offer' (SDP offer string)"
// @Success 200 {object} webrtcOfferResponse
// @Failure 400 {string} string "Invalid offer or handling failed"
// @Failure 405 {string} string "Method not allowed"
// @Router /webrtc/offer [post]
// Versioned path: /api/v1/webrtc/offer (BasePath /api/v1).
func WebrtcOfferDoc() {}

// setCORS sets CORS headers. When allowed is nil (not in config), sets Allow-Origin to * for backward compatibility.
// For production, set cors_allowed_origins in config to an explicit list; when non-empty, only matching origins are reflected.
func setCORS(w http.ResponseWriter, r *http.Request, allowed []string, methods string) {
	if allowed == nil {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if len(allowed) > 0 {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		for _, o := range allowed {
			if o == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", methods)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
}

// routeName is a low-cardinality identifier used for HTTP metrics.
func routeName(path, fallback string) string {
	switch {
	case path == "/health" || path == "/api/v1/health":
		return "health"
	case path == "/ready" || path == "/api/v1/ready":
		return "ready"
	case path == "/webrtc/offer" || path == "/api/v1/webrtc/offer":
		return "webrtc_offer"
	case path == "/start" || path == "/api/v1/start":
		return "start"
	case strings.HasPrefix(path, "/sessions/") && strings.HasSuffix(path, "/api/offer"):
		return "session_offer"
	case strings.HasPrefix(path, "/api/v1/sessions/") && strings.Contains(path, "/offer"):
		return "session_offer"
	case path == "/telephony/ws":
		return "telephony_ws"
	case path == "/daily-dialin-webhook":
		return "daily_dialin_webhook"
	case path == "/" || path == "/api/v1":
		return "root"
	default:
		if fallback != "" {
			return fallback
		}
		return "other"
	}
}

// statusRecorder wraps ResponseWriter to capture the status code for metrics.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// metricsMiddleware wraps an http.Handler to record HTTP-level Prometheus metrics.
func metricsMiddleware(next http.Handler, route string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		routeLabel := routeName(r.URL.Path, route)
		method := r.Method

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		wrapped := http.ResponseWriter(rec)

		metrics.HTTPActiveConnections.WithLabelValues(
			routeLabel, "http", "ingress", "", "",
		).Inc()
		defer metrics.HTTPActiveConnections.WithLabelValues(
			routeLabel, "http", "ingress", "", "",
		).Dec()

		next.ServeHTTP(wrapped, r)

		statusCode := rec.status
		duration := time.Since(start).Seconds()
		statusClass := "success"
		errorType := ""
		if statusCode >= 500 {
			statusClass = "error"
			errorType = "5xx"
		} else if statusCode >= 400 {
			statusClass = "error"
			errorType = "4xx"
		}

		codeStr := http.StatusText(statusCode)
		if codeStr == "" {
			codeStr = "unknown"
		}

		metrics.HTTPRequestsTotal.WithLabelValues(
			method, routeLabel, codeStr, "", "http", "ingress", statusClass, "",
		).Inc()
		metrics.HTTPRequestDurationSeconds.WithLabelValues(
			method, routeLabel, codeStr, "", "http", "ingress", statusClass, "",
		).Observe(duration)

		if errorType != "" {
			metrics.HTTPErrorsTotal.WithLabelValues(method, routeLabel, errorType).Inc()
		}
	})
}

// wrapWithMetrics conditionally wraps h with metricsMiddleware based on cfg.MetricsEnabledOrDefault.
// When metrics are disabled, it returns h unchanged.
func wrapWithMetrics(cfg *config.Config, route string, h http.Handler) http.Handler {
	if cfg == nil || cfg.MetricsEnabledOrDefault() {
		return metricsMiddleware(h, route)
	}
	return h
}

// defaultMaxRequestBodyBytes is used when config MaxRequestBodyBytes is zero (production safety).
const defaultMaxRequestBodyBytes = 256 * 1024 // 256KB

// effectiveMaxBodyBytes returns the configured limit or a safe default.
func effectiveMaxBodyBytes(cfg *config.Config) int64 {
	if cfg != nil && cfg.MaxRequestBodyBytes > 0 {
		return cfg.MaxRequestBodyBytes
	}
	return defaultMaxRequestBodyBytes
}

// bodyReader returns r.Body, optionally wrapped with MaxBytesReader when maxBytes > 0.
func bodyReader(r *http.Request, w http.ResponseWriter, maxBytes int64) io.Reader {
	if maxBytes <= 0 {
		return r.Body
	}
	return http.MaxBytesReader(w, r.Body, maxBytes)
}

// getWebSocketSerializer returns the serializer for /ws based on query params: rtvi=1 → RTVI, format=protobuf → binary protobuf (wire-compatible), else nil (JSON).
func getWebSocketSerializer(r *http.Request) serialize.Serializer {
	if r == nil || r.URL == nil {
		return nil
	}
	q := r.URL.Query()
	if q.Get("rtvi") != "" {
		return &rtvi.Serializer{}
	}
	if strings.EqualFold(strings.TrimSpace(q.Get("format")), "protobuf") {
		return serialize.ProtobufSerializer{}
	}
	return nil
}

// requireAPIKey returns true if no ServerAPIKey is set or the request presents a valid key via Authorization: Bearer <key> or X-API-Key: <key>. Otherwise writes 401 with standard error envelope and returns false.
func requireAPIKey(cfg *config.Config, w http.ResponseWriter, r *http.Request) bool {
	if cfg == nil || cfg.ServerAPIKey == "" {
		return true
	}
	key := r.Header.Get("X-API-Key")
	if key == "" {
		if auth := r.Header.Get("Authorization"); len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
			key = strings.TrimSpace(auth[7:])
		}
	}
	if key != cfg.ServerAPIKey {
		api.RespondError(w, r, &api.APIError{StatusCode: http.StatusUnauthorized, Code: api.CodeUnauthorized, Message: "Unauthorized"})
		return false
	}
	return true
}

// registerHandlers registers the web file server (when web/ exists), Swagger, runner /start and /sessions when WebRTC is enabled, and the WebRTC /webrtc/offer handler on mux.
// sessionRegistry, pipelineStore, and transcriptFetcher may be nil; they are passed through for future API routes.
func registerHandlers(mux *http.ServeMux, cfg *config.Config, ctx context.Context, onTransport func(context.Context, transport.Transport), sessionStore runner.SessionStore, sessionRegistry *SessionRegistry, pipelineStore *PipelineStore, transcriptFetcher transcripts.Fetcher) {
	idempotencyStore := NewIdempotencyStore(24 * time.Hour)
	rtcMaxDuration := time.Duration(0)
	if cfg != nil && cfg.RTCMaxDurationSecs > 0 {
		rtcMaxDuration = time.Duration(cfg.RTCMaxDurationSecs * float64(time.Second))
	}

	// Health (liveness): GET only, 200 with envelope. Served at legacy and /api/v1.
	handleHealth := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
			return
		}
		api.RespondJSON(w, r, http.StatusOK, map[string]string{"status": "ok"}, nil)
	})
	mux.Handle("/health", wrapWithMetrics(cfg, "health", handleHealth))
	mux.Handle("/api/v1/health", wrapWithMetrics(cfg, "health", handleHealth))
	// Metrics (Prometheus text format) exposed from shared registry.
	// Keep endpoint present even when disabled so scrape configs don't break;
	// it will simply export an empty/zeroed registry in that case.
	if cfg == nil || cfg.MetricsEnabledOrDefault() {
		mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
	} else {
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
	// Readiness: GET only, 200 when ready; 503 when Redis unreachable. Served at legacy and /api/v1.
	handleReady := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
			return
		}
		if sessionStore != nil {
			if redisStore, ok := sessionStore.(*runner.RedisSessionStore); ok {
				if err := redisStore.Ping(r.Context()); err != nil {
					logger.Error("ready: redis ping: %v", err)
					api.RespondError(w, r, &api.APIError{StatusCode: http.StatusServiceUnavailable, Code: api.CodeServiceUnavailable, Message: "Service unavailable"})
					return
				}
			}
		}
		api.RespondJSON(w, r, http.StatusOK, map[string]string{"status": "ok"}, nil)
	})
	mux.Handle("/ready", wrapWithMetrics(cfg, "ready", handleReady))
	mux.Handle("/api/v1/ready", wrapWithMetrics(cfg, "ready", handleReady))
	// Swagger
	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
	))
	mode := cfg.Transport
	if mode == "" {
		mode = "websocket"
	}
	enableWebRTC := mode == "smallwebrtc" || mode == "both"
	runnerMode := enableWebRTC || cfg.RunnerTransport == "daily"
	if runnerMode {
		if sessionStore == nil {
			sessionStore = runner.NewMemorySessionStore()
		}
		registerRunnerWebRTCRoutes(mux, cfg, ctx, onTransport, sessionStore, idempotencyStore)
	}
	if enableWebRTC {
		// BREAKING CHANGE: Response is now envelope { data: { answer } }; 422 for validation; 503/500 with standard error envelope.
		handleWebRTCOffer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setCORS(w, r, cfg.CORSAllowedOrigins, "POST, OPTIONS")
			w.Header().Set("Cache-Control", "no-store")
			if r.Method == http.MethodOptions {
				api.RespondNoContent(w)
				return
			}
			if !requireAPIKey(cfg, w, r) {
				return
			}
			if r.Method != http.MethodPost {
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
				return
			}
			defer r.Body.Close()

			var req struct {
				Offer string `json:"offer"`
			}
			body := bodyReader(r, w, effectiveMaxBodyBytes(cfg))
			if err := json.NewDecoder(body).Decode(&req); err != nil {
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusBadRequest, Code: api.CodeBadRequest, Message: "Invalid JSON"})
				return
			}
			if strings.TrimSpace(req.Offer) == "" {
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusUnprocessableEntity, Code: api.CodeValidationError, Message: "Missing or empty offer", Details: []api.ErrorDetail{{Field: "offer", Message: "required"}}})
				return
			}

			connCtx, cancelConn := context.WithCancel(ctx)
			tr := smallwebrtc.NewTransport(&smallwebrtc.Config{
				ICEServers: cfg.WebRTCICEServers,
				MaxDuration: rtcMaxDuration,
				OnMaxDurationTimeout: cancelConn,
				OnClosed: cancelConn,
			})
			answer, err := tr.HandleOffer(req.Offer)
			if err != nil {
				cancelConn()
				logger.Error("smallwebrtc handle offer: %v", err)
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusServiceUnavailable, Code: api.CodeServiceUnavailable, Message: "Service unavailable"})
				return
			}
			if err := tr.Start(connCtx); err != nil {
				cancelConn()
				logger.Error("smallwebrtc start: %v", err)
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusInternalServerError, Code: api.CodeInternalError, Message: "Internal server error"})
				return
			}

			if onTransport != nil {
				go onTransport(connCtx, tr)
			}

			api.RespondJSON(w, r, http.StatusOK, map[string]string{"answer": answer}, nil)
		})
		mux.Handle("/webrtc/offer", wrapWithMetrics(cfg, "webrtc_offer", handleWebRTCOffer))
		mux.Handle("/api/v1/webrtc/offer", wrapWithMetrics(cfg, "webrtc_offer", handleWebRTCOffer))
	}
	// Telephony routes (runner: twilio, telnyx, plivo, exotel)
	registerTelephonyRoutes(mux, cfg, ctx, onTransport)

	// Daily routes (runner: daily)
	registerDailyRoutes(mux, cfg, ctx, onTransport)

	// Web root last so it doesn't override /start or /sessions (skip when telephony or daily uses /)
	telephonyMode := cfg.RunnerTransport == "twilio" || cfg.RunnerTransport == "telnyx" || cfg.RunnerTransport == "plivo" || cfg.RunnerTransport == "exotel"
	dailyMode := cfg.RunnerTransport == "daily"
	if !telephonyMode && !dailyMode {
		if st, err := os.Stat("web"); err == nil && st.IsDir() {
			mux.Handle("/", http.FileServer(http.Dir("web")))
		} else {
			// No other root handler: GET / returns 200 with envelope.
			mux.Handle("/", wrapWithMetrics(cfg, "root", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" {
					api.RespondError(w, r, &api.APIError{StatusCode: http.StatusNotFound, Code: api.CodeNotFound, Message: "Not found"})
					return
				}
				api.RespondJSON(w, r, http.StatusOK, map[string]string{"status": "ok"}, nil)
			})))
		}
	}
}

// registerTelephonyRoutes adds POST / (XML webhook) and /telephony/ws when RunnerTransport is a telephony provider.
func registerTelephonyRoutes(mux *http.ServeMux, cfg *config.Config, ctx context.Context, onTransport func(context.Context, transport.Transport)) {
	t := cfg.RunnerTransport
	if t != "twilio" && t != "telnyx" && t != "plivo" && t != "exotel" {
		return
	}
	// POST / returns provider-specific XML that points Stream/Connect to wss://{host}/telephony/ws
	mux.Handle("/", wrapWithMetrics(cfg, "telephony_root", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusNotFound, Code: api.CodeNotFound, Message: "Not found"})
			return
		}
		if r.Method == http.MethodPost {
			host := cfg.ProxyHost
			if host == "" {
				host = r.Host
			}
			scheme := "wss"
			if r.TLS == nil {
				scheme = "ws"
			}
			wsURL := scheme + "://" + host + "/telephony/ws"
			switch t {
			case "twilio", "telnyx", "plivo":
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Response><Connect><Stream url="` + wsURL + `"/></Connect></Response>`))
			case "exotel":
				api.RespondJSON(w, r, http.StatusOK, map[string]interface{}{
					"error":         "Exotel doesn't use POST webhooks",
					"websocketUrl":  wsURL,
					"note":          "Configure the WebSocket URL above in your Exotel App Bazaar Voicebot Applet",
				}, nil)
			}
			return
		}
		if r.Method == http.MethodGet {
			api.RespondJSON(w, r, http.StatusOK, map[string]string{"status": "Bot started with " + t}, nil)
			return
		}
		api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
	})))
	// /telephony/ws: WebSocket with provider detection from first message(s)
	mux.Handle("/telephony/ws", wrapWithMetrics(cfg, "telephony_ws", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
			return
		}
		conn, err := ws.Upgrade(w, r)
		if err != nil {
			logger.Error("telephony ws upgrade: %v", err)
			return
		}
		defer conn.Close()
		var first, second []byte
		_, first, _ = conn.ReadMessage()
		_, second, _ = conn.ReadMessage()
		data, ok := runner.ParseTelephonyMessage(first, second)
		if !ok || data.Provider == "" {
			logger.Error("telephony: could not detect provider from handshake")
			return
		}
		getKey := func(service, envVar string) string { return cfg.GetAPIKey(service, envVar) }
		ser := runner.BuildTelephonySerializer(data, getKey)
		if ser == nil {
			logger.Error("telephony: no serializer for provider %s", data.Provider)
			return
		}
		tr := ws.NewConnTransport(conn, 64, 64, ser, nil)
		if err := tr.Start(ctx); err != nil {
			logger.Error("telephony transport start: %v", err)
			return
		}
		if onTransport != nil {
			go onTransport(ctx, tr)
		}
		<-tr.Done()
	})))
}

// registerDailyRoutes adds GET / (create room + redirect) and optionally POST /daily-dialin-webhook when RunnerTransport is "daily".
func registerDailyRoutes(mux *http.ServeMux, cfg *config.Config, ctx context.Context, onTransport func(context.Context, transport.Transport)) {
	if cfg.RunnerTransport != "daily" {
		return
	}
	apiKey := cfg.GetAPIKey("daily_api_key", "DAILY_API_KEY")
	getOpts := func() daily.Options {
		return daily.Options{
			APIKey:               apiKey,
			RoomExpDurationHrs:   2,
			TokenExpDurationHrs:  2,
		}
	}

	mux.Handle("/", wrapWithMetrics(cfg, "daily_root", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusNotFound, Code: api.CodeNotFound, Message: "Not found"})
			return
		}
		if r.Method == http.MethodGet {
			c, err := daily.Configure(ctx, getOpts())
			if err != nil {
				logger.Error("daily configure: %v", err)
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusInternalServerError, Code: api.CodeInternalError, Message: "Internal server error"})
				return
			}
			http.Redirect(w, r, c.RoomURL, http.StatusFound)
			return
		}
		api.RespondError(w, r, &api.APIError{StatusCode: http.StatusNotFound, Code: api.CodeNotFound, Message: "Not found"})
	})))

	if cfg.Dialin {
		mux.Handle("/daily-dialin-webhook", wrapWithMetrics(cfg, "daily_dialin_webhook", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
				return
			}
			if cfg.DailyDialinWebhookSecret != "" && r.Header.Get("X-Webhook-Secret") != cfg.DailyDialinWebhookSecret {
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusUnauthorized, Code: api.CodeUnauthorized, Message: "Unauthorized"})
				return
			}
			defer r.Body.Close()
			var payload struct {
				Test       bool   `json:"test"`
				From       string `json:"From"`
				To         string `json:"To"`
				CallID     string `json:"callId"`
				CallDomain string `json:"callDomain"`
			}
			body := bodyReader(r, w, effectiveMaxBodyBytes(cfg))
			if err := json.NewDecoder(body).Decode(&payload); err != nil {
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusBadRequest, Code: api.CodeBadRequest, Message: "Invalid JSON payload"})
				return
			}
			if payload.Test {
				api.RespondJSON(w, r, http.StatusOK, map[string]string{"status": "OK"}, nil)
				return
			}
			var details []api.ErrorDetail
			if payload.From == "" {
				details = append(details, api.ErrorDetail{Field: "From", Message: "required"})
			}
			if payload.To == "" {
				details = append(details, api.ErrorDetail{Field: "To", Message: "required"})
			}
			if payload.CallID == "" {
				details = append(details, api.ErrorDetail{Field: "callId", Message: "required"})
			}
			if payload.CallDomain == "" {
				details = append(details, api.ErrorDetail{Field: "callDomain", Message: "required"})
			}
			if len(details) > 0 {
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusUnprocessableEntity, Code: api.CodeValidationError, Message: "Missing required fields", Details: details})
				return
			}
			opts := getOpts()
			opts.SIPCallerPhone = payload.From
			c, err := daily.Configure(ctx, opts)
			if err != nil {
				logger.Error("daily dial-in configure: %v", err)
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusInternalServerError, Code: api.CodeInternalError, Message: "Internal server error"})
				return
			}
			api.RespondJSON(w, r, http.StatusCreated, map[string]interface{}{
				"dailyRoom":  c.RoomURL,
				"dailyToken": c.Token,
				"sessionId":  uuid.New().String(),
			}, nil)
		})))
	}
}

// registerRunnerWebRTCRoutes adds POST /start and POST/PATCH /sessions/{id}/api/offer (legacy) and /api/v1/sessions/{id}/offer (versioned).
func registerRunnerWebRTCRoutes(mux *http.ServeMux, cfg *config.Config, ctx context.Context, onTransport func(context.Context, transport.Transport), store runner.SessionStore, idempotencyStore *IdempotencyStore) {
	rtcMaxDuration := time.Duration(0)
	if cfg != nil && cfg.RTCMaxDurationSecs > 0 {
		rtcMaxDuration = time.Duration(cfg.RTCMaxDurationSecs * float64(time.Second))
	}

	// BREAKING CHANGE: POST /start now returns 201 Created with envelope { data: { sessionId, iceConfig?, ... } }; supports Idempotency-Key.
	// POST /start or /api/v1/start: create session.
	handleStart := func(w http.ResponseWriter, r *http.Request) {
		setCORS(w, r, cfg.CORSAllowedOrigins, "POST, OPTIONS")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodOptions {
			api.RespondNoContent(w)
			return
		}
		if !requireAPIKey(cfg, w, r) {
			return
		}
		if r.Method != http.MethodPost {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
			return
		}
		defer r.Body.Close()

		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key != "" {
			if cached, ok := idempotencyStore.get(key); ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(cached)
				return
			}
		}

		var req struct {
			CreateDailyRoom         bool                   `json:"createDailyRoom"`
			EnableDefaultIceServers bool                   `json:"enableDefaultIceServers"`
			Body                    map[string]interface{} `json:"body"`
		}
		body := bodyReader(r, w, effectiveMaxBodyBytes(cfg))
		if err := json.NewDecoder(body).Decode(&req); err != nil {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusBadRequest, Code: api.CodeBadRequest, Message: "Invalid JSON body"})
			return
		}

		var data map[string]interface{}
		if req.CreateDailyRoom {
			opts := daily.Options{
				APIKey:               cfg.GetAPIKey("daily_api_key", "DAILY_API_KEY"),
				RoomExpDurationHrs:   2,
				TokenExpDurationHrs:  2,
			}
			c, err := daily.Configure(ctx, opts)
			if err != nil {
				logger.Error("daily configure: %v", err)
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusInternalServerError, Code: api.CodeInternalError, Message: "Internal server error"})
				return
			}
			data = map[string]interface{}{
				"dailyRoom":  c.RoomURL,
				"dailyToken": c.Token,
				"sessionId":  uuid.New().String(),
			}
		} else {
			sessionID := uuid.New().String()
			if err := store.Put(sessionID, &runner.Session{
				Body:                    req.Body,
				EnableDefaultIceServers: req.EnableDefaultIceServers,
			}); err != nil {
				logger.Error("session store put: %v", err)
				api.RespondError(w, r, &api.APIError{StatusCode: http.StatusInternalServerError, Code: api.CodeInternalError, Message: "Internal server error"})
				return
			}
			data = map[string]interface{}{"sessionId": sessionID}
			if req.EnableDefaultIceServers {
				data["iceConfig"] = map[string]interface{}{
					"iceServers": []map[string]interface{}{
						{"urls": []string{"stun:stun.l.google.com:19302"}},
					},
				}
			}
		}

		respBytes, _ := json.Marshal(api.SuccessEnvelope{Data: data, Meta: nil})
		if key != "" {
			idempotencyStore.set(key, respBytes)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(respBytes)
	}
	mux.HandleFunc("/start", handleStart)
	mux.HandleFunc("/api/v1/start", handleStart)

	// BREAKING CHANGE: Session offer path is now /api/v1/sessions/:id/offer (legacy /sessions/:id/api/offer still supported). PATCH returns 204 No Content; POST returns 200 with envelope { data: { answer, type } }.
	handleSessionOffer := func(w http.ResponseWriter, r *http.Request) {
		if !requireAPIKey(cfg, w, r) {
			return
		}
		w.Header().Set("Cache-Control", "no-store")

		path := r.URL.Path
		var sessionID string
		var isOfferPath bool
		if strings.HasPrefix(path, "/api/v1/sessions/") {
			rest := path[len("/api/v1/sessions/"):]
			if i := strings.Index(rest, "/"); i >= 0 {
				sessionID = rest[:i]
				isOfferPath = rest[i:] == "/offer"
			} else {
				sessionID = rest
			}
		} else if strings.HasPrefix(path, "/sessions/") {
			rest := path[len("/sessions/"):]
			if i := strings.Index(rest, "/"); i >= 0 {
				sessionID = rest[:i]
				isOfferPath = rest[i:] == "/api/offer"
			} else {
				sessionID = rest
			}
		}
		if sessionID == "" || !isOfferPath {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusNotFound, Code: api.CodeNotFound, Message: "Not found"})
			return
		}
		if _, err := uuid.Parse(sessionID); err != nil {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusBadRequest, Code: api.CodeValidationError, Message: "Invalid session ID format", Details: []api.ErrorDetail{{Field: "sessionId", Message: "must be a valid UUID"}}})
			return
		}
		sess, err := store.Get(sessionID)
		if err != nil {
			logger.Error("session store get: %v", err)
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusInternalServerError, Code: api.CodeInternalError, Message: "Internal server error"})
			return
		}
		if sess == nil {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusNotFound, Code: api.CodeNotFound, Message: "Session not found"})
			return
		}

		setCORS(w, r, cfg.CORSAllowedOrigins, "POST, PATCH, OPTIONS")
		if r.Method == http.MethodOptions {
			api.RespondNoContent(w)
			return
		}
		if r.Method == http.MethodPatch {
			// ICE/trickle ack: 204 No Content (fire-and-forget).
			api.RespondNoContent(w)
			return
		}
		if r.Method != http.MethodPost {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusMethodNotAllowed, Code: api.CodeBadRequest, Message: "Method not allowed"})
			return
		}
		defer r.Body.Close()
		var offerReq struct {
			SDP            string                 `json:"sdp"`
			Type           string                 `json:"type"`
			PCID           string                 `json:"pc_id"`
			RestartPC      bool                   `json:"restart_pc"`
			RequestData    map[string]interface{} `json:"request_data"`
			RequestDataAlt map[string]interface{} `json:"requestData"`
		}
		body := bodyReader(r, w, effectiveMaxBodyBytes(cfg))
		if err := json.NewDecoder(body).Decode(&offerReq); err != nil {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusBadRequest, Code: api.CodeBadRequest, Message: "Invalid JSON body"})
			return
		}
		if strings.TrimSpace(offerReq.SDP) == "" {
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusUnprocessableEntity, Code: api.CodeValidationError, Message: "Missing or empty SDP", Details: []api.ErrorDetail{{Field: "sdp", Message: "required"}}})
			return
		}
		if offerReq.RequestData == nil {
			offerReq.RequestData = offerReq.RequestDataAlt
		}
		if offerReq.RequestData == nil {
			offerReq.RequestData = sess.Body
		}
		connCtx, cancelConn := context.WithCancel(ctx)
		tr := smallwebrtc.NewTransport(&smallwebrtc.Config{
			ICEServers: cfg.WebRTCICEServers,
			MaxDuration: rtcMaxDuration,
			OnMaxDurationTimeout: cancelConn,
			OnClosed: cancelConn,
		})
		answer, err := tr.HandleOffer(offerReq.SDP)
		if err != nil {
			cancelConn()
			logger.Error("sessions offer handle: %v", err)
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusServiceUnavailable, Code: api.CodeServiceUnavailable, Message: "Service unavailable"})
			return
		}
		if err := tr.Start(connCtx); err != nil {
			cancelConn()
			logger.Error("sessions offer start: %v", err)
			api.RespondError(w, r, &api.APIError{StatusCode: http.StatusInternalServerError, Code: api.CodeInternalError, Message: "Internal server error"})
			return
		}
		if onTransport != nil {
			go onTransport(connCtx, tr)
		}
		api.RespondJSON(w, r, http.StatusOK, map[string]string{"answer": answer, "type": "answer"}, nil)
	}
	mux.HandleFunc("/sessions/", handleSessionOffer)
	mux.HandleFunc("/api/v1/sessions/", handleSessionOffer)
}

// StartServers starts the HTTP server that serves the WebSocket endpoint at /ws and, when transport is smallwebrtc or both, the WebRTC signaling endpoint at POST /webrtc/offer.
// The onTransport callback is run in a new goroutine for each new connection.
// sessionRegistry, pipelineStore, and transcriptFetcher are optional (may be nil) and passed to handlers for future API routes.
// If cfg is nil, StartServers returns nil without starting anything.
// The server runs until ctx is canceled.
func StartServers(ctx context.Context, cfg *config.Config, onTransport func(ctx context.Context, tr transport.Transport), sessionRegistry *SessionRegistry, pipelineStore *PipelineStore, transcriptFetcher transcripts.Fetcher) error {
	if cfg == nil {
		return nil
	}

	mode := cfg.Transport
	if mode == "" {
		mode = "websocket"
	}

	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	enableWebRTC := mode == "smallwebrtc" || mode == "both"
	runnerMode := enableWebRTC || cfg.RunnerTransport == "daily"
	rtcMaxDuration := time.Duration(0)
	if cfg.RTCMaxDurationSecs > 0 {
		rtcMaxDuration = time.Duration(cfg.RTCMaxDurationSecs * float64(time.Second))
	}
	var sessionStore runner.SessionStore
	if runnerMode {
		var err error
		sessionStore, err = runner.NewSessionStoreFromConfig(cfg)
		if err != nil {
			return err
		}
	}

	server := &ws.Server{
		Host:           cfg.Host,
		Port:           port,
		SessionTimeout: ws.DefaultSessionTimeout,
		MaxDurationAfterFirstAudio: rtcMaxDuration,
		OnConn: func(c context.Context, tr *ws.ConnTransport) {
			if onTransport != nil {
				onTransport(c, tr)
			}
		},
		RegisterHandlers: func(mux *http.ServeMux) {
			registerHandlers(mux, cfg, ctx, onTransport, sessionStore, sessionRegistry, pipelineStore, transcriptFetcher)
		},
		GetSerializer:         getWebSocketSerializer,
		WriteCoalesceMs:        cfg.WSWriteCoalesceMs,
		WriteCoalesceMaxFrames: cfg.WSWriteCoalesceMaxFrames,
	}
	if cfg.ServerAPIKey != "" {
		server.CheckAuth = func(w http.ResponseWriter, r *http.Request) bool {
			return requireAPIKey(cfg, w, r)
		}
	}
	if cfg.TLSEnable && cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		server.TLSCertFile = cfg.TLSCertFile
		server.TLSKeyFile = cfg.TLSKeyFile
	}

	tlsMode := ""
	if server.TLSCertFile != "" {
		tlsMode = " tls=on"
	}
	logger.Info("starting server on %s:%d (transport=%s)%s", cfg.Host, port, mode, tlsMode)
	return server.ListenAndServe(ctx)
}

// StartServersWithListener starts the same HTTP stack as StartServers but uses the provided listener (e.g. from net.Listen("tcp", ":0")).
// The caller owns the listener and must close it when done.
// Useful for tests that need a dynamic port.
// sessionRegistry, pipelineStore, and transcriptFetcher may be nil.
// If cfg is nil, returns nil without starting.
func StartServersWithListener(ctx context.Context, listener net.Listener, cfg *config.Config, onTransport func(ctx context.Context, tr transport.Transport), sessionRegistry *SessionRegistry, pipelineStore *PipelineStore, transcriptFetcher transcripts.Fetcher) error {
	if cfg == nil {
		return nil
	}
	mode := cfg.Transport
	if mode == "" {
		mode = "websocket"
	}
	enableWebRTC := mode == "smallwebrtc" || mode == "both"
	runnerMode := enableWebRTC || cfg.RunnerTransport == "daily"
	rtcMaxDuration := time.Duration(0)
	if cfg.RTCMaxDurationSecs > 0 {
		rtcMaxDuration = time.Duration(cfg.RTCMaxDurationSecs * float64(time.Second))
	}
	var sessionStore runner.SessionStore
	if runnerMode {
		var err error
		sessionStore, err = runner.NewSessionStoreFromConfig(cfg)
		if err != nil {
			return err
		}
	}

	server := &ws.Server{
		Host:           cfg.Host,
		Port:           0,
		SessionTimeout: ws.DefaultSessionTimeout,
		MaxDurationAfterFirstAudio: rtcMaxDuration,
		OnConn: func(c context.Context, tr *ws.ConnTransport) {
			if onTransport != nil {
				onTransport(c, tr)
			}
		},
		RegisterHandlers: func(mux *http.ServeMux) {
			registerHandlers(mux, cfg, ctx, onTransport, sessionStore, sessionRegistry, pipelineStore, transcriptFetcher)
		},
		GetSerializer:         getWebSocketSerializer,
		WriteCoalesceMs:        cfg.WSWriteCoalesceMs,
		WriteCoalesceMaxFrames: cfg.WSWriteCoalesceMaxFrames,
	}
	if cfg.ServerAPIKey != "" {
		server.CheckAuth = func(w http.ResponseWriter, r *http.Request) bool {
			return requireAPIKey(cfg, w, r)
		}
	}

	return server.ServeWithListener(ctx, listener)
}
