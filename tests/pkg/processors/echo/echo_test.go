package echo_test

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/echo"
)

func TestEcho_ProcessFrame(t *testing.T) {
	e := echo.New("echo")
	ctx := context.Background()
	f := frames.NewTextFrame("hello")
	if err := e.ProcessFrame(ctx, f, processors.Downstream); err != nil {
		t.Fatal(err)
	}
	// Echo forwards to next; no next so no-op
}

func TestEcho_NewDefaultName(t *testing.T) {
	e := echo.New("")
	if e.Name() != "Echo" {
		t.Errorf("default name: got %q", e.Name())
	}
}

func TestEcho_New(t *testing.T) {
	p := echo.New("echo")
	if p == nil || p.Name() != "echo" {
		t.Errorf("New: got %v", p)
	}
}

