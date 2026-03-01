package llmcontextsummarizer_test

import (
	"context"
	"sync"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/aggregators/llmcontextsummarizer"
)

// collectProcessor records all frames received for assertions.
type collectProcessor struct {
	mu     sync.Mutex
	frames []frames.Frame
}

func (c *collectProcessor) ProcessFrame(_ context.Context, f frames.Frame, _ processors.Direction) error {
	c.mu.Lock()
	c.frames = append(c.frames, f)
	c.mu.Unlock()
	return nil
}
func (c *collectProcessor) SetNext(processors.Processor)   {}
func (c *collectProcessor) SetPrev(processors.Processor)   {}
func (c *collectProcessor) Setup(context.Context) error   { return nil }
func (c *collectProcessor) Cleanup(context.Context) error { return nil }
func (c *collectProcessor) Name() string                     { return "collect" }

func (c *collectProcessor) got() []frames.Frame {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]frames.Frame(nil), c.frames...)
}

// TestNew_DefaultConfig verifies that zero/minimal config uses DefaultConfig (e.g. AutoTrigger true).
func TestNew_DefaultConfig(t *testing.T) {
	proc := llmcontextsummarizer.New("sum", nil, llmcontextsummarizer.Config{})
	if proc == nil {
		t.Fatal("New returned nil")
	}
	cfg := proc.Config
	if !cfg.AutoTrigger {
		t.Error("zero Config should get DefaultConfig with AutoTrigger true")
	}
	if cfg.MaxUnsummarizedMessages != 20 {
		t.Errorf("DefaultConfig MaxUnsummarizedMessages: got %d", cfg.MaxUnsummarizedMessages)
	}
}

// TestProcessor_AutoTriggerOff_NoRequest verifies that when AutoTrigger is false, no LLMContextSummaryRequestFrame is sent.
func TestProcessor_AutoTriggerOff_NoRequest(t *testing.T) {
	ctx := context.Background()
	llmCtx := &frames.LLMContext{}
	for i := 0; i < 25; i++ {
		llmCtx.Messages = append(llmCtx.Messages, map[string]any{"role": "user", "content": "message"})
	}
	proc := llmcontextsummarizer.New("sum", llmCtx, llmcontextsummarizer.Config{
		MaxUnsummarizedMessages: 20,
		AutoTrigger:             false,
	})
	collector := &collectProcessor{}
	proc.SetNext(collector)
	proc.Setup(ctx)
	defer proc.Cleanup(ctx)

	_ = proc.ProcessFrame(ctx, &frames.LLMFullResponseStartFrame{}, processors.Downstream)
	got := collector.got()
	for _, f := range got {
		if _, ok := f.(*frames.LLMContextSummaryRequestFrame); ok {
			t.Error("AutoTrigger off: should not send LLMContextSummaryRequestFrame")
			break
		}
	}
}

// TestProcessor_OverThreshold_EmitsRequest mirrors upstream: on LLMFullResponseStartFrame when over threshold, downstream receives LLMContextSummaryRequestFrame.
func TestProcessor_OverThreshold_EmitsRequest(t *testing.T) {
	ctx := context.Background()
	llmCtx := &frames.LLMContext{}
	for i := 0; i < 25; i++ {
		llmCtx.Messages = append(llmCtx.Messages, map[string]any{"role": "user", "content": "message"})
	}
	proc := llmcontextsummarizer.New("sum", llmCtx, llmcontextsummarizer.Config{
		MaxUnsummarizedMessages: 20,
		MinMessagesToKeep:      5,
		AutoTrigger:            true,
	})
	collector := &collectProcessor{}
	proc.SetNext(collector)
	proc.Setup(ctx)
	defer proc.Cleanup(ctx)

	_ = proc.ProcessFrame(ctx, &frames.LLMFullResponseStartFrame{}, processors.Downstream)
	got := collector.got()
	var foundReq bool
	for _, f := range got {
		if _, ok := f.(*frames.LLMContextSummaryRequestFrame); ok {
			foundReq = true
			break
		}
	}
	if !foundReq {
		t.Error("over threshold with AutoTrigger: expected LLMContextSummaryRequestFrame downstream")
	}
}

// TestProcessor_SummaryResult_UpdatesContext verifies that LLMContextSummaryResultFrame with matching RequestID updates context.
func TestProcessor_SummaryResult_UpdatesContext(t *testing.T) {
	ctx := context.Background()
	llmCtx := &frames.LLMContext{}
	for i := 0; i < 25; i++ {
		llmCtx.Messages = append(llmCtx.Messages, map[string]any{"role": "user", "content": "msg"})
	}
	proc := llmcontextsummarizer.New("sum", llmCtx, llmcontextsummarizer.Config{
		MaxUnsummarizedMessages: 20,
		MinMessagesToKeep:       2,
		AutoTrigger:             true,
	})
	collector := &collectProcessor{}
	proc.SetNext(collector)
	proc.Setup(ctx)
	defer proc.Cleanup(ctx)

	_ = proc.ProcessFrame(ctx, &frames.LLMFullResponseStartFrame{}, processors.Downstream)
	got := collector.got()
	var reqID string
	for _, f := range got {
		if req, ok := f.(*frames.LLMContextSummaryRequestFrame); ok {
			reqID = req.RequestID
			break
		}
	}
	if reqID == "" {
		t.Fatal("expected one LLMContextSummaryRequestFrame")
	}

	// Apply result: summarize up to index 22, keep last 2 (indices 23,24). Context becomes [summary, msg, msg].
	result := &frames.LLMContextSummaryResultFrame{
		RequestID:            reqID,
		Summary:              "Summarized.",
		LastSummarizedIndex:   22,
	}
	_ = proc.ProcessFrame(ctx, result, processors.Downstream)
	if len(llmCtx.Messages) < 1 {
		t.Errorf("context should be updated with summary; messages len = %d", len(llmCtx.Messages))
	}
	// Context should contain the summary message
	foundSummary := false
	for _, m := range llmCtx.Messages {
		if c, ok := m["content"].(string); ok && len(c) > 0 && (c == "Conversation summary: Summarized." || len(c) > 20) {
			foundSummary = true
			break
		}
	}
	if !foundSummary {
		t.Error("context messages should contain conversation summary after applying result")
	}
}
