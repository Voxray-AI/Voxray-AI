// Package main provides the init subcommand to scaffold config and dirs.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfig = `{
  "host": "localhost",
  "port": 8080,
  "model": "gpt-3.5-turbo",
  "provider": "openai",
  "plugins": ["echo"],
  "turn_detection": "none",
  "turn_stop_secs": 3,
  "turn_pre_speech_ms": 500,
  "turn_max_duration_secs": 8,
  "vad_start_secs": 0,
  "vad_threshold": 0.02
}
`

// runInit scaffolds config.json and optional directories (plugins, logs).
func runInit(configPath string, createDirs bool) error {
	if configPath == "" {
		configPath = "config.json"
	}
	// Write default config
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(defaultConfig), &cfg); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	fmt.Fprintf(os.Stderr, "Created %s\n", configPath)

	if createDirs {
		for _, dir := range []string{"plugins", "logs"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dir, err)
			}
			abs, _ := filepath.Abs(dir)
			fmt.Fprintf(os.Stderr, "Created directory %s\n", abs)
		}
	}
	return nil
}
