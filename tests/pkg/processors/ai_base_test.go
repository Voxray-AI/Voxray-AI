package processors_test

import (
	"context"
	"testing"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
)

func TestAIServiceBase_Settings(t *testing.T) {
	base := processors.NewAIServiceBase("test", &processors.ServiceSettings{Model: "gpt-4", Voice: "alloy"})
	s := base.Settings()
	if s.Model != "gpt-4" || s.Voice != "alloy" {
		t.Errorf("Settings() = %+v", s)
	}
	base.ApplySettings(processors.ServiceSettings{Model: "gpt-3.5"})
	s = base.Settings()
	if s.Model != "gpt-3.5" {
		t.Errorf("after ApplySettings Model = %q", s.Model)
	}
}

func TestAIServiceBase_ProcessFrame_Forwards(t *testing.T) {
	base := processors.NewAIServiceBase("test", nil)
	col := &collector{}
	base.SetNext(col)
	ctx := context.Background()

	// StartFrame: base handles then forwards
	if err := base.ProcessFrame(ctx, frames.NewStartFrame(), processors.Downstream); err != nil {
		t.Fatal(err)
	}
	if len(col.received) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(col.received))
	}
	if _, ok := col.received[0].(*frames.StartFrame); !ok {
		t.Errorf("expected StartFrame, got %T", col.received[0])
	}

	col.received = nil
	if err := base.ProcessFrame(ctx, frames.NewEndFrame(), processors.Downstream); err != nil {
		t.Fatal(err)
	}
	if len(col.received) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(col.received))
	}
}

type collector struct {
	processors.BaseProcessor
	received []frames.Frame
}

func (c *collector) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	c.received = append(c.received, f)
	return nil
}
