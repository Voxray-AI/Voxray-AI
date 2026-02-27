package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte(`{"host":"","port":9000,"model":"gpt-4","plugins":["echo"]}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9000 || cfg.Model != "gpt-4" || len(cfg.Plugins) != 1 || cfg.Plugins[0] != "echo" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KEY", "val1")
	defer os.Unsetenv("TEST_KEY")
	if got := GetEnv("TEST_KEY", "def"); got != "val1" {
		t.Errorf("GetEnv(TEST_KEY) = %q want val1", got)
	}
	if got := GetEnv("MISSING_KEY", "def"); got != "def" {
		t.Errorf("GetEnv(MISSING_KEY) = %q want def", got)
	}
}
func TestGetAPIKey(t *testing.T) {
	cfg := &Config{
		APIKeys: map[string]string{
			"openai": "config-key",
		},
	}

	// Should prioritize config key
	if got := cfg.GetAPIKey("openai", "OPENAI_API_KEY"); got != "config-key" {
		t.Errorf("GetAPIKey(openai) = %q, want config-key", got)
	}

	// Should fallback to environment variable
	os.Setenv("TEST_ENV_KEY", "env-key")
	defer os.Unsetenv("TEST_ENV_KEY")
	if got := cfg.GetAPIKey("missing", "TEST_ENV_KEY"); got != "env-key" {
		t.Errorf("GetAPIKey(missing) = %q, want env-key", got)
	}

	// Should return empty if neither is present
	if got := cfg.GetAPIKey("missing", "TOTALLY_MISSING"); got != "" {
		t.Errorf("GetAPIKey(missing) = %q, want empty string", got)
	}
}
