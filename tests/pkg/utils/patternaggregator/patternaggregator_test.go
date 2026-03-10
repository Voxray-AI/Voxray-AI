package patternaggregator_test

import (
	"reflect"
	"testing"

	"voxray-go/pkg/utils/patternaggregator"
)

func TestAggregator_Feed(t *testing.T) {
	open := " ` "
	close := " ` "
	agg := patternaggregator.New(open, close)

	segments, matches := agg.Feed("hello")
	if len(matches) != 0 || len(segments) != 1 || segments[0] != "hello" {
		t.Errorf("Feed('hello') = segments %v, matches %v; want segments [hello], matches []", segments, matches)
	}

	segments, matches = agg.Feed(" ` 1 ` ")
	if len(segments) != 0 || len(matches) != 1 || matches[0].Content != "1" {
		t.Errorf("Feed(' ` 1 ` ') = segments %v, matches %v; want one match Content=1", segments, matches)
	}

	segments, matches = agg.Feed(" world")
	if len(segments) != 1 || segments[0] != " world" || len(matches) != 0 {
		t.Errorf("Feed(' world') = segments %v, matches %v", segments, matches)
	}

	remaining := agg.Flush()
	if remaining != "" {
		t.Errorf("Flush() = %q; want empty", remaining)
	}
}

func TestAggregator_FeedIncremental(t *testing.T) {
	agg := patternaggregator.New(" ` ", " ` ")
	var allSegments []string
	var allMatches []patternaggregator.Match
	for _, chunk := range []string{" pre ", " ` ", "dtmf", " ` ", " post"} {
		seg, mat := agg.Feed(chunk)
		allSegments = append(allSegments, seg...)
		allMatches = append(allMatches, mat...)
	}
	if !reflect.DeepEqual(allMatches, []patternaggregator.Match{{Content: "dtmf"}}) {
		t.Errorf("matches = %v; want [Match{Content: \"dtmf\"}]", allMatches)
	}
	// We should get " pre " (or prefix) and " post" among segments; exact count depends on chunk boundaries.
	if len(allSegments) < 1 {
		t.Errorf("segments = %v; want at least one", allSegments)
	}
	remaining := agg.Flush()
	// remaining is any buffered text not yet emitted; may be empty (kept for inspection in test).
	_ = remaining
}
