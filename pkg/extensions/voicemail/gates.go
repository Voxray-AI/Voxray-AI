// Package voicemail provides voicemail detection for outbound calls (human vs voicemail).
package voicemail

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/sync/notifier"
)

// NotifierGate is a base gate that starts open and closes permanently when the notifier signals.
// When closed, only system/end frames are forwarded.
type NotifierGate struct {
	*processors.BaseProcessor
	notifier   *notifier.Notifier
	gateClosed bool
	mu         sync.Mutex
	cancel     context.CancelFunc
}

// NewNotifierGate creates a gate that closes when n signals.
func NewNotifierGate(name string, n *notifier.Notifier) *NotifierGate {
	if name == "" {
		name = "NotifierGate"
	}
	return &NotifierGate{BaseProcessor: processors.NewBaseProcessor(name), notifier: n}
}

// Setup starts a goroutine that waits for the notifier and then closes the gate.
func (g *NotifierGate) Setup(ctx context.Context) error {
	ctxGate, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	go func() {
		_ = g.notifier.Wait(ctxGate)
		g.mu.Lock()
		if ctxGate.Err() == nil {
			g.gateClosed = true
		}
		g.mu.Unlock()
	}()
	return g.BaseProcessor.Setup(ctx)
}

// Cleanup cancels the wait goroutine.
func (g *NotifierGate) Cleanup(ctx context.Context) error {
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	return g.BaseProcessor.Cleanup(ctx)
}

// ProcessFrame forwards all frames when open; when closed, only system/end frames.
func (g *NotifierGate) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if g.Prev() != nil {
			return g.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	g.mu.Lock()
	closed := g.gateClosed
	g.mu.Unlock()
	if !closed {
		return g.PushDownstream(ctx, f)
	}
	switch f.(type) {
	case *frames.StartFrame, *frames.EndFrame, *frames.CancelFrame, *frames.StopFrame:
		return g.PushDownstream(ctx, f)
	}
	return nil
}

var _ processors.Processor = (*NotifierGate)(nil)

// ClassifierGate closes when the gate notifier signals; when closed, still forwards
// UserStartedSpeakingFrame/UserStoppedSpeakingFrame only if conversation was not detected (voicemail case).
type ClassifierGate struct {
	*NotifierGate
	conversationNotifier *notifier.Notifier
	conversationDetected bool
	convMu               sync.Mutex
	convCancel            context.CancelFunc
}

// NewClassifierGate creates a classifier gate. gateNotifier signals when classification is done;
// conversationNotifier signals when conversation (not voicemail) was detected.
func NewClassifierGate(name string, gateNotifier, conversationNotifier *notifier.Notifier) *ClassifierGate {
	if name == "" {
		name = "ClassifierGate"
	}
	return &ClassifierGate{
		NotifierGate:         NewNotifierGate(name, gateNotifier),
		conversationNotifier: conversationNotifier,
	}
}

// Setup starts the parent gate and a goroutine to wait for conversation detection.
func (g *ClassifierGate) Setup(ctx context.Context) error {
	if err := g.NotifierGate.Setup(ctx); err != nil {
		return err
	}
	ctxConv, cancel := context.WithCancel(ctx)
	g.convCancel = cancel
	go func() {
		_ = g.conversationNotifier.Wait(ctxConv)
		g.convMu.Lock()
		if ctxConv.Err() == nil {
			g.conversationDetected = true
		}
		g.convMu.Unlock()
	}()
	return nil
}

// Cleanup cancels the conversation wait.
func (g *ClassifierGate) Cleanup(ctx context.Context) error {
	if g.convCancel != nil {
		g.convCancel()
		g.convCancel = nil
	}
	return g.NotifierGate.Cleanup(ctx)
}

