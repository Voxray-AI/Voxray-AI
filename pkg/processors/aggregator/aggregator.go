// Package aggregator provides a processor that collects text frames and emits a single aggregated frame (e.g. sentence).
package aggregator

import (
	"context"
	"strings"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// Processor collects TextFrame (and LLMTextFrame) chunks and emits one TextFrame when a sentence boundary is seen or buffer exceeds max size.
type Processor struct {
	*processors.BaseProcessor
	// SentenceEnd marks end of sentence (e.g. ".!?").
	SentenceEnd string
	// MaxBuffer is the maximum rune count before flushing without a sentence end (0 = no limit).
	MaxBuffer int
	buf       strings.Builder
}

// New returns an aggregator processor. SentenceEnd defaults to ".!?"; MaxBuffer 0 means no limit.
func New(name string, sentenceEnd string, maxBuffer int) *Processor {
	if name == "" {
		name = "Aggregator"
	}
	if sentenceEnd == "" {
		sentenceEnd = ".!?"
	}
	return &Processor{
		BaseProcessor: processors.NewBaseProcessor(name),
		SentenceEnd:   sentenceEnd,
		MaxBuffer:     maxBuffer,
	}
}

// ProcessFrame accumulates text from TextFrame/LLMTextFrame and forwards other frames; emits aggregated TextFrame on sentence end or buffer limit.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.TextFrame:
		p.buf.WriteString(t.Text)
		p.tryFlush(ctx)
		return nil
	case *frames.LLMTextFrame:
		p.buf.WriteString(t.Text)
		p.tryFlush(ctx)
		return nil
	default:
		// Flush any pending text before forwarding
		p.flush(ctx)
		return p.PushDownstream(ctx, f)
	}
}

func (p *Processor) tryFlush(ctx context.Context) {
	s := p.buf.String()
	flush := false
	if p.MaxBuffer > 0 && len([]rune(s)) >= p.MaxBuffer {
		flush = true
	} else {
		for _, r := range p.SentenceEnd {
			if strings.ContainsRune(s, r) {
				flush = true
				break
			}
		}
	}
	if flush {
		p.flush(ctx)
	}
}

func (p *Processor) flush(ctx context.Context) {
	if p.buf.Len() == 0 {
		return
	}
	text := strings.TrimSpace(p.buf.String())
	p.buf.Reset()
	if text == "" {
		return
	}
	out := frames.NewTextFrame(text)
	_ = p.PushDownstream(ctx, out)
}
