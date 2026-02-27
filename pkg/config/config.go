// Package config handles the application configuration, including environment variables and JSON files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the server configuration, typically loaded from a JSON file.
type Config struct {
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Model    string            `json:"model"`
	Provider string            `json:"provider,omitempty"` // "openai" or "groq"; empty defaults to openai
	Plugins  []string          `json:"plugins"`
	APIKeys  map[string]string `json:"api_keys,omitempty"`
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
