// Package audio provides minimal audio processing utilities for the Voila system,
// focusing on 16-bit PCM mono audio formats used by STT and TTS services.
package audio

// PCM16MonoNumFrames calculates the number of audio frames in a PCM16 mono buffer.
// Since each frame is 16 bits (2 bytes), the number of frames is total bytes divided by 2.
func PCM16MonoNumFrames(bytes []byte) int {
	return len(bytes) / 2
}

// Default audio configuration constants for the Voila system.
const (
	// DefaultInSampleRate is the typical sample rate for STT input (16kHz).
	DefaultInSampleRate = 16000
	// DefaultOutSampleRate is the typical sample rate for TTS output (24kHz).
	DefaultOutSampleRate = 24000
)
