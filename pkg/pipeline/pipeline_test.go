package pipeline

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors/echo"
)

func TestPipeline_Add_Push(t *testing.T) {
	pl := New()
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
