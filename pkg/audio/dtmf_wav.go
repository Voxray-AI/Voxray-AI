// Package audio: DTMF tone generation and WAV writing for 16-bit mono PCM.

package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// DTMF frequencies (low, high) Hz per ITU-T Q.23.
var dtmfFreqs = map[string][2]float64{
	"1": {697, 1209}, "2": {697, 1336}, "3": {697, 1477},
	"4": {770, 1209}, "5": {770, 1336}, "6": {770, 1477},
	"7": {852, 1209}, "8": {852, 1336}, "9": {852, 1477},
	"0": {941, 1336}, "star": {941, 1209}, "pound": {941, 1477},
}

// GenerateDTMFPCM generates 16-bit little-endian mono PCM for a DTMF key.
// key: "0"-"9", "star", "pound". toneDuration and gapDuration are in seconds.
func GenerateDTMFPCM(sampleRate int, key string, toneDuration, gapDuration float64) ([]byte, error) {
	freqs, ok := dtmfFreqs[key]
	if !ok {
		return nil, fmt.Errorf("unknown DTMF key: %q", key)
	}
	toneSamples := int(float64(sampleRate) * toneDuration)
	gapSamples := int(float64(sampleRate) * gapDuration)
	total := toneSamples + gapSamples
	pcm := make([]byte, total*2)
	const twoPi = 2 * math.Pi
	// Mix two sine waves; scale so sum stays in [-1,1] then to int16
	amp := 0.5
	for i := 0; i < toneSamples; i++ {
		t := float64(i) / float64(sampleRate)
		sample := amp*math.Sin(twoPi*freqs[0]*t) + amp*math.Sin(twoPi*freqs[1]*t)
		if sample > 1 {
			sample = 1
		}
		if sample < -1 {
			sample = -1
		}
		s := int16(sample * 32767)
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(s))
	}
	for i := toneSamples; i < total; i++ {
		binary.LittleEndian.PutUint16(pcm[i*2:], 0)
	}
	return pcm, nil
}

// WritePCM16MonoWAV writes 16-bit mono PCM to a WAV file.
func WritePCM16MonoWAV(path string, pcm []byte, sampleRate int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	const bitsPerSample = 16
	numChannels := 1
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := uint32(len(pcm))
	headerLen := 44
	buf := make([]byte, headerLen+len(pcm))
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], 36+dataSize)
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(numChannels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], dataSize)
	copy(buf[44:], pcm)
	return os.WriteFile(path, buf, 0o644)
}
