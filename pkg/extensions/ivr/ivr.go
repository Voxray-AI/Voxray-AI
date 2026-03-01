// Package ivr provides Interactive Voice Response (IVR) navigation components for
// automated IVR phone system navigation using LLM-based decision making and DTMF.
package ivr

import (
	"context"
	"strings"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/utils/patternaggregator"
)

// IVRStatus represents the current state of IVR navigation.
type IVRStatus string

const (
	IVRStatusDetected  IVRStatus = "detected"
	IVRStatusCompleted IVRStatus = "completed"
	IVRStatusStuck     IVRStatus = "stuck"
	IVRStatusWait      IVRStatus = "wait"
)

// Default delimiters for XML-style tags in LLM output (backtick-space).
const defaultOpenDelim  = " ` "
const defaultCloseDelim = " ` "

// IVRProcessor processes LLM responses for IVR navigation: aggregates XML-tagged
// commands and executes DTMF, mode switching, and status updates.
type IVRProcessor struct {
	*processors.BaseProcessor
	ClassifierPrompt string
	IVRPrompt        string
	IVRVADStopSecs   float64 // VAD stop_secs when in IVR mode (e.g. 2.0)

	agg          *patternaggregator.Aggregator
	savedMessages []map[string]any
	mu            sync.Mutex

	onConversationDetected func(conversationHistory []map[string]any)
	onIVRStatusChanged     func(status IVRStatus)
}

// NewIVRProcessor creates an IVR processor with the given classifier and IVR prompts.
func NewIVRProcessor(name string, classifierPrompt, ivrPrompt string, ivrVADStopSecs float64) *IVRProcessor {
	if name == "" {
		name = "IVRProcessor"
	}
	if ivrVADStopSecs <= 0 {
		ivrVADStopSecs = 2.0
	}
	return &IVRProcessor{
		BaseProcessor:    processors.NewBaseProcessor(name),
		ClassifierPrompt: classifierPrompt,
		IVRPrompt:        ivrPrompt,
		IVRVADStopSecs:   ivrVADStopSecs,
		agg:              patternaggregator.New(defaultOpenDelim, defaultCloseDelim),
		savedMessages:    make([]map[string]any, 0),
	}
}

// SetSavedMessages sets the conversation context used when switching to IVR mode.
func (p *IVRProcessor) SetSavedMessages(msgs []map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.savedMessages = make([]map[string]any, len(msgs))
	copy(p.savedMessages, msgs)
}

// OnConversationDetected registers a callback when the classifier detects human conversation.
func (p *IVRProcessor) OnConversationDetected(fn func(conversationHistory []map[string]any)) {
	p.onConversationDetected = fn
}

// OnIVRStatusChanged registers a callback when IVR status changes (detected, completed, stuck, wait).
func (p *IVRProcessor) OnIVRStatusChanged(fn func(status IVRStatus)) {
	p.onIVRStatusChanged = fn
}

func (p *IVRProcessor) getConversationHistory() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Exclude first system message like Python _get_conversation_history.
	if len(p.savedMessages) <= 1 {
		return nil
	}
	out := make([]map[string]any, len(p.savedMessages)-1)
	copy(out, p.savedMessages[1:])
	return out
}

// ProcessFrame implements Processor.
func (p *IVRProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.StartFrame:
		_ = p.PushDownstream(ctx, f)
		// Set classifier prompt and run LLM upstream.
		msgs := []map[string]any{{"role": "system", "content": p.ClassifierPrompt}}
		runLLM := true
		update := &frames.LLMMessagesUpdateFrame{
			DataFrame: frames.DataFrame{Base: frames.NewBase()},
			Messages:  msgs,
			RunLLM:    &runLLM,
		}
		return p.PushUpstream(ctx, update)

	case *frames.LLMTextFrame:
		segments, matches := p.agg.Feed(t.Text)
		for _, seg := range segments {
			if seg == "" {
				continue
			}
			out := frames.NewAggregatedTextFrame(seg, "text")
			_ = p.PushDownstream(ctx, out)
		}
		for _, m := range matches {
			if err := p.handleMatch(ctx, strings.TrimSpace(m.Content)); err != nil {
				logger.Info("IVR: handle match %q: %v", m.Content, err)
			}
		}
		return nil

	case *frames.LLMFullResponseEndFrame, *frames.EndFrame:
		remaining := p.agg.Flush()
		if remaining != "" {
			_ = p.PushDownstream(ctx, frames.NewAggregatedTextFrame(remaining, "text"))
		}
		return p.PushDownstream(ctx, f)

	default:
		return p.PushDownstream(ctx, f)
	}
}

func (p *IVRProcessor) handleMatch(ctx context.Context, content string) error {
	// DTMF: single or multi digit 0-9, *, #
	if key, err := frames.ParseKeypadEntry(content); err == nil {
		return p.handleDTMF(ctx, key, content)
	}
	// Mode
	switch content {
	case "conversation":
		return p.handleConversation(ctx)
	case "ivr":
		return p.handleIVRDetected(ctx)
	}
	// IVR status
	switch content {
	case "detected":
		return p.handleIVRStatus(ctx, IVRStatusDetected)
	case "completed":
		return p.handleIVRStatus(ctx, IVRStatusCompleted)
	case "stuck":
		return p.handleIVRStatus(ctx, IVRStatusStuck)
	case "wait":
		return p.handleIVRStatus(ctx, IVRStatusWait)
	}
	return nil
}

func (p *IVRProcessor) handleDTMF(ctx context.Context, key frames.KeypadEntry, content string) error {
	dtmf, err := frames.NewOutputDTMFUrgentFrame(key)
	if err != nil {
		return err
	}
	_ = p.PushDownstream(ctx, dtmf)
	tf := frames.NewTextFrame(" " + content + " ")
	skip := true
	tf.SkipTTS = &skip
	_ = p.PushDownstream(ctx, tf)
	return nil
}

func (p *IVRProcessor) handleConversation(ctx context.Context) error {
	history := p.getConversationHistory()
	if p.onConversationDetected != nil {
		p.onConversationDetected(history)
	}
	return nil
}

func (p *IVRProcessor) handleIVRDetected(ctx context.Context) error {
	msgs := []map[string]any{{"role": "system", "content": p.IVRPrompt}}
	history := p.getConversationHistory()
	msgs = append(msgs, history...)
	runLLM := true
	_ = p.PushUpstream(ctx, &frames.LLMMessagesUpdateFrame{
		DataFrame: frames.DataFrame{Base: frames.NewBase()},
		Messages:  msgs,
		RunLLM:    &runLLM,
	})
	_ = p.PushUpstream(ctx, frames.NewVADParamsUpdateFrame(p.IVRVADStopSecs, 0))
	if p.onIVRStatusChanged != nil {
		p.onIVRStatusChanged(IVRStatusDetected)
	}
	return nil
}

func (p *IVRProcessor) handleIVRStatus(ctx context.Context, status IVRStatus) error {
	tf := frames.NewTextFrame(" " + string(status) + " ")
	skip := true
	tf.SkipTTS = &skip
	_ = p.PushDownstream(ctx, tf)
	if p.onIVRStatusChanged != nil {
		p.onIVRStatusChanged(status)
	}
	return nil
}

var _ processors.Processor = (*IVRProcessor)(nil)
