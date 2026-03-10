// Package config handles the application configuration, including environment variables and JSON files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host          string                     `json:"host"`
	Port          int                        `json:"port"`
	Model         string                     `json:"model"`
	Provider      string                     `json:"provider,omitempty"` // default for all tasks; "openai" or "groq"
	SttProvider   string                     `json:"stt_provider,omitempty"`
	LlmProvider   string                     `json:"llm_provider,omitempty"`
	TtsProvider   string                     `json:"tts_provider,omitempty"`
	STTModel      string                     `json:"stt_model,omitempty"`
	STTLanguage   string                     `json:"stt_language,omitempty"` // e.g. "hi-IN", "en-IN"; empty = auto-detect (Sarvam)
	TTSModel      string                     `json:"tts_model,omitempty"`
	TTSVoice      string                     `json:"tts_voice,omitempty"`
	Plugins       []string                   `json:"plugins"`
	PluginOptions map[string]json.RawMessage `json:"plugin_options,omitempty"` // per-plugin JSON options; key = plugin name
	APIKeys       map[string]string          `json:"api_keys,omitempty"`

	// Transport selects which network transports are enabled for the server.
	// Supported values:
	//   - "" or "websocket": only WebSocket (/ws).
	//   - "smallwebrtc": only SmallWebRTC signaling (/webrtc/offer).
	//   - "both": enable both WebSocket and SmallWebRTC on the same HTTP server.
	Transport string `json:"transport,omitempty"`
	// WebRTCICEServers lists ICE server URLs (e.g. STUN/TURN) for the SmallWebRTC transport.
	// When empty, a sensible default STUN server is used.
	WebRTCICEServers []string `json:"webrtc_ice_servers,omitempty"`

	// Turn detection: when to consider user finished speaking
	TurnDetection       string  `json:"turn_detection,omitempty"`         // "none" | "silence"; default "none"
	TurnStopSecs        float64 `json:"turn_stop_secs,omitempty"`         // silence after speech to end turn (default 3)
	TurnPreSpeechMs     float64 `json:"turn_pre_speech_ms,omitempty"`     // pre-speech padding ms (default 500)
	TurnMaxDurationSecs float64 `json:"turn_max_duration_secs,omitempty"` // max segment duration secs (default 8)
	VADStartSecs        float64 `json:"vad_start_secs,omitempty"`         // VAD start trigger time for turn (default 0)
	VadThreshold        float64 `json:"vad_threshold,omitempty"`          // EnergyDetector RMS threshold (default 0.02)
	TurnAsync           bool    `json:"turn_async,omitempty"`             // use async AnalyzeEndOfTurn instead of sync AppendAudio

	// User turn / idle lifecycle.
	// When zero, UserTurnStopTimeoutSecs falls back to TurnStopSecs; when both
	// are zero, a conservative default (5s) is used.
	UserTurnStopTimeoutSecs float64 `json:"user_turn_stop_timeout_secs,omitempty"` // timeout with no activity before forcing user turn stop
	// When >0, triggers a UserIdleFrame after the bot has finished speaking
	// and the user has been idle for this duration.
	UserIdleTimeoutSecs float64 `json:"user_idle_timeout_secs,omitempty"`

	// VAD analyzer configuration. When unset, defaults
	// match the Python VADParams defaults.
	VADType         string  `json:"vad_type,omitempty"`           // "energy" (default), "silero", "aic" (future)
	VADConfidence   float64 `json:"vad_confidence,omitempty"`     // default 0.7
	VADStartSecsVAD float64 `json:"vad_start_secs_vad,omitempty"` // default 0.2
	VADStopSecs     float64 `json:"vad_stop_secs,omitempty"`      // default 0.2
	VADMinVolume    float64 `json:"vad_min_volume,omitempty"`     // default 0.6

	// Interruption: allow user to interrupt bot; strategy (e.g. "keyword") and min_words for future use.
	AllowInterruptions   bool   `json:"allow_interruptions,omitempty"`
	InterruptionStrategy string `json:"interruption_strategy,omitempty"`
	MinWords             int    `json:"min_words,omitempty"`

	// Runner: development runner (transport type, port, proxy for telephony).
	// RunnerTransport: "webrtc" | "daily" | "twilio" | "telnyx" | "plivo" | "exotel" | "livekit" | "" (use Transport + /ws as before).
	RunnerTransport string `json:"runner_transport,omitempty"`
	// RunnerPort overrides Port when runner is used (default 8080; Python runner uses 7860).
	RunnerPort int `json:"runner_port,omitempty"`
	// ProxyHost is the public hostname for telephony webhook XML (e.g. mybot.ngrok.io). No protocol.
	ProxyHost string `json:"proxy_host,omitempty"`
	// Dialin enables Daily PSTN dial-in webhook (POST /daily-dialin-webhook). Only with runner_transport=daily.
	Dialin bool `json:"dialin,omitempty"`
	// DailyDialinWebhookSecret when set requires X-Webhook-Secret header to match for POST /daily-dialin-webhook. Overridden by VOXRAY_DAILY_DIALIN_WEBHOOK_SECRET.
	DailyDialinWebhookSecret string `json:"daily_dialin_webhook_secret,omitempty"`

	// Session store for runner sessions (POST /start, /sessions/{id}/...).
	// "memory" (default): in-memory per process; use for single instance or vertical scaling.
	// "redis": shared store via Redis; use for horizontal scaling behind a load balancer.
	SessionStore string `json:"session_store,omitempty"`
	// RedisURL is the Redis connection URL (e.g. redis://localhost:6379/0). Required when session_store is "redis".
	RedisURL string `json:"redis_url,omitempty"`
	// SessionTTLSecs is the TTL for sessions in seconds (default 3600). Applies to Redis store; optional for memory store.
	SessionTTLSecs int `json:"session_ttl_secs,omitempty"`

	// TLS: enable TLS and cert/key paths. Can be overridden by VOXRAY_TLS_* env vars.
	TLSEnable   bool   `json:"tls_enable,omitempty"`
	TLSCertFile string `json:"tls_cert_file,omitempty"`
	TLSKeyFile  string `json:"tls_key_file,omitempty"`

	// LogLevel is "debug", "info", or "error". Overridden by VOXRAY_LOG_LEVEL.
	LogLevel string `json:"log_level,omitempty"`
	// JSONLogs enables one-JSON-object-per-line logging. Overridden by VOXRAY_JSON_LOGS.
	JSONLogs bool `json:"json_logs,omitempty"`

	// CORSAllowedOrigins is a list of origins allowed for CORS (e.g. https://app.example.com). Empty means no CORS headers. Overridden by VOXRAY_CORS_ORIGINS (comma-separated).
	CORSAllowedOrigins []string `json:"cors_allowed_origins,omitempty"`

	// MaxRequestBodyBytes limits JSON request body size (e.g. /webrtc/offer, /start). Zero = no limit. Overridden by VOXRAY_MAX_BODY_BYTES.
	MaxRequestBodyBytes int64 `json:"max_request_body_bytes,omitempty"`

	// ServerAPIKey when non-empty requires Authorization: Bearer <key> or X-API-Key: <key> for /start, /sessions/*, /webrtc/offer, /ws. Overridden by VOXRAY_SERVER_API_KEY.
	ServerAPIKey string `json:"server_api_key,omitempty"`

	// MCP configures the MCP (Model Context Protocol) client for tool integration. When set, tools from the MCP server are registered with the LLM.
	MCP *MCPConfig `json:"mcp,omitempty"`

	// MaxConcurrentSessions caps the number of concurrent voice sessions (WebSocket and WebRTC).
	// When the limit is reached, new connections receive HTTP 503 Service Unavailable.
	// Zero means no limit (default).
	MaxConcurrentSessions int `json:"max_concurrent_sessions,omitempty"`

	// MetricsEnabled toggles Prometheus metrics collection and exposure at /metrics.
	// When omitted, metrics default to enabled to match README docs.
	// When set explicitly to false, handlers avoid recording metrics and /metrics still exists but exports an empty registry.
	MetricsEnabled *bool `json:"metrics_enabled,omitempty"`

	// Recording controls conversation-wide audio recording and async upload.
	Recording RecordingConfig `json:"recording,omitempty"`

	// Transcripts controls per-message transcript logging to an external database.
	Transcripts TranscriptConfig `json:"transcripts,omitempty"`
}

