// Package evals provides a Go-native eval runner for voice pipeline scenarios.
package evals

import (
	"encoding/json"
	"os"
)

// EvalScenario defines a single eval: prompt to send and how to assert the response.
type EvalScenario struct {
	Name             string  `json:"name"`
	Prompt           string  `json:"prompt"`
	ExpectedPattern  string  `json:"expected_pattern"`  // regex or substring; match in response => pass
	ExpectedContains string  `json:"expected_contains"` // alternative: simple substring (if set, used instead of regex)
	TimeoutSecs      float64 `json:"timeout_secs,omitempty"`
	SystemPrompt     string  `json:"system_prompt,omitempty"` // optional override for LLM system message
}

// EvalResult is the outcome of running one scenario.
type EvalResult struct {
	Name     string  `json:"name"`
	Pass     bool    `json:"pass"`
	Duration float64 `json:"duration_secs"`
	Output   string  `json:"output,omitempty"`
	Error    string  `json:"error,omitempty"`
}

// EvalConfig is the top-level config file (e.g. scenarios.json).
type EvalConfig struct {
	Scenarios []EvalScenario `json:"scenarios"`
}

// LoadEvalConfig reads and parses a JSON eval config from path.
func LoadEvalConfig(path string) (*EvalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg EvalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
