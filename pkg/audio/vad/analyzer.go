package vad

import (
	"math"
	"sync"

	"voila-go/pkg/audio"
)

// State represents the high-level VAD state.
type State int

const (
	StateQuiet State = iota + 1
	StateStarting
	StateSpeaking
	StateStopping
)

// Params configures Voice Activity Detection behaviour.
type Params struct {
	// Confidence is the minimum voice confidence (0..1) required to treat audio
	// as speech.
	Confidence float64
	// StartSecs is how long speech must be continuously detected before we move
	// from Quiet->Starting->Speaking.
	StartSecs float64
	// StopSecs is how long silence must be observed before we move from
	// Speaking->Stopping->Quiet.
	StopSecs float64
	// MinVolume is the minimum smoothed volume (0..1) required to treat audio
	// as speech. This is a second gate in addition to Confidence.
	MinVolume float64
}

const (
	defaultConfidence = 0.7
	defaultStartSecs  = 0.2
	defaultStopSecs   = 0.2
	defaultMinVolume  = 0.6
)

// normalize ensures zero-values pick reasonable defaults.
func (p Params) normalize() Params {
	if p.Confidence <= 0 {
		p.Confidence = defaultConfidence
	}
	if p.StartSecs <= 0 {
		p.StartSecs = defaultStartSecs
	}
	if p.StopSecs <= 0 {
		p.StopSecs = defaultStopSecs
	}
	if p.MinVolume <= 0 {
		p.MinVolume = defaultMinVolume
	}
	return p
}

// Analyzer is the high-level VAD interface, similar to the Python VADAnalyzer.
//
// Implementations are safe for concurrent use from a single audio stream (the
// usual case in this project); calls are internally serialised via a mutex.
type Analyzer interface {
	// SetSampleRate configures the audio sample rate. Must be called before
	// Analyze; implementations may clamp/validate as needed.
	SetSampleRate(sampleRate int)
	// SetParams updates the VAD parameters. Zero-values pick sensible defaults.
	SetParams(params Params)
	// Params returns the current VAD parameters.
	Params() Params
	// Analyze consumes audio for this stream, updates internal state, and
	// returns the current state, last confidence, and last smoothed volume.
	//
	// Audio is expected to be 16‑bit PCM mono, matching audio.Frame/Data.
	Analyze(buf []byte) (State, float64, float64, error)
}

// confidenceBackend is implemented by concrete analyzers that know how to
// compute voice confidence for a fixed-size audio window.
type confidenceBackend interface {
	numFramesRequired(sampleRate int) int
	voiceConfidence(buf []byte, sampleRate int) (float64, error)
}

// baseAnalyzer implements the generic VAD state machine and buffering logic.
// Concrete analyzers embed it and provide a confidenceBackend.
type baseAnalyzer struct {
	mu sync.Mutex

	params      Params
	sampleRate  int
	numChannels int

	backend confidenceBackend

	// derived sizing
	vadFrames        int
	vadFramesNumByte int

	// state
	buf              []byte
	state            State
	startFrames      int
	stopFrames       int
	startingCount    int
	stoppingCount    int
	prevVolume       float64
	lastConfidence   float64
	lastVolume       float64
}

func newBaseAnalyzer(b confidenceBackend) *baseAnalyzer {
	return &baseAnalyzer{
		params:      (Params{}).normalize(),
		numChannels: 1,
		backend:     b,
		state:       StateQuiet,
	}
}

func (a *baseAnalyzer) SetSampleRate(sampleRate int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if sampleRate <= 0 {
		a.sampleRate = 0
		return
	}
	a.sampleRate = sampleRate
	a.recompute()
}

func (a *baseAnalyzer) SetParams(p Params) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.params = p.normalize()
	a.recompute()
}

func (a *baseAnalyzer) Params() Params {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.params
}

