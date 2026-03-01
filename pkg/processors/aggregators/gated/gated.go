// Package gated provides a gated aggregator that buffers frames when the gate is
// closed and releases them when the gate opens (custom open/close predicates).
package gated

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

// GateFunc returns true when the frame should open or close the gate.
type GateFunc func(f frames.Frame) bool

// Processor accumulates frames when the gate is closed; when the gate opens,
// it pushes the gate-opening frame (if any) then all buffered frames.
type Processor struct {
	*processors.BaseProcessor
	GateOpen   GateFunc
	GateClose  GateFunc
	StartOpen  bool
	Direction  processors.Direction

	mu          sync.Mutex
	gateOpen    bool
	accumulator []frames.Frame
}

// New returns a gated aggregator. GateOpen and GateClose must be non-nil.
func New(name string, gateOpen, gateClose GateFunc, startOpen bool, dir processors.Direction) *Processor {
	if name == "" {
		name = "GatedAggregator"
	}
	if gateOpen == nil {
		gateOpen = func(frames.Frame) bool { return false }
	}
	if gateClose == nil {
		gateClose = func(frames.Frame) bool { return false }
	}
	if dir == 0 {
		dir = processors.Downstream
	}
	return &Processor{
		BaseProcessor: processors.NewBaseProcessor(name),
		GateOpen:      gateOpen,
		GateClose:     gateClose,
		StartOpen:     startOpen,
		Direction:     dir,
		gateOpen:      startOpen,
		accumulator:   nil,
	}
}

// ProcessFrame buffers or forwards frames based on gate state.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if isSystemFrame(f) {
		return p.PushDownstream(ctx, f)
	}

	if dir != p.Direction {
		return p.PushDownstream(ctx, f)
	}

	p.mu.Lock()
	oldOpen := p.gateOpen
	if p.gateOpen {
		p.gateOpen = !p.GateClose(f)
	} else {
		p.gateOpen = p.GateOpen(f)
	}
	newOpen := p.gateOpen

	if oldOpen != newOpen && newOpen {
		acc := p.accumulator
		p.accumulator = nil
		p.mu.Unlock()
		// Gate just opened: push current frame then any buffered frames
		if err := p.PushDownstream(ctx, f); err != nil {
			return err
		}
		for _, b := range acc {
			if err := p.PushDownstream(ctx, b); err != nil {
				return err
			}
		}
		return nil
	}

	if newOpen {
		p.mu.Unlock()
		return p.PushDownstream(ctx, f)
	}

	if p.accumulator == nil {
		p.accumulator = make([]frames.Frame, 0, 8)
	}
	p.accumulator = append(p.accumulator, f)
	p.mu.Unlock()
	return nil
}

func isSystemFrame(f frames.Frame) bool {
	switch f.(type) {
	case *frames.StartFrame, *frames.EndFrame, *frames.CancelFrame, *frames.ErrorFrame, *frames.StopFrame:
		return true
	}
	return false
}
