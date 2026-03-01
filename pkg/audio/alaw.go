// Package audio provides A-law (G.711 PCMA) encode/decode for 16-bit mono PCM.
package audio

// alawDecodeTable maps A-law byte to 16-bit linear (ITU-T G.711).
var alawDecodeTable = [256]int16{
	-5504, -5248, -6016, -5760, -4480, -4224, -4992, -4736,
	-7552, -7296, -8064, -7808, -6528, -6272, -7040, -6784,
	-2752, -2624, -3008, -2880, -2240, -2112, -2496, -2368,
	-3776, -3648, -4032, -3904, -3264, -3136, -3520, -3392,
	-22016, -20992, -24064, -23040, -17920, -16896, -19968, -18944,
	-30208, -29184, -32256, -31232, -26112, -25088, -28160, -27136,
	-11008, -10496, -12032, -11520, -8960, -8448, -9984, -9472,
	-15104, -14592, -16128, -15616, -13056, -12544, -14080, -13568,
	-344, -328, -376, -360, -280, -264, -312, -296,
	-472, -456, -504, -488, -408, -392, -440, -424,
	-88, -72, -120, -104, -24, -8, -56, -40,
	-216, -200, -248, -232, -152, -136, -184, -168,
	-1376, -1312, -1504, -1440, -1120, -1056, -1248, -1184,
	-1888, -1824, -2016, -1952, -1632, -1568, -1760, -1696,
	-688, -656, -752, -720, -560, -528, -624, -592,
	-944, -912, -1008, -976, -816, -784, -880, -848,
	5504, 5248, 6016, 5760, 4480, 4224, 4992, 4736,
	7552, 7296, 8064, 7808, 6528, 6272, 7040, 6784,
	2752, 2624, 3008, 2880, 2240, 2112, 2496, 2368,
	3776, 3648, 4032, 3904, 3264, 3136, 3520, 3392,
	22016, 20992, 24064, 23040, 17920, 16896, 19968, 18944,
	30208, 29184, 32256, 31232, 26112, 25088, 28160, 27136,
	11008, 10496, 12032, 11520, 8960, 8448, 9984, 9472,
	15104, 14592, 16128, 15616, 13056, 12544, 14080, 13568,
	344, 328, 376, 360, 280, 264, 312, 296,
	472, 456, 504, 488, 408, 392, 440, 424,
	88, 72, 120, 104, 24, 8, 56, 40,
	216, 200, 248, 232, 152, 136, 184, 168,
	1376, 1312, 1504, 1440, 1120, 1056, 1248, 1184,
	1888, 1824, 2016, 1952, 1632, 1568, 1760, 1696,
	688, 656, 752, 720, 560, 528, 624, 592,
	944, 912, 1008, 976, 816, 784, 880, 848,
}

// EncodeALaw converts 16-bit little-endian PCM to A-law (PCMA) bytes.
// len(pcm) must be even; output length is len(pcm)/2.
func EncodeALaw(pcm []byte) []byte {
	n := len(pcm) / 2
	if n == 0 {
		return nil
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		lo := uint16(pcm[i*2])
		hi := uint16(pcm[i*2+1])
		sample := int16(lo | hi<<8)
		out[i] = encodeALawSample(sample)
	}
	return out
}

func encodeALawSample(sample int16) uint8 {
	const maxExp = 7
	sign := uint8(0)
	if sample < 0 {
		sign = 0x80
		sample = -sample
		if sample < 0 {
			sample = 0x7FFF
		}
	}
	if sample < 8 {
		return ^(sign | uint8(sample<<4))
	}
	exp := 0
	for sample >= 256 && exp < maxExp {
		exp++
		sample >>= 1
	}
	mantissa := (sample >> 4) & 0x0F
	return ^(sign | (uint8(exp) << 4) | uint8(mantissa))
}

// DecodeALaw converts A-law (PCMA) bytes to 16-bit little-endian PCM.
// Output length is len(alaw)*2.
func DecodeALaw(alaw []byte) []byte {
	n := len(alaw)
	if n == 0 {
		return nil
	}
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		s := alawDecodeTable[alaw[i]]
		out[i*2] = byte(s)
		out[i*2+1] = byte(s >> 8)
	}
	return out
}
