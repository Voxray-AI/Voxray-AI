// Package patternaggregator provides XML-style tag aggregation for LLM text streams.
// It detects open/close delimiters (e.g. " ` " and " ` ") and emits matches with the inner content,
// plus non-tag text segments. Used by IVR to extract DTMF, mode, and status commands.
package patternaggregator

// Match holds the content between a pair of open/close delimiters.
type Match struct {
	Content string
}

// Aggregator consumes incremental text and emits text segments and delimiter matches.
type Aggregator struct {
	open, close string
	buf         string
}

// New creates an aggregator with the given open and close delimiter strings (e.g. " ` " and " ` ").
func New(open, close string) *Aggregator {
	return &Aggregator{open: open, close: close}
}

// Feed appends text and returns any complete text segments (before a match) and matches (content between delimiters).
// Call Flush when the stream ends to get any remaining buffered text.
func (a *Aggregator) Feed(text string) (textSegments []string, matches []Match) {
	a.buf += text
	for {
		openIdx := findSubstring(a.buf, a.open)
		if openIdx < 0 {
			// Emit everything except a suffix that could be a prefix of open.
			safeLen := len(a.buf)
			for i := 1; i < len(a.open) && len(a.buf) >= i; i++ {
				if a.buf[len(a.buf)-i:] == a.open[:i] {
					safeLen = len(a.buf) - i
					break
				}
			}
			if safeLen > 0 {
				textSegments = append(textSegments, a.buf[:safeLen])
				a.buf = a.buf[safeLen:]
			}
			return textSegments, matches
		}
		afterOpen := openIdx + len(a.open)
		if afterOpen > len(a.buf) {
			return textSegments, matches
		}
		closeIdx := findSubstring(a.buf[afterOpen:], a.close)
		if closeIdx < 0 {
			// No close yet; keep only the part that could be start of open.
			if openIdx > 0 {
				textSegments = append(textSegments, a.buf[:openIdx])
			}
			a.buf = a.buf[openIdx:]
			return textSegments, matches
		}
		closeIdx += afterOpen
		content := a.buf[afterOpen:closeIdx]
		if openIdx > 0 {
			textSegments = append(textSegments, a.buf[:openIdx])
		}
		matches = append(matches, Match{Content: content})
		a.buf = a.buf[closeIdx+len(a.close):]
	}
}

// Flush returns any remaining buffered text and clears the buffer.
func (a *Aggregator) Flush() (remaining string) {
	r := a.buf
	a.buf = ""
	return r
}

func findSubstring(s, sub string) int {
	if sub == "" {
		return 0
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
