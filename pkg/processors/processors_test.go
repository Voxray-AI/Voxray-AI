package processors

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
)

func TestNewBaseProcessor(t *testing.T) {
	b := NewBaseProcessor("test")
	if b.Name() != "test" {
		t.Errorf("Name() = %q", b.Name())
	}
}

func TestBaseProcessor_ImplementsProcessor(t *testing.T) {
	var _ Processor = (*BaseProcessor)(nil)
}

func TestBaseProcessor_ProcessFrame_NoNext(t *testing.T) {
	b := NewBaseProcessor("x")
	err := b.ProcessFrame(context.Background(), frames.NewStartFrame(), Downstream)
	if err != nil {
		t.Errorf("ProcessFrame with no next should not error: %v", err)
	}
}