// ProcessFrame when closed allows UserStarted/StoppedSpeaking only if conversation not detected.
func (g *ClassifierGate) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if g.Prev() != nil {
			return g.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	g.mu.Lock()
	closed := g.gateClosed
	g.mu.Unlock()
	if !closed {
		return g.PushDownstream(ctx, f)
	}
	switch f.(type) {
	case *frames.UserStartedSpeakingFrame, *frames.UserStoppedSpeakingFrame:
		g.convMu.Lock()
		conv := g.conversationDetected
		g.convMu.Unlock()
		if !conv {
			return g.PushDownstream(ctx, f)
		}
		return nil
	case *frames.StartFrame, *frames.EndFrame, *frames.CancelFrame, *frames.StopFrame:
		return g.PushDownstream(ctx, f)
	}
	return nil
}

// ConversationGate closes when voicemail is detected (blocks conversation flow).
type ConversationGate struct {
	*NotifierGate
}

// NewConversationGate creates a gate that closes when voicemailNotifier signals.
func NewConversationGate(name string, voicemailNotifier *notifier.Notifier) *ConversationGate {
	if name == "" {
		name = "ConversationGate"
	}
	return &ConversationGate{NotifierGate: NewNotifierGate(name, voicemailNotifier)}
}

// TTSGate buffers TTS-related frames until conversation or voicemail notifier signals.
// On conversation: release buffered frames. On voicemail: clear buffer.
type TTSGate struct {
	*processors.BaseProcessor
	conversationNotifier *notifier.Notifier
	voicemailNotifier    *notifier.Notifier
	buf                  []frames.Frame
	mu                   sync.Mutex
	gatingActive         bool
	convCancel           context.CancelFunc
	vmCancel             context.CancelFunc
}

// NewTTSGate creates a TTS gate. Conversation notifier releases frames; voicemail notifier clears them.
func NewTTSGate(name string, conversationNotifier, voicemailNotifier *notifier.Notifier) *TTSGate {
	if name == "" {
		name = "TTSGate"
	}
	return &TTSGate{
		BaseProcessor:        processors.NewBaseProcessor(name),
		conversationNotifier: conversationNotifier,
		voicemailNotifier:    voicemailNotifier,
		gatingActive:         true,
	}
}

// Setup starts goroutines waiting for conversation or voicemail.
func (g *TTSGate) Setup(ctx context.Context) error {
	ctxConv, cancelConv := context.WithCancel(ctx)
	g.convCancel = cancelConv
	ctxVM, cancelVM := context.WithCancel(ctx)
	g.vmCancel = cancelVM
	go func() {
		_ = g.conversationNotifier.Wait(ctxConv)
		if ctxConv.Err() != nil {
			return
		}
		g.mu.Lock()
		g.gatingActive = false
		for _, f := range g.buf {
			_ = g.PushDownstream(ctx, f)
		}
		g.buf = nil
		g.mu.Unlock()
	}()
	go func() {
		_ = g.voicemailNotifier.Wait(ctxVM)
		if ctxVM.Err() != nil {
			return
		}
		g.mu.Lock()
		g.gatingActive = false
		g.buf = nil
		g.mu.Unlock()
	}()
	return g.BaseProcessor.Setup(ctx)
}

// Cleanup cancels the wait goroutines.
func (g *TTSGate) Cleanup(ctx context.Context) error {
	if g.convCancel != nil {
		g.convCancel()
		g.convCancel = nil
	}
	if g.vmCancel != nil {
		g.vmCancel()
		g.vmCancel = nil
	}
	return g.BaseProcessor.Cleanup(ctx)
}

func isTTSFrame(f frames.Frame) bool {
	switch f.(type) {
	case *frames.TTSAudioRawFrame, *frames.BotStartedSpeakingFrame, *frames.BotStoppedSpeakingFrame:
		return true
	}
	return false
}

// ProcessFrame gates TTS frames; releases or clears on notifier signal.
func (g *TTSGate) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if g.Prev() != nil {
			return g.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	g.mu.Lock()
	active := g.gatingActive
	g.mu.Unlock()
	if active && isTTSFrame(f) {
		g.mu.Lock()
		g.buf = append(g.buf, f)
		g.mu.Unlock()
		return nil
	}
	return g.PushDownstream(ctx, f)
}

var _ processors.Processor = (*TTSGate)(nil)
