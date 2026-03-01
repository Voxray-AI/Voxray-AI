// Package gatedcontext provides a processor that holds LLMContextFrame until a notifier signals release.
package gatedcontext

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/utils/notifier"
)

// Processor holds the latest LLMContextFrame until Notify() is called on the notifier, then pushes it.
type Processor struct {
	*processors.BaseProcessor
	Notifier  *notifier.Notifier
	StartOpen bool

	mu        sync.Mutex
	lastFrame frames.Frame
	runCtx    context.Context
	started   bool
}

// New returns a gated LLM context processor. When Notifier.Notify() is called, the last stored LLMContextFrame is pushed downstream.
func New(name string, n *notifier.Notifier, startOpen bool) *Processor {
	if name == "" {
		name = "GatedLLMContextAggregator"
	}
	if n == nil {
		n = notifier.New()
	}
	return &Processor{
		BaseProcessor: processors.NewBaseProcessor(name),
		Notifier:      n,
		StartOpen:     startOpen,
	}
}

// Setup starts the goroutine that waits on the notifier and pushes the stored frame.
func (p *Processor) Setup(ctx context.Context) error {
	p.mu.Lock()
	p.runCtx = ctx
	if !p.started {
		p.started = true
		go p.gateLoop()
	}
	p.mu.Unlock()
	return p.BaseProcessor.Setup(ctx)
}

func (p *Processor) gateLoop() {
	for {
		if p.runCtx == nil {
			return
		}
		if err := p.Notifier.Wait(p.runCtx); err != nil {
			return
		}
		p.mu.Lock()
		f := p.lastFrame
		p.lastFrame = nil
		ctx := p.runCtx
		p.mu.Unlock()
		if f != nil {
			_ = p.PushDownstream(ctx, f)
		}
	}
}

// ProcessFrame holds LLMContextFrame until notifier signals; other frames pass through.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.StartFrame:
		return p.PushDownstream(ctx, f)

	case *frames.EndFrame, *frames.CancelFrame:
		p.mu.Lock()
		p.lastFrame = nil
		p.mu.Unlock()
		return p.PushDownstream(ctx, f)

	case *frames.LLMContextFrame:
		if p.StartOpen {
			p.StartOpen = false
			return p.PushDownstream(ctx, f)
		}
		p.mu.Lock()
		p.lastFrame = t
		p.mu.Unlock()
		return nil

	default:
		return p.PushDownstream(ctx, f)
	}
}
