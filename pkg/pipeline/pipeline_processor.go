// Package pipeline provides PipelineProcessor to use a Pipeline as a Processor node.
package pipeline

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// PipelineProcessor wraps a Pipeline so it can be used as a Processor (e.g. inside ParallelPipeline).
type PipelineProcessor struct {
	*processors.BaseProcessor
	Pipeline *Pipeline
	name     string
}

// NewPipelineProcessor returns a Processor that delegates to the given pipeline.
// name is used for Name(); if empty, "Pipeline" is used.
func NewPipelineProcessor(name string, pl *Pipeline) *PipelineProcessor {
	if name == "" {
		name = "Pipeline"
	}
	return &PipelineProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		Pipeline:      pl,
		name:          name,
	}
}

// Name returns the processor name.
func (pp *PipelineProcessor) Name() string {
	if pp.name != "" {
		return pp.name
	}
	return "Pipeline"
}

// ProcessFrame forwards downstream frames into the pipeline via Push, upstream frames via PushUpstream.
func (pp *PipelineProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if pp.Pipeline == nil {
		return nil
	}
	if dir == processors.Downstream {
		return pp.Pipeline.Push(ctx, f)
	}
	return pp.Pipeline.PushUpstream(ctx, f)
}

// Setup calls Setup on the wrapped pipeline.
func (pp *PipelineProcessor) Setup(ctx context.Context) error {
	if pp.Pipeline == nil {
		return nil
	}
	return pp.Pipeline.Setup(ctx)
}

// Cleanup calls Cleanup on the wrapped pipeline.
func (pp *PipelineProcessor) Cleanup(ctx context.Context) error {
	if pp.Pipeline == nil {
		return nil
	}
	pp.Pipeline.Cleanup(ctx)
	return nil
}

var _ processors.Processor = (*PipelineProcessor)(nil)