// RecordingConfig controls per-call/session audio recording.
type RecordingConfig struct {
	// Enable turns recording on for all sessions.
	Enable bool `json:"enable,omitempty"`
	// Bucket is the destination S3 bucket for recordings.
	Bucket string `json:"bucket,omitempty"`
	// BasePath is the key prefix within the bucket (e.g. "recordings/").
	BasePath string `json:"base_path,omitempty"`
	// Format is the file format/extension to use (e.g. "wav").
	Format string `json:"format,omitempty"`
	// WorkerCount is the number of async uploader workers (thread pool size).
	WorkerCount int `json:"worker_count,omitempty"`
}

// TranscriptConfig controls per-message transcript storage in a SQL database.
type TranscriptConfig struct {
	// Enable turns transcript logging on for all sessions.
	Enable bool `json:"enable,omitempty"`
	// Driver is the SQL driver name, e.g. "postgres" or "mysql".
	Driver string `json:"driver,omitempty"`
	// DSN is the driver-specific data source name / connection string.
	DSN string `json:"dsn,omitempty"`
	// TableName is the table to write transcript rows into (default "call_transcripts").
	TableName string `json:"table_name,omitempty"`
}

// MCPConfig configures an MCP server connection (stdio: command + args). Used to register MCP tools with the LLM.
type MCPConfig struct {
	Command     string   `json:"command"`               // executable (e.g. "npx", "go")
	Args        []string `json:"args,omitempty"`        // arguments (e.g. ["-y", "mcp-server"] or ["run", "server.go"])
	ToolsFilter []string `json:"tools_filter,omitempty"` // if non-empty, only these tool names are registered
}

