package audio

import "time"

// Frame represents a chunk of PCM audio in the Voxray system.
// It is a lightweight helper around raw bytes, separate from pipeline frames.
// Audio is expected to be 16-bit PCM little-endian.
type Frame struct {
	Data        []byte
	SampleRate  int
	NumChannels int
	Timestamp   time.Time
}

// Stream represents a bidirectional audio source or sink.
// Implementations may wrap microphones, speakers, files, or network streams.
type Stream interface {
	Read() (Frame, error)
	Write(Frame) error
	Close() error
}

