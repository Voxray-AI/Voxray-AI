package voice

import (
	"context"

	"voxray-go/pkg/audio/interruptions"
	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

// InterruptionController observes bot speech and user transcripts and, when
// allowed, emits InterruptionFrame to clear downstream playback (barge-in).
//
// It relies on a configured interruptions.Strategy to decide when the user
// has spoken “enough” to warrant an interruption.
type InterruptionController struct {
	*processors.BaseProcessor

	// Strategy decides when to interrupt based on accumulated user text.
	Strategy interruptions.Strategy

	allowInterruptions bool
	botSpeaking        bool
	interruptionSent   bool
}

// NewInterruptionController constructs a controller with the provided strategy.
// When strategy is nil, the controller behaves as a pass-through processor.
func NewInterruptionController(name string, strategy interruptions.Strategy) *InterruptionController {
	if name == "" {
		name = "InterruptionController"
	}
	return &InterruptionController{
		BaseProcessor: processors.NewBaseProcessor(name),
		Strategy:      strategy,
	}
}

// ProcessFrame tracks bot/user state and, when appropriate, emits an
// InterruptionFrame downstream. All frames are forwarded unchanged.
func (p *InterruptionController) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.StartFrame:
		p.allowInterruptions = t.AllowInterruptions
		p.botSpeaking = false
		p.interruptionSent = false
		if p.Strategy != nil {
			p.Strategy.Reset()
		}
		// StartFrame must always continue downstream.
		return p.PushDownstream(ctx, f)

	case *frames.BotStartedSpeakingFrame:
		p.botSpeaking = true
		// reset interruption state for this bot turn
		p.interruptionSent = false
		if p.Strategy != nil {
			p.Strategy.Reset()
		}

	case *frames.BotStoppedSpeakingFrame:
		p.botSpeaking = false
		p.interruptionSent = false
		if p.Strategy != nil {
			p.Strategy.Reset()
		}

	case *frames.CancelFrame, *frames.EndFrame, *frames.StopFrame:
		p.botSpeaking = false
		p.interruptionSent = false
		if p.Strategy != nil {
			p.Strategy.Reset()
		}

	case *frames.UserStartedSpeakingFrame:
		// New user utterance; clear any previous accumulated text.
		if p.Strategy != nil {
			p.Strategy.Reset()
		}

	case *frames.TranscriptionFrame:
		// Only consider finalized user transcripts while the bot is speaking,
		// when interruptions are allowed and we have not already interrupted.
		if p.allowInterruptions && p.botSpeaking && !p.interruptionSent && p.Strategy != nil && t.Finalized {
			if t.Text != "" {
				p.Strategy.AppendText(t.Text)
			}
			if p.Strategy.ShouldInterrupt() {
				// Emit interruption control frame to clear playback/buffer, then
				// mark that we've already interrupted for this bot turn.
				_ = p.PushDownstream(ctx, frames.NewInterruptionFrame())
				p.interruptionSent = true
				p.Strategy.Reset()
			}
		}
	}

	return p.PushDownstream(ctx, f)
}

var _ processors.Processor = (*InterruptionController)(nil)

