package logger_test

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/logger"
)

func TestLogger_ProcessFrame(t *testing.T) {
	p := logger.New("log")
	ctx := context.Background()
	f := frames.NewTextFrame("test")
	if err := p.ProcessFrame(ctx, f, processors.Downstream); err != nil {
		t.Fatal(err)
	}
}

func TestLogger_NewDefaultName(t *testing.T) {
	p := logger.New("")
	if p.Name() != "Logger" {
		t.Errorf("default name: got %q", p.Name())
	}
}

func TestLogger_New(t *testing.T) {
	p := logger.New("log")
	if p == nil || p.Name() != "log" {
		t.Errorf("New: got %v", p)
	}
}

