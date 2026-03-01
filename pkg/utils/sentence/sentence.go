// Package sentence provides helpers for sentence-boundary detection in aggregated text.
package sentence

import (
	"strings"
)

// DefaultSentenceEnd is the default set of runes that mark end of sentence.
const DefaultSentenceEnd = ".!?"

// MatchEndOfSentence reports whether s (after trim) ends with a sentence-ending rune.
// Sentence-ending runes are from endChars; if endChars is empty, DefaultSentenceEnd is used.
// This matches match_endofsentence behavior for aggregators.
func MatchEndOfSentence(s string, endChars string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if endChars == "" {
		endChars = DefaultSentenceEnd
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return false
	}
	last := runes[len(runes)-1]
	return strings.ContainsRune(endChars, last)
}
