// Package recording provides recording of voice sessions to local files and S3 upload.
package recording

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ConversationRecorder records mixed audio for a single call/session.
// Implementations are expected to be safe for use from a single goroutine.
type ConversationRecorder interface {
	// WriteSamples appends raw PCM 16-bit mono samples at the given sample rate.
	WriteSamples(samples []int16, sampleRate int) error
	// Close finalizes the recording and returns info about the created file.
	Close() (LocalFileInfo, error)
}

// LocalFileInfo describes a finalized local recording file.
type LocalFileInfo struct {
	Path      string
	StartedAt time.Time
	EndedAt   time.Time
	Format    string
}

// fileRecorder is a simple WAV recorder writing PCM 16-bit mono samples to a file.
type fileRecorder struct {
	path      string
	file      *os.File
	startedAt time.Time
	endedAt   time.Time
	closed    bool
	format    string
	// WAV header fields
	sampleRate int
	numSamples int64
}

// NewFileRecorder creates a new file-based recorder writing a WAV file in dir with the given basename.
// The final filename will be "<base>.wav".
func NewFileRecorder(dir, base string, sampleRate int) (ConversationRecorder, error) {
	if sampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rate %d", sampleRate)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create recording dir: %w", err)
	}
	path := filepath.Join(dir, base+".wav")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create recording file: %w", err)
	}
	r := &fileRecorder{
		path:       path,
		file:       f,
		startedAt:  time.Now().UTC(),
		format:     "wav",
		sampleRate: sampleRate,
	}
	if err := writeEmptyWAVHeader(f, sampleRate); err != nil {
		f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return r, nil
}

func (r *fileRecorder) WriteSamples(samples []int16, sampleRate int) error {
	if r.closed {
		return fmt.Errorf("recorder closed")
	}
	if sampleRate != r.sampleRate {
		return fmt.Errorf("unexpected sample rate %d (expected %d)", sampleRate, r.sampleRate)
	}
	if len(samples) == 0 {
		return nil
	}
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		u := uint16(s)
		buf[2*i] = byte(u)
		buf[2*i+1] = byte(u >> 8)
	}
	if _, err := r.file.Write(buf); err != nil {
		return err
	}
	r.numSamples += int64(len(samples))
	return nil
}

func (r *fileRecorder) Close() (LocalFileInfo, error) {
	if r.closed {
		return LocalFileInfo{Path: r.path, StartedAt: r.startedAt, EndedAt: r.endedAt, Format: r.format}, nil
	}
	r.closed = true
	r.endedAt = time.Now().UTC()
	if err := finalizeWAV(r.file, r.numSamples, r.sampleRate, 1); err != nil {
		_ = r.file.Close()
		return LocalFileInfo{}, err
	}
	if err := r.file.Close(); err != nil {
		return LocalFileInfo{}, err
	}
	return LocalFileInfo{
		Path:      r.path,
		StartedAt: r.startedAt,
		EndedAt:   r.endedAt,
		Format:    r.format,
	}, nil
}

