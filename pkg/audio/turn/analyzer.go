// Package turn provides end-of-turn detection for audio conversations,
// ported from pipecat audio/turn (base_turn_analyzer + silence-based smart turn).
package turn

import "context"

// EndOfTurnState represents whether the current turn is complete.
type EndOfTurnState int

const (
	// Incomplete indicates the user is still speaking or may continue.
	Incomplete EndOfTurnState = iota
	// Complete indicates the user has finished their turn.
	Complete
)

func (s EndOfTurnState) String() string {
	switch s {
	case Complete:
		return "complete"
	case Incomplete:
		return "incomplete"
	default:
		return "unknown"
	}
}

// Params holds configuration for turn analysis (e.g. silence-based).
type Params struct {
	StopSecs        float64 // silence duration in seconds to end turn
	PreSpeechMs     float64 // milliseconds of audio before speech start to include
	MaxDurationSecs float64 // maximum segment duration in seconds
}

// EndOfTurnResult is the result of an async end-of-turn analysis.
type EndOfTurnResult struct {
	State EndOfTurnState
	Err   error
}

// Analyzer determines when a user has finished speaking (end of turn).
// It matches the Python BaseTurnAnalyzer interface.
type Analyzer interface {
	// AppendAudio adds audio and returns the current end-of-turn state.
	AppendAudio(buffer []byte, isSpeech bool) EndOfTurnState
	// AnalyzeEndOfTurn returns the last state synchronously.
	AnalyzeEndOfTurn(ctx context.Context) (EndOfTurnState, error)
	// AnalyzeEndOfTurnAsync runs analysis in a goroutine and returns a channel that receives
	// one result then closes. Caller can select on ctx.Done() and the channel. Useful for
	// analyzers that do heavier work (e.g. ML); silence impl runs the same logic in a goroutine.
	AnalyzeEndOfTurnAsync(ctx context.Context) <-chan EndOfTurnResult
	// SpeechTriggered reports whether speech has been detected and analysis is active.
	SpeechTriggered() bool
	// SetSampleRate sets the sample rate for audio processing.
	SetSampleRate(rate int)
	// Clear resets the analyzer to initial state.
	Clear()
	// UpdateVADStartSecs updates the VAD start trigger time (for pre-speech padding).
	UpdateVADStartSecs(secs float64)
}
