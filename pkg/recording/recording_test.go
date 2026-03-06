package recording

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildS3Key(t *testing.T) {
	base := "recordings/"
	callID := "session-123"
	format := "wav"
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	key := BuildS3Key(base, callID, format, now)
	expected := "recordings/2025/01/02/session-123.wav"
	if key != expected {
		t.Fatalf("expected key %q, got %q", expected, key)
	}
}

func TestFileRecorder_WritesWAV(t *testing.T) {
	dir := t.TempDir()
	rec, err := NewFileRecorder(dir, "test-session", 16000)
	if err != nil {
		t.Fatalf("NewFileRecorder error: %v", err)
	}

	samples := []int16{0, 1000, -1000, 2000, -2000}
	if err := rec.WriteSamples(samples, 16000); err != nil {
		t.Fatalf("WriteSamples error: %v", err)
	}
	info, err := rec.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if info.Path == "" {
		t.Fatalf("expected non-empty path from Close")
	}
	st, err := os.Stat(info.Path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if st.Size() <= 44 {
		t.Fatalf("expected WAV file larger than header (44 bytes), got %d", st.Size())
	}
	if filepath.Ext(info.Path) != ".wav" {
		t.Fatalf("expected .wav extension, got %q", filepath.Ext(info.Path))
	}
}

