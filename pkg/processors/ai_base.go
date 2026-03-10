// Package processors: AIServiceBase provides a base for AI services with settings,
// Start/Stop/Cancel lifecycle, and optional metrics sync (mirrors upstream ai_service.py).
package processors

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
)

// ServiceSettings holds runtime settings for an AI service (model, voice, etc.).
type ServiceSettings struct {
	Model string `json:"model,omitempty"`
	Voice string `json:"voice,omitempty"`
}

// AIServiceBase embeds BaseProcessor and adds settings plus Start/Stop/Cancel lifecycle.
// ProcessFrame handles StartFrame, EndFrame, and CancelFrame by calling Start, Stop, Cancel
// then forwarding. Subtypes override Start, Stop, Cancel for initialization/cleanup.
type AIServiceBase struct {
	*BaseProcessor
	settings *ServiceSettings
	mu       sync.Mutex
}

// NewAIServiceBase returns a base with the given name and optional initial settings.
// If settings is nil, a zero-value ServiceSettings is used.
func NewAIServiceBase(name string, settings *ServiceSettings) *AIServiceBase {
	if settings == nil {
		settings = &ServiceSettings{}
	}
	return &AIServiceBase{
		BaseProcessor: NewBaseProcessor(name),
		settings:      settings,
	}
}

// Settings returns a copy of the current settings (caller may modify the copy).
func (b *AIServiceBase) Settings() ServiceSettings {
	b.mu.Lock()
	defer b.mu.Unlock()
	return *b.settings
}

// ApplySettings updates the service settings. Subtypes may override to react to changes.
func (b *AIServiceBase) ApplySettings(s ServiceSettings) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settings = &s
}

// Start is called when a StartFrame is processed. Override for service-specific initialization.
// Implementations may use ctx and frame; the base ignores them.
func (b *AIServiceBase) Start(ctx context.Context, frame *frames.StartFrame) {
	_ = ctx
	_ = frame
}

// Stop is called when an EndFrame is processed. Override for cleanup.
// Implementations may use ctx and frame; the base ignores them.
func (b *AIServiceBase) Stop(ctx context.Context, frame *frames.EndFrame) {
	_ = ctx
	_ = frame
}

// Cancel is called when a CancelFrame is processed. Override for cancellation logic.
// Implementations may use ctx and frame; the base ignores them.
func (b *AIServiceBase) Cancel(ctx context.Context, frame *frames.CancelFrame) {
	_ = ctx
	_ = frame
}

// ProcessFrame handles Start/End/Cancel by calling Start, Stop, Cancel (with recovery) then forwards the frame.
func (b *AIServiceBase) ProcessFrame(ctx context.Context, f frames.Frame, dir Direction) error {
	switch frame := f.(type) {
	case *frames.StartFrame:
		b.start(ctx, frame)
	case *frames.EndFrame:
		b.stop(ctx, frame)
	case *frames.CancelFrame:
		b.cancel(ctx, frame)
	}
	// Forward
	if dir == Downstream && b.Next() != nil {
		return b.Next().ProcessFrame(ctx, f, dir)
	}
	if dir == Upstream && b.Prev() != nil {
		return b.Prev().ProcessFrame(ctx, f, dir)
	}
	return nil
}

// start processes StartFrame and catches panics (mirrors Python _start).
func (b *AIServiceBase) start(ctx context.Context, frame *frames.StartFrame) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("%s: exception processing StartFrame: %v", b.Name(), r)
		}
	}()
	b.Start(ctx, frame)
}

// stop processes EndFrame and catches panics.
func (b *AIServiceBase) stop(ctx context.Context, frame *frames.EndFrame) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("%s: exception processing EndFrame: %v", b.Name(), r)
		}
	}()
	b.Stop(ctx, frame)
}

// cancel processes CancelFrame and catches panics.
func (b *AIServiceBase) cancel(ctx context.Context, frame *frames.CancelFrame) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("%s: exception processing CancelFrame: %v", b.Name(), r)
		}
	}()
	b.Cancel(ctx, frame)
}

