// Package voice provides processors that wire STT, LLM, and TTS into a pipeline.
package voice

import (
	"context"
	"strings"
	"sync"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/logger"
	"voxray-go/pkg/metrics"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/services"
)

// MinSTTBufferMs is the default minimum audio to buffer before calling STT (500ms).
// Sending very short chunks (e.g. 20ms) to STT APIs yields empty transcripts.
const MinSTTBufferMs = 500

// STTProcessor turns AudioRawFrame into TranscriptionFrame using an STTService.
// It buffers incoming audio and only calls the STT service when at least MinBufferBytes
// have been accumulated, so the API receives enough audio to return transcripts.
type STTProcessor struct {
	*processors.BaseProcessor
	STT            services.STTService
	SampleRate     int
	Channels       int
	MinBufferBytes int // min bytes before calling Transcribe (e.g. 500ms at 16kHz mono = 16000)
	mu             sync.Mutex
	buf            []byte
}

// NewSTTProcessor returns a processor that transcribes audio and pushes TranscriptionFrame(s) downstream.
// It buffers audio until at least minBufferMs of audio is available (default 500ms) before calling STT.
func NewSTTProcessor(name string, stt services.STTService, sampleRate, channels int) *STTProcessor {
	return NewSTTProcessorWithBuffer(name, stt, sampleRate, channels, MinSTTBufferMs)
}

// NewSTTProcessorWithBuffer is like NewSTTProcessor but allows setting minBufferMs (e.g. 300–800).
func NewSTTProcessorWithBuffer(name string, stt services.STTService, sampleRate, channels, minBufferMs int) *STTProcessor {
	if name == "" {
		name = "STT"
	}
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}
	if minBufferMs <= 0 {
		minBufferMs = MinSTTBufferMs
	}
	// 16-bit = 2 bytes per sample; bytes = sampleRate * channels * 2 * (minBufferMs/1000)
	minBytes := sampleRate * channels * 2 * minBufferMs / 1000
	if minBytes < 3200 {
		minBytes = 3200 // at least ~100ms at 16kHz mono
	}
	return &STTProcessor{
		BaseProcessor:  processors.NewBaseProcessor(name),
		STT:            stt,
		SampleRate:     sampleRate,
		Channels:       channels,
		MinBufferBytes: minBytes,
	}
}

// ProcessFrame buffers AudioRawFrame and transcribes when enough audio is accumulated; forwards other frames.
func (p *STTProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	ar, ok := f.(*frames.AudioRawFrame)
	if !ok {
		return p.PushDownstream(ctx, f)
	}

	p.mu.Lock()
	p.buf = append(p.buf, ar.Audio...)
	var toSend []byte
	if len(p.buf) >= p.MinBufferBytes {
		toSend = p.buf
		p.buf = nil
	}
	p.mu.Unlock()

	if toSend == nil {
		return nil
	}

	logger.Info("STT: audio packet received for transcription: %d bytes (%d Hz, %d ch), sending to STT API", len(toSend), p.SampleRate, p.Channels)
	start := time.Now()
	tfs, err := p.STT.Transcribe(ctx, toSend, p.SampleRate, p.Channels)
	latency := time.Since(start).Seconds()
	if err != nil {
		metrics.STTErrorsTotal.WithLabelValues("provider_error", "", "stt", "").Inc()
		metrics.STTTimeToFirstTokenSeconds.WithLabelValues("", "stt", "error", "").Observe(latency)
		metrics.STTTranscriptionLatencySeconds.WithLabelValues("", "stt", "error", "").Observe(latency)
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(err.Error(), false, p.Name()))
		return nil
	}
	metrics.STTTimeToFirstTokenSeconds.WithLabelValues("", "stt", "success", "").Observe(latency)
	metrics.STTTranscriptionLatencySeconds.WithLabelValues("", "stt", "success", "").Observe(latency)
	for _, tf := range tfs {
		text := strings.TrimSpace(tf.Text)
		if text == "" {
			// Don't forward empty transcripts to LLM; avoids spamming the API and only triggers response when user actually spoke.
			continue
		}
		tf.Text = text
		preview := text
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		logger.Info("STT output (to LLM): processor=%s textLen=%d finalized=%v language=%q preview=%q",
			p.Name(), len(text), tf.Finalized, tf.Language, preview)
		_ = p.PushDownstream(ctx, tf)
	}
	return nil
}
