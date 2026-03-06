package voice

import (
	"context"
	"sync"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/metrics"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/services"
)

// OnContextUpdate is called whenever the LLM context (msgs) is updated. Used by IVR to capture conversation for mode switching.
type OnContextUpdate func(msgs []map[string]any)

// LLMProcessor runs the LLM on transcription/context and streams LLMTextFrame downstream.
type LLMProcessor struct {
	*processors.BaseProcessor
	LLM            services.LLMService
	SystemPrompt   string // optional; when set, sent as system message so the LLM replies as assistant
	OnContextUpdate OnContextUpdate // optional; called when msgs is updated (e.g. for IVR SetSavedMessages)
	mu             sync.Mutex
	msgs           []map[string]any
}

// NewLLMProcessor returns a processor that runs the LLM and streams text downstream.
func NewLLMProcessor(name string, llm services.LLMService) *LLMProcessor {
	return NewLLMProcessorWithSystemPrompt(name, llm, "")
}

// NewLLMProcessorWithSystemPrompt returns a processor that runs the LLM with an optional system prompt (e.g. "You are a helpful voice assistant. Reply briefly.").
func NewLLMProcessorWithSystemPrompt(name string, llm services.LLMService, systemPrompt string) *LLMProcessor {
	if name == "" {
		name = "LLM"
	}
	return &LLMProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		LLM:           llm,
		SystemPrompt:  systemPrompt,
		msgs:          make([]map[string]any, 0),
	}
}

// ProcessFrame runs LLM on TranscriptionFrame (appends user message), LLMRunFrame, or LLMMessagesUpdateFrame; streams LLMTextFrame downstream. Forwards other frames.
func (p *LLMProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	// LLMMessagesUpdateFrame can arrive upstream (e.g. from IVR) or downstream; replace context and optionally run LLM.
	if uf, ok := f.(*frames.LLMMessagesUpdateFrame); ok {
		p.mu.Lock()
		p.msgs = make([]map[string]any, len(uf.Messages))
		copy(p.msgs, uf.Messages)
		p.notifyContextUpdateLocked()
		runLLM := uf.RunLLM != nil && *uf.RunLLM
		p.mu.Unlock()
		if runLLM {
			p.mu.Lock()
			msgs := make([]map[string]any, len(p.msgs))
			copy(msgs, p.msgs)
			p.mu.Unlock()
			return p.runLLMWithMessages(ctx, msgs, true) // skip prepending SystemPrompt; frame already has system message
		}
		// Forward downstream so other processors (e.g. IVR) see the update.
		return p.PushDownstream(ctx, f)
	}

	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.TranscriptionFrame:
		preview := t.Text
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		logger.Info("LLM: transcript received from STT: %d chars, preview=%q", len(t.Text), preview)
		p.mu.Lock()
		p.msgs = append(p.msgs, map[string]any{"role": "user", "content": t.Text})
		p.notifyContextUpdateLocked()
		msgs := make([]map[string]any, len(p.msgs))
		copy(msgs, p.msgs)
		p.mu.Unlock()
		return p.runLLMWithMessages(ctx, msgs, false)
	case *frames.LLMRunFrame:
		p.mu.Lock()
		msgs := make([]map[string]any, len(p.msgs))
		copy(msgs, p.msgs)
		p.mu.Unlock()
		return p.runLLMWithMessages(ctx, msgs, false)
	default:
		return p.PushDownstream(ctx, f)
	}
}

// runLLMWithMessages runs the LLM on the given messages. If skipSystemPrompt is true, messages are sent as-is (e.g. from LLMMessagesUpdateFrame).
func (p *LLMProcessor) runLLMWithMessages(ctx context.Context, messages []map[string]any, skipSystemPrompt bool) error {
	msgsToSend := make([]map[string]any, 0, len(messages)+1)
	if !skipSystemPrompt && p.SystemPrompt != "" {
		msgsToSend = append(msgsToSend, map[string]any{"role": "system", "content": p.SystemPrompt})
	}
	msgsToSend = append(msgsToSend, messages...)

	start := time.Now()
	var fullContent string
	var firstTokenAt time.Time
	var lastTokenAt time.Time
	err := p.LLM.Chat(ctx, msgsToSend, func(tf *frames.LLMTextFrame) {
		now := time.Now()
		if firstTokenAt.IsZero() {
			firstTokenAt = now
			metrics.LLMTimeToFirstTokenSeconds.WithLabelValues("", "llm", "success", "").Observe(now.Sub(start).Seconds())
		} else {
			metrics.LLMInterTokenLatencySeconds.WithLabelValues("", "llm", "").Observe(now.Sub(lastTokenAt).Seconds())
		}
		lastTokenAt = now
		fullContent += tf.Text
		_ = p.PushDownstream(ctx, tf)
	})
	if err != nil {
		metrics.LLMErrorsTotal.WithLabelValues("provider_error", "", "llm", "").Inc()
		now := time.Since(start).Seconds()
		metrics.LLMTimeToFirstTokenSeconds.WithLabelValues("", "llm", "error", "").Observe(now)
		metrics.LLMGenerationLatencySeconds.WithLabelValues("", "llm", "error", "").Observe(now)
		return err
	}
	metrics.LLMGenerationLatencySeconds.WithLabelValues("", "llm", "success", "").Observe(time.Since(start).Seconds())
	if fullContent != "" {
		preview := fullContent
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		logger.Info("LLM: response complete, sending to TTS: %d chars, preview=%q", len(fullContent), preview)
		p.mu.Lock()
		p.msgs = append(p.msgs, map[string]any{"role": "assistant", "content": fullContent})
		p.notifyContextUpdateLocked()
		p.mu.Unlock()
		// Signal TTS to flush any buffered text (sentence batching)
		_ = p.PushDownstream(ctx, frames.NewTTSSpeakFrame(""))
	}
	return nil
}

// notifyContextUpdateLocked must be called with p.mu held.
func (p *LLMProcessor) notifyContextUpdateLocked() {
	if p.OnContextUpdate == nil {
		return
	}
	msgs := make([]map[string]any, len(p.msgs))
	copy(msgs, p.msgs)
	go p.OnContextUpdate(msgs) // avoid holding lock in callback
}
