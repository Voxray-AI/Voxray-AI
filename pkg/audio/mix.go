// Package audio provides mix and interleave helpers for 16-bit mono PCM.
package audio

// MixMono mixes two 16-bit little-endian mono PCM buffers by averaging samples.
// If lengths differ, the shorter buffer is padded with zeros to match the longer.
// Returns a new slice of length max(len(user), len(bot)).
func MixMono(user, bot []byte) []byte {
	n := len(user)
	if len(bot) > n {
		n = len(bot)
	}
	if n == 0 {
		return nil
	}
	out := make([]byte, n)
	for i := 0; i+1 < n; i += 2 {
		var u, b int16
		if i+1 < len(user) {
			u = int16(uint16(user[i]) | uint16(user[i+1])<<8)
		}
		if i+1 < len(bot) {
			b = int16(uint16(bot[i]) | uint16(bot[i+1])<<8)
		}
		mixed := (int32(u) + int32(b)) / 2
		if mixed > 32767 {
			mixed = 32767
		}
		if mixed < -32768 {
			mixed = -32768
		}
		out[i] = byte(mixed)
		out[i+1] = byte(mixed >> 8)
	}
	return out
}

// InterleaveStereo interleaves two 16-bit mono buffers as left, right, left, right, ...
// If lengths differ, the shorter channel is padded with zeros so both have the same length.
// Returns a new slice of length 4*max(samples in left, samples in right).
func InterleaveStereo(left, right []byte) []byte {
	leftSamples := len(left) / 2
	rightSamples := len(right) / 2
	samples := leftSamples
	if rightSamples > samples {
		samples = rightSamples
	}
	if samples == 0 {
		return nil
	}
	out := make([]byte, samples*4)
	for s := 0; s < samples; s++ {
		if 2*s+1 < len(left) {
			out[4*s] = left[2*s]
			out[4*s+1] = left[2*s+1]
		}
		if 2*s+1 < len(right) {
			out[4*s+2] = right[2*s]
			out[4*s+3] = right[2*s+1]
		}
	}
	return out
}
