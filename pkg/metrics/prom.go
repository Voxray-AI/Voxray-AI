package metrics

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Registry is the shared Prometheus registry for Voxray.
var Registry = prometheus.NewRegistry()

// Common label keys.
const (
	LabelSessionID = "session_id"
	LabelStage     = "stage"
	LabelDirection = "direction"
	LabelStatus    = "status"
	LabelModel     = "model"
)

// HTTP metrics.
var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "route", "status_code", LabelSessionID, LabelStage, LabelDirection, LabelStatus, LabelModel},
	)
	HTTPRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status_code", LabelSessionID, LabelStage, LabelDirection, LabelStatus, LabelModel},
	)
	HTTPActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_active_connections",
			Help: "Number of active HTTP connections.",
		},
		[]string{"route", LabelStage, LabelDirection, LabelSessionID, LabelModel},
	)
	HTTPErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP errors.",
		},
		[]string{"method", "route", "error_type"},
	)
	HTTPTimeoutTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_timeout_total",
			Help: "Total number of HTTP timeouts.",
		},
		[]string{"method", "route"},
	)
)

// WebRTC metrics.
var (
	WebRTCPeerConnectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webrtc_peer_connections_total",
			Help: "Total number of WebRTC peer connections by state.",
		},
		[]string{"state", LabelSessionID, LabelStage},
	)
	WebRTCPeerConnectionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "webrtc_peer_connections_active",
			Help: "Current number of active WebRTC peer connections.",
		},
		[]string{LabelStage, LabelSessionID},
	)
	WebRTCBytesSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webrtc_bytes_sent_total",
			Help: "Total bytes sent over WebRTC.",
		},
		[]string{LabelDirection, LabelSessionID, LabelModel},
	)
	WebRTCBytesReceivedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webrtc_bytes_received_total",
			Help: "Total bytes received over WebRTC.",
		},
		[]string{LabelDirection, LabelSessionID, LabelModel},
	)
	WebRTCConnectionFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webrtc_connection_failures_total",
			Help: "Total WebRTC connection failures by reason.",
		},
		[]string{"reason", LabelStage},
	)
	WebRTCReconnectionAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webrtc_reconnection_attempts_total",
			Help: "Total number of WebRTC reconnection attempts.",
		},
		[]string{LabelSessionID, LabelStage},
	)
)

// STT metrics.
var (
	STTErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stt_errors_total",
			Help: "Total STT errors by type.",
		},
		[]string{"error_type", LabelSessionID, LabelStage, LabelModel},
	)
	STTFallbackTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stt_fallback_total",
			Help: "Total STT fallback invocations.",
		},
		[]string{LabelSessionID, LabelStage, LabelModel},
	)
	STTTimeToFirstTokenSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "stt_time_to_first_token_seconds",
			Help:    "Time from audio start to first STT token.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelSessionID, LabelStage, LabelStatus, LabelModel},
	)
	STTTranscriptionLatencySeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "stt_transcription_latency_seconds",
			Help:    "End-to-end STT transcription latency per utterance.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelSessionID, LabelStage, LabelStatus, LabelModel},
	)
	STTStreamingLagSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "stt_streaming_lag_seconds",
			Help:    "Lag between audio arrival and STT transcript emission.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelDirection, LabelSessionID, LabelModel},
	)
)

// LLM metrics.
var (
	LLMErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_errors_total",
			Help: "Total LLM errors by type.",
		},
		[]string{"error_type", LabelSessionID, LabelStage, LabelModel},
	)
	LLMRetriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_retries_total",
			Help: "Total LLM retries.",
		},
		[]string{LabelSessionID, LabelStage, LabelModel},
	)
	LLMFallbackTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_fallback_total",
			Help: "Total LLM fallback invocations.",
		},
		[]string{LabelSessionID, LabelStage, LabelModel},
	)
	LLMTimeToFirstTokenSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_time_to_first_token_seconds",
			Help:    "Time from request to first LLM token.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelSessionID, LabelStage, LabelStatus, LabelModel},
	)
	LLMGenerationLatencySeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_generation_latency_seconds",
			Help:    "End-to-end LLM generation latency.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelSessionID, LabelStage, LabelStatus, LabelModel},
	)
	LLMInterTokenLatencySeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_inter_token_latency_seconds",
			Help:    "Latency between streamed LLM tokens.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelSessionID, LabelStage, LabelModel},
	)
)

// TTS metrics.
var (
	TTSErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tts_errors_total",
			Help: "Total TTS errors by type.",
		},
		[]string{"error_type", LabelSessionID, LabelStage, LabelModel},
	)
	TTSFallbackTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tts_fallback_total",
			Help: "Total TTS fallback invocations.",
		},
		[]string{LabelSessionID, LabelStage, LabelModel},
	)
	TTSTimeToFirstAudioChunkSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tts_time_to_first_audio_chunk_seconds",
			Help:    "Time from text-in to first TTS audio chunk.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelSessionID, LabelStage, LabelStatus, LabelModel},
	)
	TTSSynthesisLatencySeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tts_synthesis_latency_seconds",
			Help:    "Full TTS synthesis latency.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelSessionID, LabelStage, LabelStatus, LabelModel},
	)
	TTSStreamingLagSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tts_streaming_lag_seconds",
			Help:    "Lag between text-in and audio-out for TTS.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{LabelDirection, LabelSessionID, LabelModel},
	)
)

func init() {
	metrics := []prometheus.Collector{
		HTTPRequestsTotal, HTTPRequestDurationSeconds, HTTPActiveConnections, HTTPErrorsTotal, HTTPTimeoutTotal,
		WebRTCPeerConnectionsTotal, WebRTCPeerConnectionsActive, WebRTCBytesSentTotal, WebRTCBytesReceivedTotal,
		WebRTCConnectionFailuresTotal, WebRTCReconnectionAttemptsTotal,
		STTErrorsTotal, STTFallbackTotal, STTTimeToFirstTokenSeconds, STTTranscriptionLatencySeconds, STTStreamingLagSeconds,
		LLMErrorsTotal, LLMRetriesTotal, LLMFallbackTotal, LLMTimeToFirstTokenSeconds, LLMGenerationLatencySeconds, LLMInterTokenLatencySeconds,
		TTSErrorsTotal, TTSFallbackTotal, TTSTimeToFirstAudioChunkSeconds, TTSSynthesisLatencySeconds, TTSStreamingLagSeconds,
	}
	for _, m := range metrics {
		_ = Registry.Register(m)
	}
	rand.Seed(time.Now().UnixNano())
}

// SampledSessionID returns a stable, low-cardinality session ID label.
// It either hashes the raw ID to a short hex string or, when sampled out,
// returns the constant "sampled_out".
func SampledSessionID(raw string, sampleRate int) string {
	if raw == "" {
		return ""
	}
	if sampleRate <= 1 {
		return hashSessionID(raw)
	}
	if rand.Intn(sampleRate) != 0 {
		return "sampled_out"
	}
	return hashSessionID(raw)
}

func hashSessionID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8])
}

