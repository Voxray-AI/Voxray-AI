// Package pipeline provides SyncParallelPipeline for synchronized parallel frame processing.
package pipeline

import (
	"context"
	"fmt"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/processors"
)

func isSystemOrEndFrame(f frames.Frame) bool {
	if f == nil {
		return false
	}
	switch f.(type) {
	case *frames.StartFrame, *frames.EndFrame, *frames.CancelFrame, *frames.StopFrame:
		return true
	default:
		return false
	}
}

func isSyncFrame(f frames.Frame) bool {
	_, ok := f.(*frames.SyncFrame)
	return ok
}

// SyncParallelPipeline processes frames through multiple parallel branches and synchronizes
// their output so that we only proceed when all branches have processed the current frame.
// It uses SyncFrame so that after each frame we wait for all branches to emit SyncFrame.
type SyncParallelPipeline struct {
	*processors.BaseProcessor
	branches []*Pipeline
	outChs   []chan frames.Frame // one per branch for collecting output
	seenIDs  map[uint64]struct{}
	mu       sync.Mutex
}

// NewSyncParallelPipeline builds a synchronous parallel pipeline. Each branch is a slice of
// processors; each branch is wrapped with a sink that writes to an internal channel.
// At least one branch is required.
func NewSyncParallelPipeline(branches [][]processors.Processor) (*SyncParallelPipeline, error) {
	if len(branches) == 0 {
		return nil, fmt.Errorf("SyncParallelPipeline needs at least one branch")
	}
	sp := &SyncParallelPipeline{
		BaseProcessor: processors.NewBaseProcessor("SyncParallelPipeline"),
		branches:      make([]*Pipeline, 0, len(branches)),
		outChs:        make([]chan frames.Frame, 0, len(branches)),
		seenIDs:       make(map[uint64]struct{}),
	}
	for i, procs := range branches {
		outCh := make(chan frames.Frame, 64)
		sp.outChs = append(sp.outChs, outCh)
		pl := New()
		for _, p := range procs {
			pl.Add(p)
		}
		sink := NewSink(fmt.Sprintf("SyncParallelPipeline::Sink%d", i), outCh)
		pl.Add(sink)
		sp.branches = append(sp.branches, pl)
	}
	logger.Info("sync parallel pipeline: created %d branches", len(sp.branches))
	return sp, nil
}

// ProcessFrame implements Processor. It pushes the frame and then a SyncFrame to each branch,
// then waits until each branch has emitted SyncFrame (reading and collecting intermediate frames),
// then forwards collected frames with dedupe.
func (sp *SyncParallelPipeline) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Upstream {
		if sp.Prev() != nil {
			return sp.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	// Push frame to all branches
	for _, pl := range sp.branches {
		if err := pl.Push(ctx, f); err != nil {
			return err
		}
	}

	// Push SyncFrame to all branches so they signal when done
	syncFrame := frames.NewSyncFrame()
	for _, pl := range sp.branches {
		if err := pl.Push(ctx, syncFrame); err != nil {
			return err
		}
	}

	// Read from each branch until we get SyncFrame (or system/end), collect others
	var collected []frames.Frame
	for _, ch := range sp.outChs {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out := <-ch:
				if out == nil {
					continue
				}
				if isSyncFrame(out) {
					goto nextBranch
				}
				collected = append(collected, out)
			}
		}
	nextBranch:
	}

	// Forward collected frames with dedupe
	sp.mu.Lock()
	for _, cf := range collected {
		if _, seen := sp.seenIDs[cf.ID()]; !seen {
			sp.seenIDs[cf.ID()] = struct{}{}
			if sp.Next() != nil {
				_ = sp.Next().ProcessFrame(ctx, cf, processors.Downstream)
			}
		}
	}
	// Clear seen IDs on StartFrame so new session doesn't reuse old IDs
	if _, ok := f.(*frames.StartFrame); ok {
		sp.seenIDs = make(map[uint64]struct{})
	}
	sp.mu.Unlock()
	return nil
}

// Setup calls Setup on all branch pipelines.
func (sp *SyncParallelPipeline) Setup(ctx context.Context) error {
	for _, pl := range sp.branches {
		if err := pl.Setup(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Cleanup calls Cleanup on all branch pipelines.
func (sp *SyncParallelPipeline) Cleanup(ctx context.Context) error {
	for _, pl := range sp.branches {
		pl.Cleanup(ctx)
	}
	return nil
}

// Branches returns the internal pipelines (read-only).
func (sp *SyncParallelPipeline) Branches() []*Pipeline {
	return sp.branches
}

var _ processors.Processor = (*SyncParallelPipeline)(nil)
