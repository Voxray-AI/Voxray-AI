package logger

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/processors"
)

// Processor logs each frame and forwards it.
type Processor struct {
	*processors.BaseProcessor
}

// New returns a new logger processor.
func New(name string) *Processor {
	if name == "" {
		name = "Logger"
	}
	return &Processor{BaseProcessor: processors.NewBaseProcessor(name)}
}

// ProcessFrame logs the frame type and ID then forwards.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	logger.Debug("frame %s id=%d dir=%d", f.FrameType(), f.ID(), dir)
	if dir == processors.Downstream && p.Next() != nil {
		return p.Next().ProcessFrame(ctx, f, dir)
	}
	if dir == processors.Upstream && p.Prev() != nil {
		return p.Prev().ProcessFrame(ctx, f, dir)
	}
	return nil
}
