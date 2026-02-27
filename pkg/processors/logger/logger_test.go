package logger

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

func TestLogger_ProcessFrame(t *testing.T) {
	p := New("log")
	ctx := context.Background()
	f := frames.NewTextFrame("test")
	if err := p.ProcessFrame(ctx, f, processors.Downstream); err != nil {
		t.Fatal(err)
	}
}

func TestLogger_NewDefaultName(t *testing.T) {
	p := New("")
	if p.Name() != "Logger" {
		t.Errorf("default name: got %q", p.Name())
	}
}
