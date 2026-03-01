package vad

import (
	"math"

	"voxray-go/pkg/audio"
)

// Detector decides whether a given audio frame contains speech.
// Implementations should assume 16-bit PCM mono by default.
// SetSampleRate configures the detector for the given pipeline input rate (e.g. 16000).
type Detector interface {
	IsSpeech(f audio.Frame) (bool, error)
	SetSampleRate(sampleRate int)
}

// EnergyAnalyzerBackend is a simple confidence backend based on RMS energy.
type EnergyAnalyzerBackend struct {
	Threshold float64
}

// numFramesRequired returns a 10ms window at the given sample rate.
func (b *EnergyAnalyzerBackend) numFramesRequired(sampleRate int) int {
	if sampleRate <= 0 {
		return 160
	}
	return sampleRate / 100
}

// voiceConfidence returns an energy-based confidence in [0,1].
func (b *EnergyAnalyzerBackend) voiceConfidence(buf []byte, _ int) (float64, error) {
	if len(buf) < 2 {
		return 0, nil
	}
	var sumSquares float64
	samples := 0
	for i := 0; i+1 < len(buf); i += 2 {
		sample := int16(uint16(buf[i]) | uint16(buf[i+1])<<8)
		fs := float64(sample) / 32768.0
		sumSquares += fs * fs
		samples++
	}
	if samples == 0 {
		return 0, nil
	}
	rms := math.Sqrt(sumSquares / float64(samples))
	// Map RMS threshold to confidence: below threshold -> 0, above -> clamp to 1.
	if b.Threshold <= 0 {
		return rms, nil
	}
	conf := rms / b.Threshold
	if conf > 1 {
		conf = 1
	}
	if conf < 0 {
		conf = 0
	}
	return conf, nil
}

// NewEnergyAnalyzer returns an Analyzer that uses EnergyAnalyzerBackend.
func NewEnergyAnalyzer(p Params) Analyzer {
	p = p.normalize()
	thresh := p.Threshold
	if thresh <= 0 {
		thresh = defaultThreshold
	}
	backend := &EnergyAnalyzerBackend{
		Threshold: thresh,
	}
	a := newBaseAnalyzer(backend)
	a.params = p
	return a
}

// EnergyDetector is preserved for compatibility; it wraps an AnalyzerDetector
// using an internal EnergyAnalyzer.
type EnergyDetector struct {
	Threshold float64
	detector  *AnalyzerDetector
}

// NewEnergyDetector creates an EnergyDetector with a reasonable default threshold.
func NewEnergyDetector() *EnergyDetector {
	return NewEnergyDetectorWithParams(Params{})
}

// NewEnergyDetectorWithParams allows callers to override Params; zero-values pick defaults.
func NewEnergyDetectorWithParams(p Params) *EnergyDetector {
	p = p.normalize()
	thresh := p.Threshold
	if thresh <= 0 {
		thresh = defaultThreshold
	}
	a := NewEnergyAnalyzer(p)
	// Use a default sample rate; callers that care can call SetSampleRate on
	// the underlying Analyzer via type assertions if needed, but the usual
	// usage path (TurnProcessor) only relies on IsSpeech().
	a.SetSampleRate(audio.DefaultInSampleRate)
	return &EnergyDetector{
		Threshold: thresh,
		detector:  &AnalyzerDetector{Analyzer: a},
	}
}

// IsSpeech delegates to the internal AnalyzerDetector.
func (e *EnergyDetector) IsSpeech(f audio.Frame) (bool, error) {
	if e == nil || e.detector == nil {
		return false, nil
	}
	return e.detector.IsSpeech(f)
}

// SetSampleRate sets the sample rate on the internal analyzer (e.g. 16000 for pipeline input).
func (e *EnergyDetector) SetSampleRate(sampleRate int) {
	if e != nil && e.detector != nil && e.detector.Analyzer != nil {
		e.detector.Analyzer.SetSampleRate(sampleRate)
	}
}

