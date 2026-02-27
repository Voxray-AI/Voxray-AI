package vad

import (
	"math"

	"voila-go/pkg/audio"
)

// Detector decides whether a given audio frame contains speech.
// Implementations should assume 16-bit PCM mono by default.
type Detector interface {
	IsSpeech(f audio.Frame) (bool, error)
}

// EnergyDetector is a very simple VAD that classifies speech based on RMS energy.
// It is intended as a baseline implementation and can be replaced with a more
// sophisticated detector later (e.g. ML-based VAD).
type EnergyDetector struct {
	// Threshold is the minimum RMS energy (0..1, relative to int16 max) to be
	// considered speech.
	Threshold float64
}

// NewEnergyDetector creates an EnergyDetector with a reasonable default threshold.
func NewEnergyDetector() *EnergyDetector {
	return &EnergyDetector{Threshold: 0.02} // small but above typical background noise
}

// IsSpeech returns true if the frame's RMS energy exceeds the configured threshold.
func (e *EnergyDetector) IsSpeech(f audio.Frame) (bool, error) {
	if len(f.Data) < 2 {
		return false, nil
	}
	// Interpret bytes as little-endian int16 samples.
	var sumSquares float64
	sampleCount := 0
	for i := 0; i+1 < len(f.Data); i += 2 {
		sample := int16(uint16(f.Data[i]) | uint16(f.Data[i+1])<<8)
		fs := float64(sample) / 32768.0
		sumSquares += fs * fs
		sampleCount++
	}
	if sampleCount == 0 {
		return false, nil
	}
	rms := math.Sqrt(sumSquares / float64(sampleCount))
	return rms >= e.Threshold, nil
}

