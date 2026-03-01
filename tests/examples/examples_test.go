package main_test

import (
	"os/exec"
	"testing"
)

func TestBuildExamples(t *testing.T) {
	cmd := exec.Command("go", "build", "./examples/...")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./examples/...: %v\n%s", err, out)
	}
}
