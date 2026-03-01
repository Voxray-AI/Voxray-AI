// Package audio provides μ-law (G.711 PCMU) encode/decode for 16-bit mono PCM.
package audio

// ulawDecodeTable maps μ-law byte to 16-bit linear (ITU-T G.711).
var ulawDecodeTable = [256]int16{
	-32124, -31100, -30076, -29052, -28028, -27004, -25980, -24956,
	-23932, -22908, -21884, -20860, -19836, -18812, -17788, -16764,
	-15996, -15484, -14972, -14460, -13948, -13436, -12924, -12412,
	-11900, -11388, -10876, -10364, -9852, -9340, -8828, -8316,
	-7932, -7676, -7420, -7164, -6908, -6652, -6396, -6140,
	-5884, -5628, -5372, -5116, -4860, -4604, -4348, -4092,
	-3900, -3772, -3644, -3516, -3388, -3260, -3132, -3004,
	-2876, -2748, -2620, -2492, -2364, -2236, -2108, -1980,
	-1884, -1820, -1756, -1692, -1628, -1564, -1500, -1436,
	-1372, -1308, -1244, -1180, -1116, -1052, -988, -924,
	-876, -844, -812, -780, -748, -716, -684, -652,
	-620, -588, -556, -524, -492, -460, -428, -396,
	-372, -356, -340, -324, -308, -292, -276, -260,
	-244, -228, -212, -196, -180, -164, -148, -132,
	-120, -112, -104, -96, -88, -80, -72, -64,
	-56, -48, -40, -32, -24, -16, -8, 0,
	32124, 31100, 30076, 29052, 28028, 27004, 25980, 24956,
	23932, 22908, 21884, 20860, 19836, 18812, 17788, 16764,
	15996, 15484, 14972, 14460, 13948, 13436, 12924, 12412,
	11900, 11388, 10876, 10364, 9852, 9340, 8828, 8316,
	7932, 7676, 7420, 7164, 6908, 6652, 6396, 6140,
	5884, 5628, 5372, 5116, 4860, 4604, 4348, 4092,
	3900, 3772, 3644, 3516, 3388, 3260, 3132, 3004,
	2876, 2748, 2620, 2492, 2364, 2236, 2108, 1980,
	1884, 1820, 1756, 1692, 1628, 1564, 1500, 1436,
	1372, 1308, 1244, 1180, 1116, 1052, 988, 924,
	876, 844, 812, 780, 748, 716, 684, 652,
	620, 588, 556, 524, 492, 460, 428, 396,
	372, 356, 340, 324, 308, 292, 276, 260,
	244, 228, 212, 196, 180, 164, 148, 132,
	120, 112, 104, 96, 88, 80, 72, 64,
	56, 48, 40, 32, 24, 16, 8, 0,
}

// EncodeULaw converts 16-bit little-endian PCM to μ-law (PCMU) bytes.
// len(pcm) must be even; output length is len(pcm)/2.
func EncodeULaw(pcm []byte) []byte {
	n := len(pcm) / 2
	if n == 0 {
		return nil
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		lo := uint16(pcm[i*2])
		hi := uint16(pcm[i*2+1])
		sample := int16(lo | hi<<8)
		out[i] = encodeULawSample(sample)
	}
	return out
}

func encodeULawSample(sample int16) uint8 {
	const bias = 33
	const maxExp = 7
	sign := uint8(0)
	if sample < 0 {
		sign = 0x80
		sample = -sample
		if sample < 0 {
			sample = 0x7FFF
		}
	}
	sample32 := int32(sample) + bias
	if sample32 > 0x7FFF {
		sample32 = 0x7FFF
	}
	mag := uint16(sample32)
	exp := uint8(0)
	for (64 << exp) <= mag && exp < maxExp {
		exp++
	}
	mantissa := (sample32 >> (exp + 3)) & 0x0F
	return ^(sign | (exp << 4) | uint8(mantissa))
}

// DecodeULaw converts μ-law (PCMU) bytes to 16-bit little-endian PCM.
// Output length is len(ulaw)*2.
func DecodeULaw(ulaw []byte) []byte {
	n := len(ulaw)
	if n == 0 {
		return nil
	}
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		s := ulawDecodeTable[ulaw[i]]
		out[i*2] = byte(s)
		out[i*2+1] = byte(s >> 8)
	}
	return out
}
