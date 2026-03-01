package processors_test

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

func TestNewBaseProcessor(t *testing.T) {
	b := processors.NewBaseProcessor("test")
	if b.Name() != "test" {
		t.Errorf("Name() = %q", b.Name())
	}
}

func TestBaseProcessor_ImplementsProcessor(t *testing.T) {
	var _ processors.Processor = (*processors.BaseProcessor)(nil)
}

func TestBaseProcessor_ProcessFrame_NoNext(t *testing.T) {
	b := processors.NewBaseProcessor("x")
	err := b.ProcessFrame(context.Background(), frames.NewStartFrame(), processors.Downstream)
	if err != nil {
		t.Errorf("ProcessFrame with no next should not error: %v", err)
	}
}
