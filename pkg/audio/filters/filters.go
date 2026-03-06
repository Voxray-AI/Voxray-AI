package filters

import (
	"encoding/binary"
)

// Filter processes PCM16 audio (little-endian) and returns filtered audio.
// Callers should treat the returned slice as the canonical output; an
// implementation may choose to return the original slice when no change is
// needed or a new slice when modification is required.
type Filter interface {
	// Apply runs the filter over the provided audio buffer.
	// audio is 16-bit PCM little-endian mono or multi-channel.
	Apply(audio []byte, sampleRate, channels int) []byte
}

// Chain composes multiple filters and applies them in sequence.
type Chain struct {
	filters []Filter
}

// NewChain constructs a Chain from the provided filters. Nil filters are
// ignored during Apply.
func NewChain(fs ...Filter) *Chain {
	return &Chain{filters: fs}
}

// Apply runs all filters in order. When no filters are configured, the input
// slice is returned unchanged.
func (c *Chain) Apply(audio []byte, sampleRate, channels int) []byte {
	if c == nil || len(c.filters) == 0 || len(audio) == 0 {
		return audio
	}
	out := audio
	for _, f := range c.filters {
		if f == nil {
			continue
		}
		out = f.Apply(out, sampleRate, channels)
		if len(out) == 0 {
			// Allow a filter to drop audio entirely if desired.
			break
		}
	}
	return out
}

// GainFilter scales audio samples by a constant gain factor.
// Gain values >1.0 amplify, values between 0 and 1.0 attenuate.
type GainFilter struct {
	Gain float64
}

// NewGainFilter constructs a GainFilter. When gain is zero or negative it
// falls back to 1.0 (no-op).
func NewGainFilter(gain float64) *GainFilter {
	if gain <= 0 {
		gain = 1.0
	}
	return &GainFilter{Gain: gain}
}

func (g *GainFilter) Apply(audio []byte, _ int, _ int) []byte {
	// PCM16 requires an even number of bytes; reject malformed input to avoid
	// silently corrupting the tail of the buffer.
	if len(audio) == 0 {
		return audio
	}
	if len(audio)%2 != 0 {
		return nil
	}
	if g == nil || g.Gain == 1.0 {
		return audio
	}
	// Work on a copy to avoid mutating upstream buffers.
	out := make([]byte, len(audio))
	for i := 0; i+1 < len(audio); i += 2 {
		// Little-endian 16-bit PCM.
		sample := int16(binary.LittleEndian.Uint16(audio[i:]))
		scaled := int(float64(sample) * g.Gain)
		if scaled > 32767 {
			scaled = 32767
		} else if scaled < -32768 {
			scaled = -32768
		}
		binary.LittleEndian.PutUint16(out[i:], uint16(int16(scaled)))
	}
	return out
}

