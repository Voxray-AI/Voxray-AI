package main_test

import (
	"os/exec"
	"testing"
)

func TestBuildVoiceExample(t *testing.T) {
	cmd := exec.Command("go", "build", "./examples/voice")
	cmd.Dir = "../../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./examples/voice: %v\n%s", err, out)
	}
}
