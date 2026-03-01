package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"voila-go/pkg/config"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte(`{"host":"","port":9000,"model":"gpt-4","plugins":["echo"]}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(path)
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
	if got := config.GetEnv("TEST_KEY", "def"); got != "val1" {
		t.Errorf("GetEnv(TEST_KEY) = %q want val1", got)
	}
	if got := config.GetEnv("MISSING_KEY", "def"); got != "def" {
		t.Errorf("GetEnv(MISSING_KEY) = %q want def", got)
	}
}

func TestGetAPIKey(t *testing.T) {
	cfg := &config.Config{
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

func TestSTTProvider(t *testing.T) {
	c := &config.Config{Provider: "openai"}
	if got := c.STTProvider(); got != "openai" {
		t.Errorf("STTProvider() = %q, want openai", got)
	}
	c.SttProvider = "groq"
	if got := c.STTProvider(); got != "groq" {
		t.Errorf("STTProvider() = %q, want groq", got)
	}
}

func TestVADParams(t *testing.T) {
	c := &config.Config{}
	p := c.VADParams()
	if p.Confidence != 0 || p.StartSecs != 0 || p.StopSecs != 0 || p.MinVolume != 0 {
		t.Errorf("VADParams() zero config should give zero struct: %+v", p)
	}
	c.VADConfidence = 0.7
	c.VADStartSecsVAD = 0.2
	c.VADStopSecs = 0.3
	c.VADMinVolume = 0.5
	p = c.VADParams()
	if p.Confidence != 0.7 || p.StartSecs != 0.2 || p.StopSecs != 0.3 || p.MinVolume != 0.5 {
		t.Errorf("VADParams() = %+v", p)
	}
}

func TestTurnEnabled(t *testing.T) {
	c := &config.Config{}
	if c.TurnEnabled() {
		t.Error("TurnEnabled() should be false when TurnDetection is empty")
	}
	c.TurnDetection = "silence"
	if !c.TurnEnabled() {
		t.Error("TurnEnabled() should be true when TurnDetection is silence")
	}
}

