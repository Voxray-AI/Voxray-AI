package processors

import (
	"context"
	"testing"

	"voila-go/pkg/frames"
)

func TestBaseProcessor_Name(t *testing.T) {
	b := NewBaseProcessor("myproc")
	if b.Name() != "myproc" {
		t.Errorf("Name() = %q", b.Name())
	}
	b2 := NewBaseProcessor("")
	if b2.Name() != "BaseProcessor" {
		t.Errorf("empty name: got %q", b2.Name())
	}
}

func TestBaseProcessor_Link(t *testing.T) {
	a := NewBaseProcessor("a")
	b := NewBaseProcessor("b")
	a.SetNext(b)
	b.SetPrev(a)
	if a.Next() != b || b.Prev() != a {
		t.Error("link mismatch")
	}
}

func TestBaseProcessor_ProcessFrame_ForwardsDownstream(t *testing.T) {
	a := NewBaseProcessor("a")
	b := NewBaseProcessor("b")
	a.SetNext(b)
	ctx := context.Background()
	f := frames.NewTextFrame("hi")
	if err := a.ProcessFrame(ctx, f, Downstream); err != nil {
		t.Fatal(err)
	}
	// b just forwards to next (nil); no error
}

func TestBaseProcessor_ProcessFrame_ForwardsUpstream(t *testing.T) {
	a := NewBaseProcessor("a")
	b := NewBaseProcessor("b")
	a.SetNext(b)
	b.SetPrev(a)
	ctx := context.Background()
	f := frames.NewCancelFrame("done")
	if err := b.ProcessFrame(ctx, f, Upstream); err != nil {
		t.Fatal(err)
	}
}
