package interruptions_test

import (
	"testing"

	"voxray-go/pkg/audio/interruptions"
)

func TestMinWordsStrategy_ShouldInterruptAtThreshold(t *testing.T) {
	s := interruptions.NewMinWordsStrategy(3)
	s.AppendText("hello world")
	if s.ShouldInterrupt() {
		t.Fatalf("expected no interruption before min words reached")
	}
	s.AppendText("from voxray")
	if !s.ShouldInterrupt() {
		t.Fatalf("expected interruption after >= min words")
	}
}

func TestMinWordsStrategy_ResetClearsState(t *testing.T) {
	s := interruptions.NewMinWordsStrategy(2)
	s.AppendText("one two")
	if !s.ShouldInterrupt() {
		t.Fatalf("expected interruption after two words")
	}
	s.Reset()
	if s.ShouldInterrupt() {
		t.Fatalf("expected no interruption immediately after reset")
	}
}

func TestNewStrategy_KeywordMapsToMinWords(t *testing.T) {
	strat := interruptions.NewStrategy("keyword", 2)
	if strat == nil {
		t.Fatalf("expected non-nil strategy for keyword")
	}
	strat.AppendText("one")
	if strat.ShouldInterrupt() {
		t.Fatalf("expected no interruption before second word")
	}
	strat.AppendText("two")
	if !strat.ShouldInterrupt() {
		t.Fatalf("expected interruption after two words with keyword strategy")
	}
}

