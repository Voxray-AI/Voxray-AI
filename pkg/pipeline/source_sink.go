package pipeline

import (
	"context"
	"sync"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/processors"
)

// DownstreamCallback is called when a frame is emitted downstream (e.g. from a sink).
type DownstreamCallback func(ctx context.Context, f frames.Frame) error

// UpstreamCallback is called when a frame is emitted upstream (e.g. from a source).
type UpstreamCallback func(ctx context.Context, f frames.Frame) error

// Source is a processor that reads frames from a channel and pushes them downstream.
type Source struct {
	*processors.BaseProcessor
	In <-chan frames.Frame
}

// NewSource returns a Source that reads from ch.
func NewSource(name string, ch <-chan frames.Frame) *Source {
	if name == "" {
		name = "Source"
	}
	return &Source{BaseProcessor: processors.NewBaseProcessor(name), In: ch}
}

// Run reads from In and pushes to next until context is done or channel closed.
func (s *Source) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-s.In:
			if !ok {
				return
			}
			_ = s.PushDownstream(ctx, f)
		}
	}
}

const sinkWriterQueueSize = 128

// Sink is a processor that forwards all frames to a channel (for transport output).
// When Setup(ctx) is called, a single writer goroutine runs so we do not spawn one
// goroutine per frame (which can cause scheduler saturation at high TTS frame rates).
type Sink struct {
	*processors.BaseProcessor
	Out     chan<- frames.Frame
	ttsLogOnce sync.Once

	// Single writer: ProcessFrame sends here; runWriter sends to Out.
	sendCh    chan frames.Frame
	writerDone chan struct{}
}

// NewSink returns a Sink that writes to ch.
func NewSink(name string, ch chan<- frames.Frame) *Sink {
	if name == "" {
		name = "Sink"
	}
	return &Sink{BaseProcessor: processors.NewBaseProcessor(name), Out: ch}
}

// Setup starts the single writer goroutine. Must be called before ProcessFrame (pipeline does this).
func (s *Sink) Setup(ctx context.Context) error {
	if err := s.BaseProcessor.Setup(ctx); err != nil {
		return err
	}
	if s.Out == nil {
		return nil
	}
	s.sendCh = make(chan frames.Frame, sinkWriterQueueSize)
	s.writerDone = make(chan struct{})
	go s.runWriter(ctx)
	return nil
}

// runWriter reads from sendCh and writes to Out. Exits when ctx is done or sendCh is closed.
func (s *Sink) runWriter(ctx context.Context) {
	defer close(s.writerDone)
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-s.sendCh:
			if !ok {
				return
			}
			func() {
				defer func() { _ = recover() }()
				select {
				case s.Out <- f:
				case <-ctx.Done():
					return
				}
			}()
		}
	}
}

// Cleanup closes the send channel and waits for the writer to exit.
func (s *Sink) Cleanup(ctx context.Context) error {
	if s.sendCh != nil {
		close(s.sendCh)
		s.sendCh = nil
		select {
		case <-s.writerDone:
		case <-ctx.Done():
		}
	}
	return s.BaseProcessor.Cleanup(ctx)
}

// ProcessFrame forwards the frame to Out and does not call next (end of chain).
func (s *Sink) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if s.Prev() != nil {
			return s.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	if f != nil {
		if _, ok := f.(*frames.TTSAudioRawFrame); ok {
			s.ttsLogOnce.Do(func() { logger.Info("pipeline (sink): first TTS audio frame to transport") })
		} else {
			logger.Info("pipeline (sink): forwarding to transport frame type=%s id=%d", f.FrameType(), f.ID())
		}
	}
	if s.Out != nil && f != nil {
		if s.sendCh != nil {
			select {
			case s.sendCh <- f:
			case <-ctx.Done():
			}
		} else {
			// Fallback when Setup was not called (e.g. some tests)
			go func() {
				defer func() { _ = recover() }()
				select {
				case s.Out <- f:
				case <-ctx.Done():
				}
			}()
		}
	}
	return nil
}

// PipelineSource is a processor that acts as the entry point of a pipeline; it forwards
// downstream frames to the next processor and upstream frames to the provided callback.
type PipelineSource struct {
	*processors.BaseProcessor
	OnUpstream UpstreamCallback
	name       string
}

// NewPipelineSource returns a source that forwards upstream frames to the callback.
func NewPipelineSource(name string, onUpstream UpstreamCallback) *PipelineSource {
	if name == "" {
		name = "PipelineSource"
	}
	return &PipelineSource{BaseProcessor: processors.NewBaseProcessor(name), OnUpstream: onUpstream, name: name}
}

// ProcessFrame implements Processor. Downstream goes to next; upstream goes to OnUpstream.
func (s *PipelineSource) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Downstream {
		if s.Next() != nil {
			return s.Next().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	if dir == processors.Upstream && s.OnUpstream != nil {
		return s.OnUpstream(ctx, f)
	}
	return nil
}

// PipelineSinkCallback is a processor that acts as the exit point of a pipeline; it forwards
// upstream frames to the previous processor and downstream frames to the provided callback.
type PipelineSinkCallback struct {
	*processors.BaseProcessor
	OnDownstream DownstreamCallback
	name         string
}

// NewPipelineSinkCallback returns a sink that forwards downstream frames to the callback.
func NewPipelineSinkCallback(name string, onDownstream DownstreamCallback) *PipelineSinkCallback {
	if name == "" {
		name = "PipelineSink"
	}
	return &PipelineSinkCallback{BaseProcessor: processors.NewBaseProcessor(name), OnDownstream: onDownstream, name: name}
}

// ProcessFrame implements Processor. Upstream goes to prev; downstream goes to OnDownstream.
func (s *PipelineSinkCallback) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Upstream {
		if s.Prev() != nil {
			return s.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	if dir == processors.Downstream && s.OnDownstream != nil {
		return s.OnDownstream(ctx, f)
	}
	return nil
}
