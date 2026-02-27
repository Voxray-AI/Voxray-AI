package pipeline

import (
	"context"
	"testing"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors/echo"
)

// TestEchoPipelineIntegration connects echo processor to a sink channel, pushes a frame, and asserts the same frame is received.
func TestEchoPipelineIntegration(t *testing.T) {
	outCh := make(chan frames.Frame, 4)
	pl := New()
	pl.Add(echo.New("echo"))
	pl.Add(NewSink("sink", outCh))

	ctx := context.Background()
	if err := pl.Setup(ctx); err != nil {
		t.Fatal(err)
	}
	defer pl.Cleanup(ctx)
	if err := pl.Start(ctx, frames.NewStartFrame()); err != nil {
		t.Fatal(err)
	}

	tf := frames.NewTextFrame("hello")
	if err := pl.Push(ctx, tf); err != nil {
		t.Fatal(err)
	}

	// Sink receives StartFrame first, then echoed TextFrame
	var got frames.Frame
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		select {
		case got = <-outCh:
			if _, ok := got.(*frames.TextFrame); ok {
				if out := got.(*frames.TextFrame); out.Text != "hello" {
					t.Errorf("TextFrame.Text = %q", out.Text)
				}
				return
			}
			// else discard StartFrame or other
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
	t.Fatal("timeout waiting for echoed TextFrame")
}
