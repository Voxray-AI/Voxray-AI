package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteReport writes results to dir as results.json and prints a human-readable summary to stdout.
func WriteReport(results []EvalResult, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "results.json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		return err
	}

	// Summary to stdout
	var pass, fail int
	var totalTime float64
	for _, r := range results {
		totalTime += r.Duration
		if r.Pass {
			pass++
		} else {
			fail++
		}
	}
	fmt.Println()
	fmt.Println("================================================================================")
	fmt.Printf("TOTAL: %d  |  PASS: %d  |  FAIL: %d  |  TIME: %.2fs\n", len(results), pass, fail, totalTime)
	fmt.Println("================================================================================")
	for _, r := range results {
		status := "FAIL"
		if r.Pass {
			status = "PASS"
		}
		fmt.Printf("  %s  %s (%.2fs)\n", status, r.Name, r.Duration)
		if r.Error != "" {
			fmt.Printf("      error: %s\n", r.Error)
		}
	}
	fmt.Println()
	fmt.Printf("Results written to %s\n", path)
	return nil
}
