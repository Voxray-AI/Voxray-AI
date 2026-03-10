// Package server (middleware.go): CORS, metrics, body limit, and API key helpers.
package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/metrics"
	"voxray-go/pkg/frames/serialize"
	rtvi "voxray-go/pkg/processors/frameworks/rtvi"
)

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
	case path == "/health":
		return "health"
	case path == "/ready":
		return "ready"
	case path == "/webrtc/offer":
		return "webrtc_offer"
	case path == "/start":
		return "start"
	case strings.HasPrefix(path, "/sessions/") && strings.HasSuffix(path, "/api/offer"):
		return "session_offer"
	case path == "/telephony/ws":
		return "telephony_ws"
	case path == "/daily-dialin-webhook":
		return "daily_dialin_webhook"
	case path == "/":
		return "root"
	default:
		if fallback != "" {
			return fallback
		}
		return "other"
	}
}

// metricsMiddleware wraps an http.Handler to record HTTP-level Prometheus metrics.
func metricsMiddleware(next http.Handler, route string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		routeLabel := routeName(r.URL.Path, route)
		method := r.Method

		// Wrap ResponseWriter to capture status code.
		type statusRecorder struct {
			http.ResponseWriter
			status int
		}
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
	if cfg != nil && cfg.MetricsEnabledOrDefault() {
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

// requireAPIKey returns true if no ServerAPIKey is set or the request presents a valid key via Authorization: Bearer <key> or X-API-Key: <key>. Otherwise writes 401 JSON and returns false.
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return false
	}
	return true
}
