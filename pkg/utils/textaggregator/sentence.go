package textaggregator

import (
	"strings"
)

// SentenceAggregator emits a segment when a sentence-ending rune is seen or max runes reached.
type SentenceAggregator struct {
	SentenceEnd string
	MaxRunes    int

	buf strings.Builder
}

// NewSentenceAggregator returns a sentence-based aggregator. sentenceEnd defaults to ".!?"; maxRunes 0 = no limit.
func NewSentenceAggregator(sentenceEnd string, maxRunes int) *SentenceAggregator {
	if sentenceEnd == "" {
		sentenceEnd = ".!?"
	}
	return &SentenceAggregator{SentenceEnd: sentenceEnd, MaxRunes: maxRunes}
}

// Aggregate appends text and returns complete sentence segments.
func (s *SentenceAggregator) Aggregate(text string) []Segment {
	s.buf.WriteString(text)
	var out []Segment
	for {
		seg := s.tryCut()
		if seg == nil {
			break
		}
		out = append(out, *seg)
	}
	return out
}

func (s *SentenceAggregator) tryCut() *Segment {
	str := s.buf.String()
	if str == "" {
		return nil
	}
	runes := []rune(str)
	cut := -1
	if s.MaxRunes > 0 && len(runes) >= s.MaxRunes {
		cut = s.MaxRunes
	} else {
		for i, r := range runes {
			if strings.ContainsRune(s.SentenceEnd, r) {
				cut = i + 1
				break
			}
		}
	}
	if cut < 0 {
		return nil
	}
	segment := string(runes[:cut])
	s.buf.Reset()
	if cut < len(runes) {
		s.buf.WriteString(string(runes[cut:]))
	}
	return &Segment{Text: strings.TrimSpace(segment), Type: "sentence"}
}

// Flush returns remaining buffer as one segment and resets.
func (s *SentenceAggregator) Flush() *Segment {
	text := strings.TrimSpace(s.buf.String())
	s.buf.Reset()
	if text == "" {
		return nil
	}
	return &Segment{Text: text, Type: "sentence"}
}

// Reset clears the buffer without emitting.
func (s *SentenceAggregator) Reset() {
	s.buf.Reset()
}

// HandleInterruption clears the buffer.
func (s *SentenceAggregator) HandleInterruption() {
	s.buf.Reset()
}
