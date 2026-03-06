package interruptions

import "strings"

// Strategy decides when a user interruption should occur while the bot is speaking.
//
// Implementations typically accumulate speech/text during a user turn and, when
// queried via ShouldInterrupt, decide whether an interruption should be issued.
type Strategy interface {
	// Reset clears any accumulated state for a new analysis window.
	Reset()
	// AppendText appends recognized user text to the internal buffer.
	AppendText(text string)
	// ShouldInterrupt reports whether the current accumulated state warrants
	// interrupting the bot (e.g. via an InterruptionFrame).
	ShouldInterrupt() bool
}

// MinWordsStrategy triggers an interruption once at least MinWords have been
// spoken by the user in the current window.
type MinWordsStrategy struct {
	MinWords int
	buf      string
}

// NewMinWordsStrategy constructs a MinWordsStrategy. When minWords is zero or
// negative, it falls back to 1 word.
func NewMinWordsStrategy(minWords int) *MinWordsStrategy {
	if minWords <= 0 {
		minWords = 1
	}
	return &MinWordsStrategy{MinWords: minWords}
}

// Reset clears accumulated text.
func (s *MinWordsStrategy) Reset() {
	s.buf = ""
}

// AppendText appends user text into the internal buffer for later analysis.
func (s *MinWordsStrategy) AppendText(text string) {
	if text == "" {
		return
	}
	if s.buf == "" {
		s.buf = text
		return
	}
	s.buf += " " + text
}

// ShouldInterrupt returns true when the accumulated text contains at least
// MinWords whitespace-separated tokens.
func (s *MinWordsStrategy) ShouldInterrupt() bool {
	if s.MinWords <= 0 {
		return false
	}
	if s.buf == "" {
		return false
	}
	words := strings.Fields(s.buf)
	return len(words) >= s.MinWords
}

// Strategy identifiers used by config and factories.
const (
	StrategyNone     = "none"
	StrategyMinWords = "min_words"
)

// NewStrategy constructs a Strategy from a simple kind + parameter tuple.
// Unknown kinds return nil so callers can gracefully skip interruption
// handling when unsupported strategies are configured.
func NewStrategy(kind string, minWords int) Strategy {
	switch kind {
	case "", StrategyNone:
		return nil
	case StrategyMinWords, "keyword":
		return NewMinWordsStrategy(minWords)
	default:
		return nil
	}
}

