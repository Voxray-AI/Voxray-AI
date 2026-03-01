package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/transcribestreaming"
	"github.com/aws/aws-sdk-go-v2/service/transcribestreaming/types"

	"voxray-go/pkg/frames"
)

// AWS Transcribe streaming supports 8000 and 16000 Hz.
const transcribeSampleRate = 16000

// STTService implements services.STTService using Amazon Transcribe streaming.
type STTService struct {
	client     *transcribestreaming.Client
	languageCode types.LanguageCode
}

// NewSTT creates an AWS Transcribe STT service from an existing AWS config.
func NewSTT(cfg aws.Config, languageCode types.LanguageCode) *STTService {
	if languageCode == "" {
		languageCode = types.LanguageCodeEnUs
	}
	return &STTService{
		client:       transcribestreaming.NewFromConfig(cfg),
		languageCode: languageCode,
	}
}

// NewSTTWithRegion creates a Transcribe STT service by loading default config for the given region.
// languageCode can be empty for default (en-US), or an AWS Transcribe language code (e.g. "en-US", "es-ES").
func NewSTTWithRegion(ctx context.Context, region, languageCode string) (*STTService, error) {
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws transcribe: load config: %w", err)
	}
	lc := types.LanguageCode(languageCode)
	if lc == "" {
		lc = types.LanguageCodeEnUs
	}
	return NewSTT(cfg, lc), nil
}

// Transcribe sends audio to AWS Transcribe streaming and returns the final transcript as TranscriptionFrame(s).
// Audio must be 16-bit PCM mono at 8000 or 16000 Hz; other sample rates are not supported by Transcribe streaming.
func (s *STTService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	if sampleRate != 8000 && sampleRate != 16000 {
		sampleRate = transcribeSampleRate
	}
	sr32 := int32(sampleRate)
	input := &transcribestreaming.StartStreamTranscriptionInput{
		LanguageCode:                  s.languageCode,
		MediaEncoding:                 types.MediaEncodingPcm,
		MediaSampleRateHertz:          &sr32,
		EnablePartialResultsStabilization: true,
		PartialResultsStability:       types.PartialResultsStabilityHigh,
	}
	output, err := s.client.StartStreamTranscription(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("aws transcribe: start stream: %w", err)
	}
	stream := output.GetStream()
	defer stream.Close()

	var (
		mu       sync.Mutex
		finalText string
		errResult error
	)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer stream.Close()
		const chunkSize = 1024
		for i := 0; i < len(audio); i += chunkSize {
			end := i + chunkSize
			if end > len(audio) {
				end = len(audio)
			}
			chunk := audio[i:end]
			if err := stream.Send(ctx, &types.AudioStreamMemberAudioEvent{Value: types.AudioEvent{AudioChunk: chunk}}); err != nil {
				mu.Lock()
				errResult = err
				mu.Unlock()
				return
			}
		}
	}()

	for event := range stream.Events() {
		switch v := event.(type) {
		case *types.TranscriptResultStreamMemberTranscriptEvent:
			for _, res := range v.Value.Transcript.Results {
				if res.Alternatives != nil && len(res.Alternatives) > 0 && res.Alternatives[0].Transcript != nil {
					text := *res.Alternatives[0].Transcript
					if !res.IsPartial {
						mu.Lock()
						finalText = text
						mu.Unlock()
					}
				}
			}
		}
	}
	<-done
	if errResult != nil {
		return nil, errResult
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	tf := frames.NewTranscriptionFrame(finalText, "user", "", true)
	return []*frames.TranscriptionFrame{tf}, nil
}