// GetAPIKey returns the API key for the given service, checking the config first,
// then falling back to environment variables.
func (c *Config) GetAPIKey(service string, envVar string) string {
	if c.APIKeys != nil {
		if key, ok := c.APIKeys[service]; ok && key != "" {
			return key
		}
	}
	return os.Getenv(envVar)
}

// STTProvider returns the provider to use for STT (stt_provider if set, else provider).
func (c *Config) STTProvider() string {
	if c.SttProvider != "" {
		return c.SttProvider
	}
	return c.Provider
}

// LLMProvider returns the provider to use for LLM (llm_provider if set, else provider).
func (c *Config) LLMProvider() string {
	if c.LlmProvider != "" {
		return c.LlmProvider
	}
	return c.Provider
}

// TTSProvider returns the provider to use for TTS (tts_provider if set, else provider).
func (c *Config) TTSProvider() string {
	if c.TtsProvider != "" {
		return c.TtsProvider
	}
	return c.Provider
}

// TurnEnabled returns true when turn detection is set to "silence".
func (c *Config) TurnEnabled() bool {
	return c.TurnDetection == "silence"
}

// VADBackendOrDefault returns the configured VAD backend, defaulting to "energy".
func (c *Config) VADBackendOrDefault() string {
	if c.VADType == "" {
		return "energy"
	}
	return c.VADType
}

// VADParams returns a simple struct with the configured VAD parameters.
// Zero-values are allowed; the consumer (audio/vad) applies its own defaults.
func (c *Config) VADParams() (p struct {
	Confidence float64
	StartSecs  float64
	StopSecs   float64
	MinVolume  float64
}) {
	p.Confidence = c.VADConfidence
	if c.VADStartSecsVAD != 0 {
		p.StartSecs = c.VADStartSecsVAD
	}
	if c.VADStopSecs != 0 {
		p.StopSecs = c.VADStopSecs
	}
	p.MinVolume = c.VADMinVolume
	return p
}

