// Package pipeline provides ParallelPipeline for concurrent frame processing.
package pipeline

import (
	"context"
	"fmt"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/processors"
)

func isLifecycleFrame(f frames.Frame) bool {
	if f == nil {
		return false
	}
	switch f.(type) {
	case *frames.StartFrame, *frames.EndFrame, *frames.CancelFrame:
		return true
	default:
		return false
	}
}

// OutputFilter if set is called before forwarding a frame from a branch. If it returns false the frame is not forwarded.
type OutputFilter func(f frames.Frame) bool

// ParallelPipeline processes frames through multiple sub-pipelines concurrently.
// Each branch receives the same input. Lifecycle frames (Start, End, Cancel) are
// synchronized: the frame is pushed to all branches and forwarded once when all
// have processed it. Non-lifecycle frames are pushed to all branches and forwarded
// once (deduplicated by frame ID).
type ParallelPipeline struct {
	*processors.BaseProcessor
	branches      []*Pipeline
	mu            sync.Mutex
	syncFrameID   uint64
	counter       int
	synchronizing bool
	buffered      []frames.Frame
	seenIDs       map[uint64]struct{}
	outputFilter  OutputFilter
}

// SetOutputFilter sets an optional filter for frames emitted from branches. Used by ServiceSwitcher.
func (pp *ParallelPipeline) SetOutputFilter(f OutputFilter) { pp.outputFilter = f }

// NewParallelPipeline builds a parallel pipeline from N branch definitions.
// Each branch is a slice of processors; each branch is wrapped with an internal
// source and sink so the parallel pipeline can inject input and collect output.
// At least one branch is required.
func NewParallelPipeline(branches [][]processors.Processor) (*ParallelPipeline, error) {
	if len(branches) == 0 {
		return nil, fmt.Errorf("ParallelPipeline needs at least one branch")
	}
	pp := &ParallelPipeline{
		BaseProcessor: processors.NewBaseProcessor("ParallelPipeline"),
		branches:      make([]*Pipeline, 0, len(branches)),
		seenIDs:       make(map[uint64]struct{}),
	}
	for i, procs := range branches {
		pl := New()
		src := NewPipelineSource(pp.sourceName(i), func(ctx context.Context, f frames.Frame) error {
			return pp.onUpstreamFromBranch(ctx, f)
		})
		pl.Add(src)
		for _, p := range procs {
			pl.Add(p)
		}
		sink := NewPipelineSinkCallback(pp.sinkName(i), func(ctx context.Context, f frames.Frame) error {
			return pp.onDownstreamFromBranch(ctx, f)
		})
		pl.Add(sink)
		pp.branches = append(pp.branches, pl)
	}
	logger.Info("parallel pipeline: created %d branches", len(pp.branches))
	return pp, nil
}

func (pp *ParallelPipeline) sourceName(i int) string { return fmt.Sprintf("ParallelPipeline::Source%d", i) }
func (pp *ParallelPipeline) sinkName(i int) string   { return fmt.Sprintf("ParallelPipeline::Sink%d", i) }

func (pp *ParallelPipeline) onUpstreamFromBranch(ctx context.Context, f frames.Frame) error {
	if pp.Prev() != nil {
		return pp.Prev().ProcessFrame(ctx, f, processors.Upstream)
	}
	return nil
}

func (pp *ParallelPipeline) onDownstreamFromBranch(ctx context.Context, f frames.Frame) error {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	if isLifecycleFrame(f) {
		if f.ID() == pp.syncFrameID && pp.synchronizing {
			pp.counter--
			if pp.counter == 0 {
				pp.synchronizing = false
				if pp.Next() != nil {
					_ = pp.Next().ProcessFrame(ctx, f, processors.Downstream)
				}
				for _, bf := range pp.buffered {
					if pp.Next() != nil {
						_ = pp.Next().ProcessFrame(ctx, bf, processors.Downstream)
					}
				}
				pp.buffered = nil
			}
		}
		return nil
	}

	if pp.synchronizing {
		pp.buffered = append(pp.buffered, f)
		return nil
	}

	if _, seen := pp.seenIDs[f.ID()]; seen {
		return nil
	}
	if pp.outputFilter != nil && !pp.outputFilter(f) {
		return nil
	}
	pp.seenIDs[f.ID()] = struct{}{}
	if pp.Next() != nil {
		return pp.Next().ProcessFrame(ctx, f, processors.Downstream)
	}
	return nil
}

// Push sends a frame to all branches (downstream). Used by callers (e.g. ServiceSwitcher) to inject frames.
func (pp *ParallelPipeline) Push(ctx context.Context, f frames.Frame) error {
	for _, pl := range pp.branches {
		if err := pl.Push(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

// ProcessFrame implements Processor. Downstream frames are pushed to all branches;
// lifecycle frames are synchronized and forwarded once.
func (pp *ParallelPipeline) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Upstream {
		if pp.Prev() != nil {
			return pp.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	pp.mu.Lock()
	if isLifecycleFrame(f) {
		pp.syncFrameID = f.ID()
		pp.counter = len(pp.branches)
		pp.synchronizing = true
	}
	pp.mu.Unlock()

	for _, pl := range pp.branches {
		if err := pl.Push(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

// Setup calls Setup on all branch pipelines.
func (pp *ParallelPipeline) Setup(ctx context.Context) error {
	for _, pl := range pp.branches {
		if err := pl.Setup(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Cleanup calls Cleanup on all branch pipelines.
func (pp *ParallelPipeline) Cleanup(ctx context.Context) error {
	for _, pl := range pp.branches {
		pl.Cleanup(ctx)
	}
	return nil
}

// Branches returns the internal pipelines (read-only).
func (pp *ParallelPipeline) Branches() []*Pipeline {
	return pp.branches
}

var _ processors.Processor = (*ParallelPipeline)(nil)
