package voice

import (
	"context"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/services"
)

// LLMProcessor runs the LLM on transcription/context and streams LLMTextFrame downstream.
type LLMProcessor struct {
	*processors.BaseProcessor
	LLM   services.LLMService
	mu    sync.Mutex
	msgs  []map[string]any
}

// NewLLMProcessor returns a processor that runs the LLM and streams text downstream.
func NewLLMProcessor(name string, llm services.LLMService) *LLMProcessor {
	if name == "" {
		name = "LLM"
	}
	return &LLMProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		LLM:           llm,
		msgs:          make([]map[string]any, 0),
	}
}

// ProcessFrame runs LLM on TranscriptionFrame (appends user message) or LLMRunFrame; streams LLMTextFrame downstream. Forwards other frames.
func (p *LLMProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.TranscriptionFrame:
		p.mu.Lock()
		p.msgs = append(p.msgs, map[string]any{"role": "user", "content": t.Text})
		msgs := make([]map[string]any, len(p.msgs))
		copy(msgs, p.msgs)
		p.mu.Unlock()
		return p.runLLM(ctx, msgs)
	case *frames.LLMRunFrame:
		p.mu.Lock()
		msgs := make([]map[string]any, len(p.msgs))
		copy(msgs, p.msgs)
		p.mu.Unlock()
		return p.runLLM(ctx, msgs)
	default:
		return p.PushDownstream(ctx, f)
	}
}

func (p *LLMProcessor) runLLM(ctx context.Context, messages []map[string]any) error {
	var fullContent string
	err := p.LLM.Chat(ctx, messages, func(tf *frames.LLMTextFrame) {
		fullContent += tf.Text
		_ = p.PushDownstream(ctx, tf)
	})
	if err != nil {
		return err
	}
	if fullContent != "" {
		p.mu.Lock()
		p.msgs = append(p.msgs, map[string]any{"role": "assistant", "content": fullContent})
		p.mu.Unlock()
	}
	return nil
}
