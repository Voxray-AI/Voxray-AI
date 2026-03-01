// Package userresponse provides a processor that aggregates TranscriptionFrame into a single TextFrame when the user turn ends (e.g. UserStoppedSpeakingFrame).
package userresponse

import (
	"context"
	"strings"
	"sync"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

// Processor buffers TranscriptionFrame text and emits one TextFrame when the user turn ends.
type Processor struct {
	*processors.BaseProcessor

	mu  sync.Mutex
	buf strings.Builder
}

// New returns a user response aggregator.
func New(name string) *Processor {
	if name == "" {
		name = "UserResponseAggregator"
	}
	return &Processor{
		BaseProcessor: processors.NewBaseProcessor(name),
	}
}

// ProcessFrame buffers transcription; on UserStoppedSpeakingFrame or End/Cancel flushes one TextFrame.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.TranscriptionFrame:
		p.mu.Lock()
		if p.buf.Len() > 0 {
			p.buf.WriteString(" ")
		}
		p.buf.WriteString(t.Text)
		p.mu.Unlock()
		return nil

	case *frames.UserStoppedSpeakingFrame:
		p.flushThenForward(ctx, f)
		return nil

	case *frames.EndFrame, *frames.CancelFrame:
		p.flushThenForward(ctx, f)
		return nil

	default:
		return p.PushDownstream(ctx, f)
	}
}

func (p *Processor) flushThenForward(ctx context.Context, boundary frames.Frame) {
	p.mu.Lock()
	text := strings.TrimSpace(p.buf.String())
	p.buf.Reset()
	p.mu.Unlock()

	if text != "" {
		ts := time.Now().UTC().Format(time.RFC3339)
		tf := frames.NewTranscriptionFrame(text, "", ts, true)
		_ = p.PushDownstream(ctx, tf)
	}
	_ = p.PushDownstream(ctx, boundary)
}
