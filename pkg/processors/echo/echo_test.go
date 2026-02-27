package echo

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

func TestEcho_ProcessFrame(t *testing.T) {
	e := New("echo")
	ctx := context.Background()
	f := frames.NewTextFrame("hello")
	if err := e.ProcessFrame(ctx, f, processors.Downstream); err != nil {
		t.Fatal(err)
	}
	// Echo forwards to next; no next so no-op
}

func TestEcho_NewDefaultName(t *testing.T) {
	e := New("")
	if e.Name() != "Echo" {
		t.Errorf("default name: got %q", e.Name())
	}
}
