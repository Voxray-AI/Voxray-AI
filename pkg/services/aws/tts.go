package aws

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/aws/aws-sdk-go-v2/service/polly/types"

	"voxray-go/pkg/frames"
)

// Polly PCM only supports 8000 and 16000 Hz.
const pollyPCMSampleRate = 16000

// TTSService implements services.TTSService using Amazon Polly.
type TTSService struct {
	client  *polly.Client
	voiceID string
	engine  types.Engine
}

// NewTTS creates an AWS Polly TTS service from an existing AWS config.
func NewTTS(cfg aws.Config, voiceID string, engine types.Engine) *TTSService {
	if voiceID == "" {
		voiceID = "Joanna"
	}
	if engine == "" {
		engine = types.EngineNeural
	}
	return &TTSService{
		client:  polly.NewFromConfig(cfg),
		voiceID: voiceID,
		engine:  engine,
	}
}

// NewTTSWithRegion creates a Polly TTS service by loading default config for the given region.
// engine can be empty for default (neural), or "standard", "neural", "long-form", "generative".
func NewTTSWithRegion(ctx context.Context, region, voiceID, engine string) (*TTSService, error) {
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws polly: load config: %w", err)
	}
	eng := types.Engine(engine)
	if eng == "" {
		eng = types.EngineNeural
	}
	return NewTTS(cfg, voiceID, eng), nil
}

// Speak synthesizes text to speech and returns TTSAudioRawFrame(s). Polly PCM is 16-bit 16kHz mono.
func (s *TTSService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if sampleRate <= 0 {
		sampleRate = pollyPCMSampleRate
	}
	input := &polly.SynthesizeSpeechInput{
		Text:         aws.String(text),
		OutputFormat: types.OutputFormatPcm,
		VoiceId:      types.VoiceId(s.voiceID),
		Engine:      s.engine,
		SampleRate:   aws.String("16000"),
	}
	out, err := s.client.SynthesizeSpeech(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("aws polly: %w", err)
	}
	defer out.AudioStream.Close()
	data, err := io.ReadAll(out.AudioStream)
	if err != nil {
		return nil, fmt.Errorf("aws polly: read audio: %w", err)
	}
	f := frames.NewTTSAudioRawFrame(data, sampleRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}

// SpeakStream runs TTS and sends the resulting TTSAudioRawFrame(s) to outCh.
func (s *TTSService) SpeakStream(ctx context.Context, text string, sampleRate int, outCh chan<- frames.Frame) {
	framesOut, err := s.Speak(ctx, text, sampleRate)
	if err != nil {
		return
	}
	for _, f := range framesOut {
		select {
		case <-ctx.Done():
			return
		case outCh <- f:
		}
	}
}
