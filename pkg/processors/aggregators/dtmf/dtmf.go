// Package dtmf provides a DTMF aggregator that accumulates InputDTMFFrame digits
// and emits TranscriptionFrame on timeout, termination digit (#), or EndFrame/CancelFrame.
package dtmf

import (
	"context"
	"sync"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// Processor aggregates DTMF digits and flushes as TranscriptionFrame for LLM context.
type Processor struct {
	*processors.BaseProcessor
	Timeout          time.Duration
	TerminationDigit frames.KeypadEntry
	Prefix           string
	PushInterruption bool // if true, push InterruptionFrame on first digit

	mu      sync.Mutex
	buf     string
	timer   *time.Timer
	started bool
}

// New returns a DTMF aggregator. Timeout 0 defaults to 2s; empty Prefix to "DTMF: ".
func New(name string, timeout time.Duration, terminationDigit frames.KeypadEntry, prefix string) *Processor {
	if name == "" {
		name = "DTMFAggregator"
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	if prefix == "" {
		prefix = "DTMF: "
	}
	if terminationDigit == "" {
		terminationDigit = frames.KeypadPound
	}
	return &Processor{
		BaseProcessor:     processors.NewBaseProcessor(name),
		Timeout:           timeout,
		TerminationDigit:  terminationDigit,
		Prefix:            prefix,
		PushInterruption: true,
	}
}

// ProcessFrame accumulates InputDTMFFrame digits; flushes on timeout, #, or End/Cancel.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.StartFrame:
		p.mu.Lock()
		p.started = true
		p.mu.Unlock()
		return p.PushDownstream(ctx, f)

	case *frames.EndFrame, *frames.CancelFrame:
		p.mu.Lock()
		p.stopTimerLocked()
		p.mu.Unlock()
		p.flushAndForward(ctx, f)
		p.mu.Lock()
		p.started = false
		p.mu.Unlock()
		return nil

	case *frames.InputDTMFFrame:
		// Forward DTMF frame downstream first (Python: push then handle)
		if err := p.PushDownstream(ctx, f); err != nil {
			return err
		}
		p.handleDTMF(ctx, t)
		return nil

	default:
		return p.PushDownstream(ctx, f)
	}
}

func (p *Processor) handleDTMF(ctx context.Context, frame *frames.InputDTMFFrame) {
	p.mu.Lock()
	wasEmpty := p.buf == ""
	digit := string(frame.Digit)
	p.buf += digit

	if wasEmpty && p.PushInterruption {
		p.mu.Unlock()
		_ = p.PushDownstream(ctx, frames.NewInterruptionFrame())
		p.mu.Lock()
	}

	if frame.Digit == p.TerminationDigit {
		p.stopTimerLocked()
		p.mu.Unlock()
		p.flushAndForward(ctx, nil)
		return
	}

	p.resetTimerLocked()
	p.mu.Unlock()
}

func (p *Processor) resetTimerLocked() {
	p.stopTimerLocked()
	timer := time.AfterFunc(p.Timeout, func() {
		p.flushAndForward(context.Background(), nil)
	})
	p.timer = timer
}

func (p *Processor) stopTimerLocked() {
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
}

func (p *Processor) flushAndForward(ctx context.Context, after frames.Frame) {
	p.mu.Lock()
	text := p.buf
	p.buf = ""
	p.stopTimerLocked()
	p.mu.Unlock()

	if text != "" {
		ts := time.Now().UTC().Format(time.RFC3339)
		tf := frames.NewTranscriptionFrame(p.Prefix+text, "", ts, true)
		_ = p.PushDownstream(ctx, tf)
	}
	if after != nil {
		_ = p.PushDownstream(ctx, after)
	}
}

// Cleanup stops the idle timer and clears state.
func (p *Processor) Cleanup(ctx context.Context) error {
	p.mu.Lock()
	p.stopTimerLocked()
	p.buf = ""
	p.started = false
	p.mu.Unlock()
	return p.BaseProcessor.Cleanup(ctx)
}
