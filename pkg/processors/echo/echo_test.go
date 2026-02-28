package echo

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

func TestNew(t *testing.T) {
	p := New("echo")
	if p == nil || p.Name() != "echo" {
		t.Errorf("New: got %v", p)
	}
}

func TestEcho_ForwardsFrame(t *testing.T) {
	p := New("echo")
	ctx := context.Background()
	f := frames.NewTextFrame("hi")
	err := p.ProcessFrame(ctx, f, processors.Downstream)
	if err != nil {
		t.Fatal(err)
	}
}
