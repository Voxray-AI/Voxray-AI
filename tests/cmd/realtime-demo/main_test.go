package main_test

import (
	"os/exec"
	"testing"
)

func TestBuildRealtimeDemo(t *testing.T) {
	cmd := exec.Command("go", "build", "./cmd/realtime-demo")
	cmd.Dir = "../../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/realtime-demo: %v\n%s", err, out)
	}
}
