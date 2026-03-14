// Package pipeline provides pipeline construction and execution.
package pipeline

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/processors"
)

// Pipeline holds a linear chain of processors and orchestrates the flow of frames through them.
// It manages the lifecycle of processors (Setup/Cleanup) and provides methods to inject frames.
// THREAD SAFETY: mu guards processors and startFrame; Push/PushUpstream may be called from multiple goroutines (e.g. runner worker and nested pipelines).
type Pipeline struct {
	mu         sync.Mutex
	processors []processors.Processor
	startFrame *frames.StartFrame
	cancel     context.CancelFunc
}

// New returns a new Pipeline.
func New() *Pipeline {
	return &Pipeline{processors: make([]processors.Processor, 0)}
}

// Add appends a processor and links it to the previous one.
func (p *Pipeline) Add(proc processors.Processor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.processors) > 0 {
		p.processors[len(p.processors)-1].SetNext(proc)
		proc.SetPrev(p.processors[len(p.processors)-1])
	}
	p.processors = append(p.processors, proc)
}

// Link adds multiple processors in order (same as repeated Add).
func (p *Pipeline) Link(procs ...processors.Processor) {
	for _, proc := range procs {
		p.Add(proc)
	}
}

// Processors returns a copy of the processor list.
func (p *Pipeline) Processors() []processors.Processor {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]processors.Processor, len(p.processors))
	copy(out, p.processors)
	return out
}

// Setup calls Setup on all processors.
func (p *Pipeline) Setup(ctx context.Context) error {
	list := p.Processors()
	logger.Info("pipeline: setup with %d processors", len(list))
	for _, proc := range list {
		if err := proc.Setup(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Cleanup calls Cleanup on all processors (reverse order).
func (p *Pipeline) Cleanup(ctx context.Context) {
	list := p.Processors()
	logger.Info("pipeline: cleanup (%d processors)", len(list))
	for i := len(list) - 1; i >= 0; i-- {
		_ = list[i].Cleanup(ctx)
	}
}

// Push injects a frame into the first processor (downstream).
func (p *Pipeline) Push(ctx context.Context, f frames.Frame) error {
	list := p.Processors()
	if len(list) == 0 {
		return nil
	}
	// Log non-audio frames only to avoid flooding (audio frames are pushed at high rate).
	if f != nil {
		switch f.(type) {
		case *frames.AudioRawFrame, *frames.TTSAudioRawFrame:
			// skip logging high-volume audio frames
		default:
			logger.Info("pipeline: push frame type=%s id=%d", f.FrameType(), f.ID())
		}
	}
	return list[0].ProcessFrame(ctx, f, processors.Downstream)
}

// PushUpstream injects a frame into the last processor (upstream). Used when the pipeline is nested (e.g. inside ParallelPipeline).
func (p *Pipeline) PushUpstream(ctx context.Context, f frames.Frame) error {
	list := p.Processors()
	if len(list) == 0 {
		return nil
	}
	return list[len(list)-1].ProcessFrame(ctx, f, processors.Upstream)
}

// Start pushes a StartFrame and stores it for reference; call once before feeding frames.
func (p *Pipeline) Start(ctx context.Context, start *frames.StartFrame) error {
	if start == nil {
		start = frames.NewStartFrame()
	}
	p.mu.Lock()
	p.startFrame = start
	p.mu.Unlock()
	logger.Info("pipeline: start (StartFrame pushed)")
	return p.Push(ctx, start)
}

// Cancel pushes a CancelFrame into the pipeline.
func (p *Pipeline) Cancel(ctx context.Context, reason any) error {
	return p.Push(ctx, frames.NewCancelFrame(reason))
}