// MetricsEnabledOrDefault reports whether Prometheus metrics should be recorded and exported.
// When cfg or MetricsEnabled is nil, it defaults to true (metrics on by default).
func (c *Config) MetricsEnabledOrDefault() bool {
	if c == nil {
		return true
	}
	if c.MetricsEnabled == nil {
		return true
	}
	return *c.MetricsEnabled
}

// LoadConfig reads a JSON configuration file from the specified path and returns a Config struct.
// It returns an error if the file cannot be read or if the JSON format is invalid.
// Call ApplyEnvOverrides(cfg) after LoadConfig to apply 12-factor env overrides.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config format: %v", err)
	}

	ApplyEnvOverrides(&cfg)
	return &cfg, nil
}

// ApplyEnvOverrides applies environment variable overrides to cfg (12-factor config).
// VOXRAY_PORT or PORT, VOXRAY_HOST or HOST, VOXRAY_LOG_LEVEL, VOXRAY_JSON_LOGS,
// VOXRAY_TLS_ENABLE, VOXRAY_TLS_CERT_FILE, VOXRAY_TLS_KEY_FILE, VOXRAY_CORS_ORIGINS (comma-separated),
// VOXRAY_MAX_BODY_BYTES. Unset env vars leave cfg unchanged.
func ApplyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	if v := os.Getenv("VOXRAY_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	} else if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("VOXRAY_HOST"); v != "" {
		cfg.Host = v
	} else if v := os.Getenv("HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("VOXRAY_LOG_LEVEL"); v != "" {
		cfg.LogLevel = strings.ToLower(strings.TrimSpace(v))
	}
	if v := os.Getenv("VOXRAY_JSON_LOGS"); v != "" {
		cfg.JSONLogs = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("VOXRAY_TLS_ENABLE"); v != "" {
		cfg.TLSEnable = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("VOXRAY_TLS_CERT_FILE"); v != "" {
		cfg.TLSCertFile = v
	}
	if v := os.Getenv("VOXRAY_TLS_KEY_FILE"); v != "" {
		cfg.TLSKeyFile = v
	}
	if v := os.Getenv("VOXRAY_CORS_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		cfg.CORSAllowedOrigins = make([]string, 0, len(parts))
		for _, p := range parts {
			if o := strings.TrimSpace(p); o != "" {
				cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, o)
			}
		}
	}
	if v := os.Getenv("VOXRAY_MAX_BODY_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			cfg.MaxRequestBodyBytes = n
		}
	}
	if v := os.Getenv("VOXRAY_SERVER_API_KEY"); v != "" {
		cfg.ServerAPIKey = v
	}
	if v := os.Getenv("VOXRAY_DAILY_DIALIN_WEBHOOK_SECRET"); v != "" {
		cfg.DailyDialinWebhookSecret = v
	}
	if v := os.Getenv("VOXRAY_RECORDING_ENABLE"); v != "" {
		cfg.Recording.Enable = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("VOXRAY_RECORDING_BUCKET"); v != "" {
		cfg.Recording.Bucket = v
	}
	if v := os.Getenv("VOXRAY_RECORDING_BASE_PATH"); v != "" {
		cfg.Recording.BasePath = v
	}
	if v := os.Getenv("VOXRAY_RECORDING_FORMAT"); v != "" {
		cfg.Recording.Format = v
	}
	if v := os.Getenv("VOXRAY_RECORDING_WORKER_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.Recording.WorkerCount = n
		}
	}
	if v := os.Getenv("VOXRAY_TRANSCRIPTS_ENABLE"); v != "" {
		cfg.Transcripts.Enable = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("VOXRAY_TRANSCRIPTS_DRIVER"); v != "" {
		cfg.Transcripts.Driver = v
	}
	if v := os.Getenv("VOXRAY_TRANSCRIPTS_DSN"); v != "" {
		cfg.Transcripts.DSN = v
	}
	if v := os.Getenv("VOXRAY_TRANSCRIPTS_TABLE"); v != "" {
		cfg.Transcripts.TableName = v
	}
}

// GetEnv returns the value of an environment variable, or def if unset.
// Used for API keys (e.g. OPENAI_API_KEY).
func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
