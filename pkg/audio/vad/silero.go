//go:build silero

package vad

// NOTE: This file intentionally contains a very small placeholder
// implementation so that enabling the "silero" build tag compiles, while the
// heavy ONNX wiring can be filled in later without changing the public API.

// NewSileroAnalyzer creates an Analyzer using the Silero ONNX VAD model.
func NewSileroAnalyzer(p Params, sampleRate int) (Analyzer, error) {
	// For now, defer to the stub error; a future implementation can replace
	// this with a real Silero-backed Analyzer.
	return nil, ErrSileroUnavailable
}

// sileroBackend implements confidenceBackend using the Silero model.
type sileroBackend struct {
}

func (b *sileroBackend) numFramesRequired(sampleRate int) int {
	if sampleRate == 16000 {
		return 512
	}
	return 256
}

func (b *sileroBackend) voiceConfidence(buf []byte, sampleRate int) (float64, error) {
	// This is a minimal placeholder implementation; wiring full Silero
	// inference is beyond the current scope but the interface is ready.
	// For now, treat non-empty audio as low confidence speech.
	if len(buf) == 0 {
		return 0, nil
	}
	return 0.5, nil
}