func (a *baseAnalyzer) recompute() {
	if a.sampleRate <= 0 || a.backend == nil {
		return
	}
	a.vadFrames = a.backend.numFramesRequired(a.sampleRate)
	if a.vadFrames <= 0 {
		a.vadFrames = int(0.01 * float64(a.sampleRate)) // fallback 10ms
	}
	a.vadFramesNumByte = a.vadFrames * a.numChannels * 2

	vadFramesPerSec := float64(a.vadFrames) / float64(a.sampleRate)
	if vadFramesPerSec <= 0 {
		vadFramesPerSec = 0.01
	}
	a.startFrames = int(math.Round(a.params.StartSecs / vadFramesPerSec))
	a.stopFrames = int(math.Round(a.params.StopSecs / vadFramesPerSec))
	a.startingCount = 0
	a.stoppingCount = 0
	if a.state == 0 {
		a.state = StateQuiet
	}
}

// Analyze implements the main VAD loop, similar to Python VADAnalyzer._run_analyzer.
func (a *baseAnalyzer) Analyze(buf []byte) (State, float64, float64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.sampleRate <= 0 || a.backend == nil || a.vadFramesNumByte <= 0 {
		return a.state, a.lastConfidence, a.lastVolume, nil
	}

	a.buf = append(a.buf, buf...)
	numRequired := a.vadFramesNumByte
	if len(a.buf) < numRequired {
		return a.state, a.lastConfidence, a.lastVolume, nil
	}

	for len(a.buf) >= numRequired {
		window := a.buf[:numRequired]
		a.buf = a.buf[numRequired:]

		conf, err := a.backend.voiceConfidence(window, a.sampleRate)
		if err != nil {
			// treat as silence but keep previous state
			conf = 0
		}
		vol := calculateVolume(window)
		vol = expSmoothing(vol, a.prevVolume, 0.2)
		a.prevVolume = vol

		a.lastConfidence = conf
		a.lastVolume = vol

		speaking := conf >= a.params.Confidence && vol >= a.params.MinVolume

		if speaking {
			switch a.state {
			case StateQuiet:
				a.state = StateStarting
				a.startingCount = 1
				a.stoppingCount = 0
			case StateStarting:
				a.startingCount++
			case StateStopping:
				a.state = StateSpeaking
				a.stoppingCount = 0
			}
		} else {
			switch a.state {
			case StateStarting:
				a.state = StateQuiet
				a.startingCount = 0
			case StateSpeaking:
				a.state = StateStopping
				a.stoppingCount = 1
			case StateStopping:
				a.stoppingCount++
			}
		}

		if a.state == StateStarting && a.startingCount >= a.startFrames && a.startFrames > 0 {
			a.state = StateSpeaking
			a.startingCount = 0
		}

		if a.state == StateStopping && a.stoppingCount >= a.stopFrames && a.stopFrames > 0 {
			a.state = StateQuiet
			a.stoppingCount = 0
		}
	}

	return a.state, a.lastConfidence, a.lastVolume, nil
}

// calculateVolume returns a simple RMS-based loudness normalised to [0,1].
func calculateVolume(buf []byte) float64 {
	if len(buf) < 2 {
		return 0
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
		return 0
	}
	rms := math.Sqrt(sumSquares / float64(samples))
	if rms < 0 {
		return 0
	}
	if rms > 1 {
		return 1
	}
	return rms
}

// expSmoothing applies exponential smoothing to a value.
func expSmoothing(value, prev, factor float64) float64 {
	return prev + factor*(value-prev)
}

// AnalyzerDetector bridges a VAD Analyzer to the existing Detector interface.
type AnalyzerDetector struct {
	Analyzer Analyzer
}

// IsSpeech reports true when the underlying analyzer is in StateSpeaking.
func (d *AnalyzerDetector) IsSpeech(f audio.Frame) (bool, error) {
	if d == nil || d.Analyzer == nil {
		return false, nil
	}
	state, _, _, err := d.Analyzer.Analyze(f.Data)
	if err != nil {
		return false, err
	}
	return state == StateSpeaking, nil
}

