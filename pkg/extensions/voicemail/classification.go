// Package voicemail provides ClassificationProcessor for voicemail vs conversation detection.
package voicemail

import (
	"context"
	"strings"
	"sync"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/sync/notifier"
)

// ClassificationProcessor aggregates LLM classification responses and triggers
// gate notifiers and event handlers for CONVERSATION vs VOICEMAIL.
type ClassificationProcessor struct {
	*processors.BaseProcessor
	gateNotifier         *notifier.Notifier
	conversationNotifier *notifier.Notifier
	voicemailNotifier    *notifier.Notifier
	voicemailDelaySecs   float64

	processingResponse bool
	responseBuffer     strings.Builder
	decisionMade      bool
	voicemailDetected  bool
	mu                 sync.Mutex

	// voicemailDetectedNotify signals delayedVoicemailHandler when voicemail is detected (avoids polling sleep).
	voicemailDetectedNotify chan struct{}
	// voicemailEvent: when set (user started speaking) the delay timer resets; when clear we run the delay.
	voicemailEvent   chan struct{}
	voicemailEventMu sync.Mutex
	delayCancel      context.CancelFunc
	onConversation  func()
	onVoicemail     func()
}

// NewClassificationProcessor creates a processor that classifies CONVERSATION vs VOICEMAIL from LLM output.
func NewClassificationProcessor(name string, gateNotifier, conversationNotifier, voicemailNotifier *notifier.Notifier, voicemailResponseDelaySecs float64) *ClassificationProcessor {
	if name == "" {
		name = "ClassificationProcessor"
	}
	if voicemailResponseDelaySecs <= 0 {
		voicemailResponseDelaySecs = 2.0
	}
	return &ClassificationProcessor{
		BaseProcessor:           processors.NewBaseProcessor(name),
		gateNotifier:            gateNotifier,
		conversationNotifier:    conversationNotifier,
		voicemailNotifier:       voicemailNotifier,
		voicemailDelaySecs:     voicemailResponseDelaySecs,
		voicemailDetectedNotify: make(chan struct{}, 1),
		voicemailEvent:          make(chan struct{}),
	}
}

// OnConversationDetected sets the callback when CONVERSATION is classified.
func (p *ClassificationProcessor) OnConversationDetected(fn func()) {
	p.onConversation = fn
}

// OnVoicemailDetected sets the callback when VOICEMAIL is classified (after delay).
func (p *ClassificationProcessor) OnVoicemailDetected(fn func()) {
	p.onVoicemail = fn
}

func (p *ClassificationProcessor) setVoicemailEvent() {
	p.voicemailEventMu.Lock()
	defer p.voicemailEventMu.Unlock()
	select {
	case p.voicemailEvent <- struct{}{}:
	default:
	}
}

func (p *ClassificationProcessor) clearVoicemailEvent() {
	p.voicemailEventMu.Lock()
	old := p.voicemailEvent
	p.voicemailEvent = make(chan struct{})
	p.voicemailEventMu.Unlock()
	close(old) // unblock goroutine waiting on old channel so it can re-read the new one
}

// Setup starts the delayed voicemail handler goroutine.
func (p *ClassificationProcessor) Setup(ctx context.Context) error {
	ctxDelay, cancel := context.WithCancel(ctx)
	p.delayCancel = cancel
	go p.delayedVoicemailHandler(ctxDelay)
	return p.BaseProcessor.Setup(ctx)
}

// Cleanup cancels the delay goroutine.
func (p *ClassificationProcessor) Cleanup(ctx context.Context) error {
	if p.delayCancel != nil {
		p.delayCancel()
		p.delayCancel = nil
	}
	return p.BaseProcessor.Cleanup(ctx)
}

func (p *ClassificationProcessor) delayedVoicemailHandler(ctx context.Context) {
	delay := time.Duration(p.voicemailDelaySecs * float64(time.Second))
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		p.mu.Lock()
		vd := p.voicemailDetected
		p.mu.Unlock()
		if !vd {
			// CONCURRENCY: wait for voicemail detection or 100ms instead of polling with Sleep.
			select {
			case <-ctx.Done():
				return
			case <-p.voicemailDetectedNotify:
				// voicemailDetected was set, fall through
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}
		p.voicemailEventMu.Lock()
		ch := p.voicemailEvent
		p.voicemailEventMu.Unlock()
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				// Channel was closed (clearVoicemailEvent); next iteration will get the new channel
			}
			continue
		case <-time.After(delay):
			if p.onVoicemail != nil {
				p.onVoicemail()
			}
			return
		}
	}
}

// ProcessFrame aggregates LLM text and classifies on LLMFullResponseEndFrame.
func (p *ClassificationProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.LLMFullResponseStartFrame:
		p.mu.Lock()
		p.processingResponse = true
		p.responseBuffer.Reset()
		p.mu.Unlock()
		return p.PushDownstream(ctx, f)

	case *frames.LLMFullResponseEndFrame:
		p.mu.Lock()
		if p.processingResponse && !p.decisionMade {
			fullResponse := strings.TrimSpace(p.responseBuffer.String())
			p.processingResponse = false
			p.responseBuffer.Reset()
			p.mu.Unlock()
			p.processClassification(ctx, fullResponse)
		} else {
			p.processingResponse = false
			p.responseBuffer.Reset()
			p.mu.Unlock()
		}
		return p.PushDownstream(ctx, f)

	case *frames.LLMTextFrame:
		p.mu.Lock()
		if p.processingResponse {
			p.responseBuffer.WriteString(t.Text)
		}
		p.mu.Unlock()
		return p.PushDownstream(ctx, f)

	case *frames.UserStartedSpeakingFrame:
		if p.voicemailDetected {
			p.setVoicemailEvent()
		}
		return p.PushDownstream(ctx, f)

	case *frames.UserStoppedSpeakingFrame:
		if p.voicemailDetected {
			p.clearVoicemailEvent()
		}
		return p.PushDownstream(ctx, f)

	default:
		return p.PushDownstream(ctx, f)
	}
}

func (p *ClassificationProcessor) processClassification(ctx context.Context, fullResponse string) {
	p.mu.Lock()
	if p.decisionMade {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	response := strings.ToUpper(fullResponse)
	logger.Info("voicemail classification: response %q", fullResponse)

	if strings.Contains(response, "CONVERSATION") {
		p.mu.Lock()
		p.decisionMade = true
		p.mu.Unlock()
		logger.Info("voicemail: CONVERSATION detected")
		p.gateNotifier.Notify()
		p.conversationNotifier.Notify()
		if p.onConversation != nil {
			p.onConversation()
		}
		return
	}

	if strings.Contains(response, "VOICEMAIL") {
		p.mu.Lock()
		p.decisionMade = true
		p.voicemailDetected = true
		p.mu.Unlock()
		select {
		case p.voicemailDetectedNotify <- struct{}{}:
		default:
		}
		logger.Info("voicemail: VOICEMAIL detected")
		p.gateNotifier.Notify()
		p.voicemailNotifier.Notify()
		_ = p.PushDownstream(ctx, frames.NewCancelFrame("voicemail detected"))
		p.clearVoicemailEvent()
		return
	}
}

var _ processors.Processor = (*ClassificationProcessor)(nil)
