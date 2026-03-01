// Package llmfullresponse provides a processor that aggregates LLM text between
// LLMFullResponseStartFrame and LLMFullResponseEndFrame and invokes a callback on completion or interruption.
package llmfullresponse

import (
	"context"
	"strings"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// OnCompletion is called when a full response ends (completed=true) or is interrupted (completed=false).
// Text is the aggregated content so far.
type OnCompletion func(text string, completed bool)

// Processor aggregates LLMTextFrame between start/end frames and calls OnCompletion.
type Processor struct {
	*processors.BaseProcessor
	OnCompletion OnCompletion

	mu        sync.Mutex
	buf       strings.Builder
	started   bool
}

// New returns an LLM full-response aggregator. onCompletion may be nil.
func New(name string, onCompletion OnCompletion) *Processor {
	if name == "" {
		name = "LLMFullResponseAggregator"
	}
	return &Processor{
		BaseProcessor: processors.NewBaseProcessor(name),
		OnCompletion:  onCompletion,
	}
}

// ProcessFrame accumulates LLM text; forwards all frames; calls OnCompletion on end or interruption.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.InterruptionFrame:
		p.mu.Lock()
		text := p.buf.String()
		p.buf.Reset()
		p.started = false
		p.mu.Unlock()
		if p.OnCompletion != nil {
			p.OnCompletion(text, false)
		}
		return p.PushDownstream(ctx, f)

	case *frames.LLMFullResponseStartFrame:
		p.mu.Lock()
		p.started = true
		p.mu.Unlock()
		return p.PushDownstream(ctx, f)

	case *frames.LLMFullResponseEndFrame:
		p.mu.Lock()
		text := p.buf.String()
		p.buf.Reset()
		p.started = false
		p.mu.Unlock()
		if p.OnCompletion != nil {
			p.OnCompletion(text, true)
		}
		return p.PushDownstream(ctx, f)

	case *frames.LLMTextFrame:
		p.mu.Lock()
		if p.started {
			p.buf.WriteString(t.Text)
		}
		p.mu.Unlock()
		return p.PushDownstream(ctx, f)

	default:
		return p.PushDownstream(ctx, f)
	}
}
