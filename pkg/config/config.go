// Package config handles the application configuration, including environment variables and JSON files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the server configuration, typically loaded from a JSON file or environment.
// It aligns with the Pipecat Python settings concept (see pipecat/services/settings.py): provider
// and model per task, with optional stt_model, tts_model, tts_voice. API keys are resolved via
// APIKeys map or environment (e.g. OPENAI_API_KEY, GROQ_API_KEY). Use GetAPIKey(service, envVar)
// for provider-specific lookup; the services factory uses this for construction.
//
// Provider is the default for all tasks (STT, LLM, TTS); stt_provider, llm_provider, tts_provider override when set.
// Model is the chat/LLM model; stt_model, tts_model, tts_voice are task-specific and optional.
type Config struct {
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Model       string            `json:"model"`
	Provider    string            `json:"provider,omitempty"` // default for all tasks; "openai" or "groq"
	SttProvider string            `json:"stt_provider,omitempty"`
	LlmProvider string            `json:"llm_provider,omitempty"`
	TtsProvider string            `json:"tts_provider,omitempty"`
	STTModel   string            `json:"stt_model,omitempty"`
	TTSModel   string            `json:"tts_model,omitempty"`
	TTSVoice   string            `json:"tts_voice,omitempty"`
	Plugins     []string          `json:"plugins"`
	APIKeys     map[string]string `json:"api_keys,omitempty"`

	// Turn detection (pipecat audio/turn): when to consider user finished speaking
	TurnDetection       string  `json:"turn_detection,omitempty"`        // "none" | "silence"; default "none"
	TurnStopSecs        float64 `json:"turn_stop_secs,omitempty"`        // silence after speech to end turn (default 3)
	TurnPreSpeechMs     float64 `json:"turn_pre_speech_ms,omitempty"`  // pre-speech padding ms (default 500)
	TurnMaxDurationSecs float64 `json:"turn_max_duration_secs,omitempty"` // max segment duration secs (default 8)
	VADStartSecs        float64 `json:"vad_start_secs,omitempty"`        // VAD start trigger time for turn (default 0)
	VadThreshold        float64 `json:"vad_threshold,omitempty"`         // EnergyDetector RMS threshold (default 0.02)
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

// LoadConfig reads a JSON configuration file from the specified path and returns a Config struct.
// It returns an error if the file cannot be read or if the JSON format is invalid.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config format: %v", err)
	}

	return &cfg, nil
}

// GetEnv returns the value of an environment variable, or def if unset.
// Used for API keys (e.g. OPENAI_API_KEY).
func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
