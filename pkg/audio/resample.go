// Package audio provides optional sample rate conversion (resample) for 16-bit mono PCM.
package audio

// Resample16Mono converts 16-bit mono PCM from inRate to outRate using linear interpolation.
// It performs a sample rate conversion to match the target service's requirements.
// in and out can be the same slice if inRate == outRate (no-op). Otherwise out must have capacity
// for the resampled length: len(out) >= len(in) * outRate / inRate (rounded up).
func Resample16Mono(in []byte, inRate, outRate int, out []byte) []byte {
	if inRate <= 0 || outRate <= 0 {
		return out
	}
	if inRate == outRate {
		return append(out[:0], in...)
	}
	// 16-bit = 2 bytes per sample
	inSamples := len(in) / 2
	outLen := (inSamples * outRate) / inRate
	if outLen < 0 {
		outLen = 0
	}
	if cap(out) < outLen*2 {
		out = make([]byte, 0, outLen*2)
	} else {
		out = out[:0]
	}
	for i := 0; i < outLen; i++ {
		// Map output sample index to input float position
		pos := float64(i) * float64(inRate) / float64(outRate)
		idx := int(pos)
		frac := pos - float64(idx)
		if idx >= inSamples-1 {
			idx = inSamples - 1
			frac = 0
		}
		if idx < 0 {
			idx = 0
		}
		// Linear interpolation between in[idx] and in[idx+1] (16-bit LE)
		lo := int16(uint16(in[idx*2]) | uint16(in[idx*2+1])<<8)
		hi := lo
		if idx+1 < inSamples {
			hi = int16(uint16(in[(idx+1)*2]) | uint16(in[(idx+1)*2+1])<<8)
		}
		sample := float64(lo)*(1-frac) + float64(hi)*frac
		s := int16(sample)
		out = append(out, byte(s), byte(s>>8))
	}
	return out
}

// Resample16MonoAlloc returns a new slice with 16-bit mono PCM resampled from inRate to outRate.
// It is a convenience wrapper around Resample16Mono that handles allocation.
func Resample16MonoAlloc(in []byte, inRate, outRate int) []byte {
	var out []byte
	return Resample16Mono(in, inRate, outRate, out)
}
