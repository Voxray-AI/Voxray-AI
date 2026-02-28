// evals runs eval scenarios from a JSON config (LLM-only pipeline, prompt → assert on output).
//
// Usage:
//
//	go run ./cmd/evals -config scripts/evals/config/scenarios.json -voila-config config.json
//
// Flags:
//   -config       path to eval scenarios JSON (default: scripts/evals/config/scenarios.json)
//   -voila-config path to voila config for API keys and provider (default: config.json)
//   -scenario     run only scenario with this name (default: all)
//   -out-dir      directory for results.json (default: scripts/evals/test-runs/<timestamp>)
//   -v            verbose
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"voila-go/pkg/evals"
)

func main() {
	configPath := flag.String("config", "scripts/evals/config/scenarios.json", "path to eval scenarios JSON")
	voilaConfigPath := flag.String("voila-config", "config.json", "path to voila config (API keys, provider)")
	scenarioName := flag.String("scenario", "", "run only this scenario (default: all)")
	outDir := flag.String("out-dir", "", "output directory for results (default: scripts/evals/test-runs/<timestamp>)")
	verbose := flag.Bool("v", false, "verbose")
	flag.Parse()

	cfg, err := evals.LoadEvalConfig(*configPath)
	if err != nil {
		log.Fatalf("load eval config %s: %v", *configPath, err)
	}

	scenarios := cfg.Scenarios
	if *scenarioName != "" {
		var found []evals.EvalScenario
		for _, s := range scenarios {
			if s.Name == *scenarioName {
				found = append(found, s)
				break
			}
		}
		if len(found) == 0 {
			log.Fatalf("scenario %q not found in config", *scenarioName)
		}
		scenarios = found
	}

	if *outDir == "" {
		*outDir = filepath.Join("scripts", "evals", "test-runs", time.Now().Format("20060102_150405"))
	}

	var results []evals.EvalResult
	ctx := context.Background()
	for _, s := range scenarios {
		if *verbose {
			fmt.Printf("Running scenario: %s\n", s.Name)
		}
		result := evals.RunScenario(ctx, *voilaConfigPath, s)
		results = append(results, result)
		if *verbose && result.Error != "" {
			fmt.Printf("  error: %s\n", result.Error)
		}
	}

	if err := evals.WriteReport(results, *outDir); err != nil {
		log.Fatalf("write report: %v", err)
	}

	fail := 0
	for _, r := range results {
		if !r.Pass {
			fail++
		}
	}
	if fail > 0 {
		os.Exit(1)
	}
}
