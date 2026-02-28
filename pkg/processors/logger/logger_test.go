package logger

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

func TestNew(t *testing.T) {
	p := New("log")
	if p == nil || p.Name() != "log" {
		t.Errorf("New: got %v", p)
	}
}

func TestLogger_ProcessFrame(t *testing.T) {
	p := New("log")
	ctx := context.Background()
	err := p.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream)
	if err != nil {
		t.Fatal(err)
	}
}
