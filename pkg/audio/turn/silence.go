package turn

import (
	"context"
	"sync"
	"time"
)

// Default silence-based turn parameters.
const (
	DefaultStopSecs        = 3
	DefaultPreSpeechMs     = 500
	DefaultMaxDurationSecs = 8
)

// bufferedChunk holds a timestamp and raw PCM bytes for one chunk.
type bufferedChunk struct {
	at   time.Time
	data []byte
}

// SilenceTurnAnalyzer implements Analyzer using silence duration after speech.
// When silence exceeds StopSecs after speech has been detected, the turn is Complete.
type SilenceTurnAnalyzer struct {
	mu sync.Mutex

	params Params
	sampleRate int

	initSampleRate *int // if set, overrides SetSampleRate until first set

	// buffer: chunks of (timestamp, audio)
	buffer          []bufferedChunk
	speechTriggered bool
	silenceMs       float64
	speechStartTime time.Time
	lastState       EndOfTurnState
	vadStartSecs    float64
}

// NewSilenceTurnAnalyzer creates a silence-based turn analyzer with the given params.
// Zero values in params use DefaultStopSecs, DefaultPreSpeechMs, DefaultMaxDurationSecs.
func NewSilenceTurnAnalyzer(params Params) *SilenceTurnAnalyzer {
	if params.StopSecs <= 0 {
		params.StopSecs = DefaultStopSecs
	}
	if params.PreSpeechMs <= 0 {
		params.PreSpeechMs = DefaultPreSpeechMs
	}
	if params.MaxDurationSecs <= 0 {
		params.MaxDurationSecs = DefaultMaxDurationSecs
	}
	return &SilenceTurnAnalyzer{
		params:    params,
		lastState: Incomplete,
	}
}

// NewSilenceTurnAnalyzerWithSampleRate is like NewSilenceTurnAnalyzer but sets an initial sample rate.
func NewSilenceTurnAnalyzerWithSampleRate(params Params, sampleRate int) *SilenceTurnAnalyzer {
	a := NewSilenceTurnAnalyzer(params)
	a.sampleRate = sampleRate
	a.initSampleRate = &sampleRate
	return a
}

func (s *SilenceTurnAnalyzer) SetSampleRate(rate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initSampleRate != nil {
		s.sampleRate = *s.initSampleRate
		s.initSampleRate = nil
	}
	if rate > 0 {
		s.sampleRate = rate
	}
}

func (s *SilenceTurnAnalyzer) UpdateVADStartSecs(secs float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vadStartSecs = secs
}

// UpdateParams updates turn parameters (e.g. from VADParamsUpdateFrame).
func (s *SilenceTurnAnalyzer) UpdateParams(p Params) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.StopSecs > 0 {
		s.params.StopSecs = p.StopSecs
	}
	if p.PreSpeechMs > 0 {
		s.params.PreSpeechMs = p.PreSpeechMs
	}
	if p.MaxDurationSecs > 0 {
		s.params.MaxDurationSecs = p.MaxDurationSecs
	}
}

func (s *SilenceTurnAnalyzer) SpeechTriggered() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.speechTriggered
}

func (s *SilenceTurnAnalyzer) AppendAudio(buffer []byte, isSpeech bool) EndOfTurnState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(buffer) == 0 {
		return s.lastState
	}

	now := time.Now()
	s.buffer = append(s.buffer, bufferedChunk{at: now, data: append([]byte(nil), buffer...)})

	stopMs := s.params.StopSecs * 1000
	state := Incomplete

	if isSpeech {
		s.silenceMs = 0
		s.speechTriggered = true
		if s.speechStartTime.IsZero() {
			s.speechStartTime = now
		}
	} else {
		if s.speechTriggered && s.sampleRate > 0 {
			// chunk duration in ms: bytes/2 = samples, samples/sampleRate = seconds, *1000 = ms
			chunkMs := float64(len(buffer)/2) / (float64(s.sampleRate) / 1000)
			s.silenceMs += chunkMs
			if s.silenceMs >= stopMs {
				state = Complete
				s.clearLocked(Complete)
			}
		}
	}

	// Trim buffer to avoid unbounded growth: keep only last max_buffer_time
	maxBufferTime := (s.params.PreSpeechMs/1000 + s.vadStartSecs) + s.params.StopSecs + s.params.MaxDurationSecs
	cutoff := now.Add(-time.Duration(maxBufferTime * float64(time.Second)))
	for len(s.buffer) > 0 && s.buffer[0].at.Before(cutoff) {
		s.buffer = s.buffer[1:]
	}

	s.lastState = state
	return state
}

func (s *SilenceTurnAnalyzer) AnalyzeEndOfTurn(ctx context.Context) (EndOfTurnState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastState, nil
}

// AnalyzeEndOfTurnAsync runs the end-of-turn analysis in a separate goroutine and returns
// a channel that receives one EndOfTurnResult then closes. Callers should select on the
// channel and ctx.Done(). For the silence impl the work is trivial; the async pattern
// supports future ML-based analyzers that do heavier work in the goroutine.
func (s *SilenceTurnAnalyzer) AnalyzeEndOfTurnAsync(ctx context.Context) <-chan EndOfTurnResult {
	ch := make(chan EndOfTurnResult, 1)
	go func() {
		defer close(ch)
		s.mu.Lock()
		state := s.lastState
		s.mu.Unlock()

		// If the context is already cancelled, prefer returning a cancelled result.
		if err := ctx.Err(); err != nil {
			ch <- EndOfTurnResult{State: Incomplete, Err: err}
			return
		}

		select {
		case <-ctx.Done():
			ch <- EndOfTurnResult{State: Incomplete, Err: ctx.Err()}
		default:
			ch <- EndOfTurnResult{State: state, Err: nil}
		}
	}()
	return ch
}

func (s *SilenceTurnAnalyzer) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked(Complete)
}

func (s *SilenceTurnAnalyzer) clearLocked(turnState EndOfTurnState) {
	s.speechTriggered = turnState == Incomplete
	s.buffer = nil
	s.speechStartTime = time.Time{}
	s.silenceMs = 0
	s.lastState = Incomplete
}
