// Package textaggregator provides an interface and implementations for aggregating
// incremental text (e.g. LLM tokens) into segments (e.g. sentences).
package textaggregator

// Segment is one emitted aggregation (text and type label).
type Segment struct {
	Text string
	Type string
}

// Aggregator consumes incremental text and yields complete segments (e.g. by sentence).
type Aggregator interface {
	// Aggregate appends text and returns any complete segments. Caller may get 0, 1, or more.
	Aggregate(text string) []Segment
	// Flush returns any remaining buffered text as a single segment and clears the buffer.
	Flush() *Segment
	// Reset clears state without emitting.
	Reset()
	// HandleInterruption clears state (e.g. on barge-in).
	HandleInterruption()
}
