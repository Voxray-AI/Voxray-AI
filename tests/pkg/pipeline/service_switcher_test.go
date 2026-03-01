package pipeline_test

import (
	"context"
	"sync"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
)

// serviceSwitcherMock is a processor that records received frames for testing which branch is active.
type serviceSwitcherMock struct {
	name    string
	mu      sync.Mutex
	received []frames.Frame
}

func (m *serviceSwitcherMock) ProcessFrame(_ context.Context, f frames.Frame, _ processors.Direction) error {
	m.mu.Lock()
	m.received = append(m.received, f)
	m.mu.Unlock()
	return nil
}
func (m *serviceSwitcherMock) SetNext(processors.Processor)   {}
func (m *serviceSwitcherMock) SetPrev(processors.Processor)   {}
func (m *serviceSwitcherMock) Setup(context.Context) error   { return nil }
func (m *serviceSwitcherMock) Cleanup(context.Context) error { return nil }
func (m *serviceSwitcherMock) Name() string                   { return m.name }

func (m *serviceSwitcherMock) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.received)
}

// TestServiceSwitcher_InitialActive mirrors upstream test_service_switcher: build with two services, assert initial active is first.
func TestServiceSwitcher_InitialActive(t *testing.T) {
	strategy := pipeline.NewServiceSwitcherStrategyManual([]string{"A", "B"})
	if strategy.ActiveServiceName() != "A" {
		t.Errorf("initial active service should be A, got %q", strategy.ActiveServiceName())
	}
}

// TestServiceSwitcher_SwitchToB builds ServiceSwitcher with two mock processors, pushes ManuallySwitchServiceFrame(B), asserts strategy switches and only active branch receives downstream frames.
func TestServiceSwitcher_SwitchToB(t *testing.T) {
	ctx := context.Background()
	mockA := &serviceSwitcherMock{name: "A"}
	mockB := &serviceSwitcherMock{name: "B"}
	strategy := pipeline.NewServiceSwitcherStrategyManual([]string{"A", "B"})
	ss, err := pipeline.NewServiceSwitcher([]struct {
		Name      string
		Processor processors.Processor
	}{
		{"A", mockA},
		{"B", mockB},
	}, strategy)
	if err != nil {
		t.Fatalf("NewServiceSwitcher: %v", err)
	}
	if strategy.ActiveServiceName() != "A" {
		t.Errorf("initial active should be A, got %q", strategy.ActiveServiceName())
	}
	if err := ss.Setup(ctx); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer ss.Cleanup(ctx)

	// Push StartFrame downstream: only active branch (A) should receive
	_ = ss.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream)
	if mockA.count() != 1 {
		t.Errorf("mockA should have received 1 frame, got %d", mockA.count())
	}
	if mockB.count() != 0 {
		t.Errorf("mockB should have received 0 frames (inactive), got %d", mockB.count())
	}

	// Switch to B
	_ = ss.ProcessFrame(ctx, frames.NewManuallySwitchServiceFrame("B"), processors.Downstream)
	if strategy.ActiveServiceName() != "B" {
		t.Errorf("after switch to B, active should be B, got %q", strategy.ActiveServiceName())
	}

	// Push another frame: only B should receive (B also received ServiceSwitcherRequestMetadataFrame on switch, so 2 total)
	_ = ss.ProcessFrame(ctx, frames.NewTextFrame("hello"), processors.Downstream)
	if mockA.count() != 1 {
		t.Errorf("mockA should still have 1 frame after switch, got %d", mockA.count())
	}
	if mockB.count() != 2 {
		t.Errorf("mockB should have received 2 frames (metadata on switch + TextFrame), got %d", mockB.count())
	}
}

// TestServiceSwitcher_InvalidSwitchName leaves active unchanged.
func TestServiceSwitcher_InvalidSwitchName(t *testing.T) {
	strategy := pipeline.NewServiceSwitcherStrategyManual([]string{"A", "B"})
	ctx := context.Background()
	mockA := &serviceSwitcherMock{name: "A"}
	mockB := &serviceSwitcherMock{name: "B"}
	ss, err := pipeline.NewServiceSwitcher([]struct {
		Name      string
		Processor processors.Processor
	}{
		{"A", mockA},
		{"B", mockB},
	}, strategy)
	if err != nil {
		t.Fatalf("NewServiceSwitcher: %v", err)
	}
	if err := ss.Setup(ctx); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer ss.Cleanup(ctx)

	_ = ss.ProcessFrame(ctx, frames.NewManuallySwitchServiceFrame("Unknown"), processors.Downstream)
	if strategy.ActiveServiceName() != "A" {
		t.Errorf("invalid switch name should leave active unchanged (A), got %q", strategy.ActiveServiceName())
	}
}

// TestNewServiceSwitcher_Errors returns error for empty services or nil strategy.
func TestNewServiceSwitcher_Errors(t *testing.T) {
	_, err := pipeline.NewServiceSwitcher(nil, pipeline.NewServiceSwitcherStrategyManual([]string{"A"}))
	if err == nil {
		t.Error("NewServiceSwitcher with no services should return error")
	}
	_, err = pipeline.NewServiceSwitcher([]struct {
		Name      string
		Processor processors.Processor
	}{{"A", &serviceSwitcherMock{name: "A"}}}, nil)
	if err == nil {
		t.Error("NewServiceSwitcher with nil strategy should return error")
	}
}

// TestNewLLMSwitcher builds LLMSwitcher and asserts it behaves like ServiceSwitcher (initial active, switch).
func TestNewLLMSwitcher(t *testing.T) {
	ctx := context.Background()
	mockA := &serviceSwitcherMock{name: "A"}
	mockB := &serviceSwitcherMock{name: "B"}
	strategy := pipeline.NewServiceSwitcherStrategyManual([]string{"A", "B"})
	ls, err := pipeline.NewLLMSwitcher([]struct {
		Name      string
		Processor processors.Processor
	}{
		{"A", mockA},
		{"B", mockB},
	}, strategy)
	if err != nil {
		t.Fatalf("NewLLMSwitcher: %v", err)
	}
	if strategy.ActiveServiceName() != "A" {
		t.Errorf("LLMSwitcher initial active should be A, got %q", strategy.ActiveServiceName())
	}
	if err := ls.Setup(ctx); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer ls.Cleanup(ctx)
	_ = ls.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream)
	_ = ls.ProcessFrame(ctx, frames.NewManuallySwitchServiceFrame("B"), processors.Downstream)
	if strategy.ActiveServiceName() != "B" {
		t.Errorf("after switch LLMSwitcher active should be B, got %q", strategy.ActiveServiceName())
	}
}
