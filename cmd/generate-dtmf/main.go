// generate-dtmf generates DTMF WAV files (0-9, *, #) using pkg/audio.
// Usage: go run ./cmd/generate-dtmf [output_dir]
// Default output dir is current directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"voila-go/pkg/audio"
)

const (
	sampleRate   = 8000
	toneDuration = 0.3
	gapDuration  = 0.2
)

var keys = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "star", "pound"}

func main() {
	outDir := "."
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	for _, key := range keys {
		pcm, err := audio.GenerateDTMFPCM(sampleRate, key, toneDuration, gapDuration)
		if err != nil || pcm == nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", key, err)
			continue
		}
		path := filepath.Join(outDir, "dtmf-"+key+".wav")
		if err := audio.WritePCM16MonoWAV(path, pcm, sampleRate); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("Generated %s\n", path)
	}
}
