// Package llmtext provides a processor that converts LLMTextFrame to AggregatedTextFrame
// using a configurable text aggregator (e.g. sentence-based).
package llmtext

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/utils/textaggregator"
)

// Processor converts LLMTextFrame into AggregatedTextFrame via a text aggregator.
type Processor struct {
	*processors.BaseProcessor
	Aggregator textaggregator.Aggregator
}

// New returns an LLM text processor. If agg is nil, a default sentence aggregator is used.
func New(name string, agg textaggregator.Aggregator) *Processor {
	if name == "" {
		name = "LLMTextProcessor"
	}
	if agg == nil {
		agg = textaggregator.NewSentenceAggregator("", 0)
	}
	return &Processor{
		BaseProcessor: processors.NewBaseProcessor(name),
		Aggregator:     agg,
	}
}

// ProcessFrame feeds LLM text into the aggregator and emits AggregatedTextFrame; flushes on end/interruption.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.InterruptionFrame:
		p.Aggregator.HandleInterruption()
		return p.PushDownstream(ctx, f)

	case *frames.LLMTextFrame:
		segments := p.Aggregator.Aggregate(t.Text)
		for _, seg := range segments {
			out := frames.NewAggregatedTextFrame(seg.Text, seg.Type)
			if t.SkipTTS != nil {
				out.SkipTTS = t.SkipTTS
			}
			if err := p.PushDownstream(ctx, out); err != nil {
				return err
			}
		}
		return nil

	case *frames.LLMFullResponseEndFrame:
		if seg := p.Aggregator.Flush(); seg != nil {
			out := frames.NewAggregatedTextFrame(seg.Text, seg.Type)
			if err := p.PushDownstream(ctx, out); err != nil {
				return err
			}
		}
		return p.PushDownstream(ctx, f)

	case *frames.EndFrame:
		if seg := p.Aggregator.Flush(); seg != nil {
			out := frames.NewAggregatedTextFrame(seg.Text, seg.Type)
			if err := p.PushDownstream(ctx, out); err != nil {
				return err
			}
		}
		return p.PushDownstream(ctx, f)

	default:
		return p.PushDownstream(ctx, f)
	}
}
