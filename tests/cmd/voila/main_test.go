package main_test

import (
	"os/exec"
	"testing"
)

func TestBuildVoila(t *testing.T) {
	cmd := exec.Command("go", "build", "./cmd/voila")
	cmd.Dir = "../../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/voila: %v\n%s", err, out)
	}
}
