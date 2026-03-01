package vad

import "testing"

func TestParamsNormalizeDefaults(t *testing.T) {
	p := Params{}
	p2 := p.normalize()
	if p2.Confidence <= 0 || p2.StartSecs <= 0 || p2.StopSecs <= 0 || p2.MinVolume <= 0 {
		t.Fatalf("expected normalized params to have positive values, got %+v", p2)
	}
}

func TestBaseAnalyzerStateTransitions(t *testing.T) {
	backend := &EnergyAnalyzerBackend{Threshold: 0.02}
	a := newBaseAnalyzer(backend)
	a.SetSampleRate(16000)
	a.SetParams(Params{
		// Use low thresholds so a single loud window is enough to move
		// the state machine away from Quiet.
		Confidence: 0.05,
		StartSecs:  0.0,
		StopSecs:   0.0,
		MinVolume:  0.1,
	})

	// Generate a buffer large and loud enough to be treated as speech by the
	// energy backend and to satisfy the window-size requirement.
	// For 16 kHz, EnergyAnalyzerBackend uses a 10 ms window, i.e. 160 frames
	// or 320 bytes for mono 16‑bit PCM.
	speech := make([]byte, 320)
	for i := 0; i < len(speech); i += 2 {
		// Max positive 16‑bit sample (0x7FFF) in little‑endian.
		speech[i] = 0xFF
		speech[i+1] = 0x7F
	}
	state, _, _, err := a.Analyze(speech)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if state == StateQuiet {
		t.Fatalf("expected state to move away from Quiet with speech-like buffer")
	}
}

