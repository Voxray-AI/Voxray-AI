// Package llmcontextsummarizer provides a processor that monitors LLM context size and
// emits LLMContextSummaryRequestFrame when thresholds are exceeded; applies results from LLMContextSummaryResultFrame.
package llmcontextsummarizer

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

// Config holds summarizer thresholds and options.
type Config struct {
	MaxContextTokens        int
	MaxUnsummarizedMessages int
	MinMessagesToKeep       int
	TargetContextTokens     int
	SummarizationPrompt     string
	SummarizationTimeoutSec int
	AutoTrigger             bool
}

// DefaultConfig returns default config values.
func DefaultConfig() Config {
	return Config{
		MaxContextTokens:        12000,
		MaxUnsummarizedMessages: 20,
		MinMessagesToKeep:       5,
		TargetContextTokens:     2000,
		SummarizationPrompt:     "Summarize the following conversation concisely, preserving key facts and decisions.",
		SummarizationTimeoutSec: 60,
		AutoTrigger:             true,
	}
}

// Processor monitors context and pushes summary requests; applies summary results.
type Processor struct {
	*processors.BaseProcessor
	Context *frames.LLMContext
	Config  Config

	mu                       sync.Mutex
	pendingRequestID         string
	summarizationInProgress  bool
}

// New returns a context summarizer. If cfg is zero value, DefaultConfig is used.
func New(name string, ctx *frames.LLMContext, cfg Config) *Processor {
	if name == "" {
		name = "LLMContextSummarizer"
	}
	if ctx == nil {
		ctx = &frames.LLMContext{}
	}
	if cfg.MaxContextTokens == 0 && cfg.MaxUnsummarizedMessages == 0 {
		cfg = DefaultConfig()
	}
	return &Processor{
		BaseProcessor: processors.NewBaseProcessor(name),
		Context:       ctx,
		Config:        cfg,
	}
}

// ProcessFrame checks thresholds on LLMFullResponseStartFrame; applies result on LLMContextSummaryResultFrame.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.LLMFullResponseStartFrame:
		if p.Config.AutoTrigger && p.shouldSummarize() {
			p.requestSummarization(ctx)
		}
		return p.PushDownstream(ctx, f)

	case *frames.LLMContextSummaryResultFrame:
		p.handleSummaryResult(t)
		return p.PushDownstream(ctx, f)

	case *frames.InterruptionFrame:
		p.mu.Lock()
		p.summarizationInProgress = false
		p.pendingRequestID = ""
		p.mu.Unlock()
		return p.PushDownstream(ctx, f)

	default:
		return p.PushDownstream(ctx, f)
	}
}

func (p *Processor) shouldSummarize() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.summarizationInProgress {
		return false
	}
	tokens := estimateContextTokens(p.Context)
	msgCount := len(p.Context.Messages)
	if p.Config.MaxContextTokens > 0 && tokens >= p.Config.MaxContextTokens {
		return true
	}
	if p.Config.MaxUnsummarizedMessages > 0 && msgCount >= p.Config.MaxUnsummarizedMessages {
		return true
	}
	return false
}

func (p *Processor) requestSummarization(ctx context.Context) {
	p.mu.Lock()
	if p.summarizationInProgress {
		p.mu.Unlock()
		return
	}
	requestID := uuid.New().String()
	p.pendingRequestID = requestID
	p.summarizationInProgress = true
	p.mu.Unlock()

	timeout := p.Config.SummarizationTimeoutSec
	if timeout <= 0 {
		timeout = 60
	}
	req := &frames.LLMContextSummaryRequestFrame{
		DataFrame:              frames.DataFrame{Base: frames.NewBase()},
		RequestID:              requestID,
		Context:                p.Context,
		MinMessagesToKeep:      p.Config.MinMessagesToKeep,
		TargetContextTokens:    p.Config.TargetContextTokens,
		SummarizationPrompt:    p.Config.SummarizationPrompt,
		SummarizationTimeout:   timeout,
	}
	_ = p.PushDownstream(ctx, req)
}

func (p *Processor) handleSummaryResult(t *frames.LLMContextSummaryResultFrame) {
	p.mu.Lock()
	if t.RequestID != p.pendingRequestID {
		p.mu.Unlock()
		return
	}
	p.summarizationInProgress = false
	p.pendingRequestID = ""
	p.mu.Unlock()

	if t.Error != "" || t.LastSummarizedIndex < 0 {
		return
	}
	applySummary(p.Context, t.Summary, t.LastSummarizedIndex, p.Config.MinMessagesToKeep)
}

// estimateContextTokens returns a rough token count for the context (chars/4).
func estimateContextTokens(c *frames.LLMContext) int {
	n := 0
	for _, m := range c.Messages {
		if content, ok := m["content"]; ok {
			switch v := content.(type) {
			case string:
				n += len(v)
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						if t, ok := m["text"].(string); ok {
							n += len(t)
						}
					}
				}
			}
		}
	}
	return n / 4
}

// applySummary replaces messages [0..lastSummarizedIndex] with a single user message containing the summary.
func applySummary(c *frames.LLMContext, summary string, lastSummarizedIndex, minKeep int) {
	if lastSummarizedIndex < 0 || lastSummarizedIndex >= len(c.Messages) {
		return
	}
	keep := len(c.Messages) - 1 - lastSummarizedIndex
	if keep < minKeep {
		return
	}
	// Preserve first system message if any
	var systemMsg map[string]any
	for _, m := range c.Messages {
		if r, _ := m["role"].(string); r == "system" {
			systemMsg = m
			break
		}
	}
	newMsgs := make([]map[string]any, 0, 4)
	if systemMsg != nil {
		newMsgs = append(newMsgs, systemMsg)
	}
	newMsgs = append(newMsgs, map[string]any{"role": "user", "content": "Conversation summary: " + summary})
	newMsgs = append(newMsgs, c.Messages[lastSummarizedIndex+1:]...)
	c.Messages = newMsgs
}
