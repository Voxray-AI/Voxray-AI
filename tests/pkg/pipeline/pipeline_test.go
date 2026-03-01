package pipeline_test

import (
	"context"
	"sync"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors/echo"
	"voila-go/pkg/processors/filters"
)

func TestPipeline_Add_Push(t *testing.T) {
	pl := pipeline.New()
	pl.Add(echo.New("e1"))
	pl.Add(echo.New("e2"))
	if len(pl.Processors()) != 2 {
		t.Fatalf("len(Processors()) = %d", len(pl.Processors()))
	}
	ctx := context.Background()
	if err := pl.Setup(ctx); err != nil {
		t.Fatal(err)
	}
	defer pl.Cleanup(ctx)
	start := frames.NewStartFrame()
	if err := pl.Start(ctx, start); err != nil {
		t.Fatal(err)
	}
	tf := frames.NewTextFrame("hi")
	if err := pl.Push(ctx, tf); err != nil {
		t.Fatal(err)
	}
}

func TestNewStartFrame(t *testing.T) {
	f := frames.NewStartFrame()
	if f == nil || f.FrameType() != "StartFrame" {
		t.Errorf("unexpected: %v", f)
	}
}

// TestPipelineTask_QueueRunHasFinished mirrors upstream test_pipeline.py task_single:
// queue frames (TextFrame, EndFrame), run task, then assert task has finished.
func TestPipelineTask_QueueRunHasFinished(t *testing.T) {
	pl := pipeline.New()
	pl.Add(filters.NewIdentityFilter("id"))
	task := pipeline.NewPipelineTask("task", pl)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = task.Run(ctx, pipeline.TaskParams{})
	}()

	// Queue StartFrame so pipeline gets init, then TextFrame, then EndFrame; then cancel to stop Run.
	_ = task.QueueFrame(ctx, frames.NewStartFrame())
	_ = task.QueueFrame(ctx, frames.NewTextFrame("Hello!"))
	_ = task.QueueFrame(ctx, frames.NewEndFrame())
	cancel()
	wg.Wait()

	if !task.HasFinished() {
		t.Error("expected HasFinished() true after Run returns")
	}
}

// TestPipelineTask_Cancel stops the task when Cancel is called (queues CancelFrame and cancels context).
func TestPipelineTask_Cancel(t *testing.T) {
	pl := pipeline.New()
	pl.Add(filters.NewIdentityFilter("id"))
	task := pipeline.NewPipelineTask("task", pl)
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = task.Run(ctx, pipeline.TaskParams{})
	}()

	_ = task.QueueFrame(ctx, frames.NewStartFrame())
	_ = task.QueueFrame(ctx, frames.NewTextFrame("Hi"))
	task.Cancel(ctx)
	wg.Wait()

	if !task.HasFinished() {
		t.Error("expected HasFinished() true after Cancel")
	}
}

