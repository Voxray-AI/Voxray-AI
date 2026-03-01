//go:build !silero

package vad

// This file provides a stub SileroAnalyzer for builds that do not include the
// Silero VAD dependency. When the "silero" build tag is enabled, silero.go
// will provide a real implementation.

import "errors"

// ErrSileroUnavailable is returned when Silero support cannot be used.
var ErrSileroUnavailable = errors.New("silero VAD is not available in this build")

// NewSileroAnalyzer returns (nil, ErrSileroUnavailable) when Silero support is
// not compiled in.
func NewSileroAnalyzer(p Params, sampleRate int) (Analyzer, error) {
	return nil, ErrSileroUnavailable
}

