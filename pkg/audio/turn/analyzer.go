// Package turn provides end-of-turn detection for audio conversations
// (base turn analyzer + silence-based smart turn).
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
//
// Implementations should keep AppendAudio cheap and non-blocking; it is called
// for every incoming audio chunk to update lightweight internal state and
// bookkeeping. AnalyzeEndOfTurn returns a synchronous snapshot of the current
// end-of-turn state derived from that internal state.
//
// AnalyzeEndOfTurnAsync exposes the same information via a goroutine-backed
// channel. Implementations may perform heavier work inside the goroutine (for
// example, ML-based models), but they must:
//   - Send exactly one EndOfTurnResult on the returned channel, then close it.
//   - Be safe to call repeatedly; implementations should internally deduplicate
//     or cache work if repeated async calls would otherwise be expensive.
//   - Respect ctx.Done(): if the context is cancelled before a result is sent,
//     they should send a non-Complete state with Err set to ctx.Err().
type Analyzer interface {
	// AppendAudio adds audio and returns the current end-of-turn state. It must
	// be fast and non-blocking; callers may invoke this on every audio frame.
	AppendAudio(buffer []byte, isSpeech bool) EndOfTurnState
	// AnalyzeEndOfTurn returns the last state synchronously as a cheap snapshot.
	AnalyzeEndOfTurn(ctx context.Context) (EndOfTurnState, error)
	// AnalyzeEndOfTurnAsync runs analysis in a goroutine and returns a channel
	// that receives one EndOfTurnResult then closes. Callers should select on
	// ctx.Done() and the channel. This is intended for analyzers that do
	// heavier work (e.g. ML); the silence implementation runs the same logic
	// in a goroutine but still follows the same contract.
	AnalyzeEndOfTurnAsync(ctx context.Context) <-chan EndOfTurnResult
	// SpeechTriggered reports whether speech has been detected and analysis is active.
	SpeechTriggered() bool
	// SetSampleRate sets the sample rate for audio processing.
	SetSampleRate(rate int)
	// Clear resets the analyzer to initial state.
	Clear()
	// UpdateVADStartSecs updates the VAD start trigger time (for pre-speech padding).
	UpdateVADStartSecs(secs float64)
	// UpdateParams updates turn parameters (e.g. StopSecs for IVR mode). Used when receiving VADParamsUpdateFrame.
	UpdateParams(p Params)
}
