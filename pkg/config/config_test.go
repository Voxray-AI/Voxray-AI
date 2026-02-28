package config

import "testing"

func TestSTTProvider(t *testing.T) {
	c := &Config{Provider: "openai"}
	if got := c.STTProvider(); got != "openai" {
		t.Errorf("STTProvider() = %q, want openai", got)
	}
	c.SttProvider = "groq"
	if got := c.STTProvider(); got != "groq" {
		t.Errorf("STTProvider() = %q, want groq", got)
	}
}

func TestVADParams(t *testing.T) {
	c := &Config{}
	p := c.VADParams()
	if p.Confidence != 0 || p.StartSecs != 0 || p.StopSecs != 0 || p.MinVolume != 0 {
		t.Errorf("VADParams() zero config should give zero struct: %+v", p)
	}
	c.VADConfidence = 0.7
	c.VADStartSecsVAD = 0.2
	c.VADStopSecs = 0.3
	c.VADMinVolume = 0.5
	p = c.VADParams()
	if p.Confidence != 0.7 || p.StartSecs != 0.2 || p.StopSecs != 0.3 || p.MinVolume != 0.5 {
		t.Errorf("VADParams() = %+v", p)
	}
}

func TestTurnEnabled(t *testing.T) {
	c := &Config{}
	if c.TurnEnabled() {
		t.Error("TurnEnabled() should be false when TurnDetection is empty")
	}
	c.TurnDetection = "silence"
	if !c.TurnEnabled() {
		t.Error("TurnEnabled() should be true when TurnDetection is silence")
	}
}
