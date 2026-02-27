package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"voila-go/pkg/config"
	"voila-go/pkg/services"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: go run examples/transcribe_file.go <path-to-audio-file>\n")
		os.Exit(1)
	}

	filePath := os.Args[1]
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Printf("Error getting absolute path: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Printf("File does not exist: %s\n", absPath)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Force Groq provider for this example
	cfg.Provider = "groq"
	_, sttSvc, _ := services.NewServicesFromConfig(cfg)

	audioData, err := os.ReadFile(absPath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Transcribing file: %s (%d bytes)...\n", absPath, len(audioData))

	ctx := context.Background()
	// Sample rate and channels aren't strictly used by Groq Whisper as it uses the file header
	frames, err := sttSvc.Transcribe(ctx, audioData, 16000, 1)
	if err != nil {
		fmt.Printf("Transcription error: %v\n", err)
		os.Exit(1)
	}

	if len(frames) > 0 {
		fmt.Println("\nTranscription result:")
		fmt.Println("---------------------")
		fmt.Println(frames[0].Text)
		fmt.Println("---------------------")
	} else {
		fmt.Println("No transcription result received.")
	}
}
