package observers

import (
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

func TestNoopObserver_OnFrameProcessed(t *testing.T) {
	var o NoopObserver
	// Should not panic.
	o.OnFrameProcessed("test", frames.NewStartFrame(), processors.Downstream)
}

func TestNewCompositeObserver(t *testing.T) {
	c := NewCompositeObserver()
	if c == nil {
		t.Fatal("NewCompositeObserver returned nil")
	}
	// With no observers, OnFrameProcessed should not panic.
	c.OnFrameProcessed("p", frames.NewTextFrame("x"), processors.Downstream)
}

func TestNewCompositeObserver_Delegates(t *testing.T) {
	called := false
	o := &mockObserver{onProcessed: func() { called = true }}
	c := NewCompositeObserver(o)
	c.OnFrameProcessed("p", frames.NewStartFrame(), processors.Downstream)
	if !called {
		t.Error("CompositeObserver did not call child OnFrameProcessed")
	}
}

type mockObserver struct {
	onProcessed func()
}

func (m *mockObserver) OnFrameProcessed(_ string, _ frames.Frame, _ processors.Direction) {
	if m.onProcessed != nil {
		m.onProcessed()
	}
}
