package voice

import (
	"context"
	"strings"
	"unicode/utf8"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/processors"
	"voila-go/pkg/services"
)

// Sentence-ending runes (English + Devanagari "।"). TTS flushes when buffer contains one of these or exceeds MaxBatchRunes.
const ttsSentenceEnd = ".!?\n।"
const defaultMaxBatchRunes = 120
const minBatchRunes = 30 // don't flush tiny fragments; wait for at least this many runes (or max batch) to reduce stutter

// TTSProcessor turns LLMTextFrame, TextFrame, or TTSSpeakFrame into TTSAudioRawFrame using a TTSService.
// It batches streamed LLM text by sentence (or MaxBatchRunes) so each TTS API call gets a full phrase, reducing choppy playback.
// Emits BotStartedSpeakingFrame before the first TTS audio in a response and BotStoppedSpeakingFrame after each segment for observers.
type TTSProcessor struct {
	*processors.BaseProcessor
	TTS            services.TTSService
	SampleRate     int
	MaxBatchRunes  int   // max runes before flushing without sentence end (0 = use default)
	buf            strings.Builder
	botSpeaking    bool // true after we've pushed BotStartedSpeakingFrame this turn; reset on UserStartedSpeakingFrame/StartFrame
}

// NewTTSProcessor returns a processor that speaks text and pushes TTSAudioRawFrame(s) downstream.
func NewTTSProcessor(name string, tts services.TTSService, sampleRate int) *TTSProcessor {
	if name == "" {
		name = "TTS"
	}
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	maxBatch := defaultMaxBatchRunes
	return &TTSProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		TTS:           tts,
		SampleRate:    sampleRate,
		MaxBatchRunes: maxBatch,
	}
}

// ProcessFrame buffers LLMTextFrame/TextFrame until a sentence boundary or limit, then speaks; TTSSpeakFrame is spoken immediately. Forwards other frames (flushing any pending text first).
func (p *TTSProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.LLMTextFrame:
		p.buf.WriteString(t.Text)
		return p.tryFlush(ctx)
	case *frames.TextFrame:
		p.buf.WriteString(t.Text)
		return p.tryFlush(ctx)
	case *frames.TTSSpeakFrame:
		// Explicit speak request: flush pending then speak this text immediately
		_ = p.flush(ctx)
		if t.Text == "" {
			return nil
		}
		return p.speak(ctx, t.Text)
	case *frames.UserStartedSpeakingFrame:
		// Barge-in: clear buffered text so no new speech is generated from the previous bot turn.
		p.buf.Reset()
		p.botSpeaking = false
		return p.PushDownstream(ctx, f)
	case *frames.StartFrame:
		p.botSpeaking = false
		return p.PushDownstream(ctx, f)
	default:
		_ = p.flush(ctx)
		return p.PushDownstream(ctx, f)
	}
}

func (p *TTSProcessor) tryFlush(ctx context.Context) error {
	s := p.buf.String()
	runeCount := utf8.RuneCountInString(s)
	if runeCount == 0 {
		return nil
	}
	maxRunes := p.MaxBatchRunes
	if maxRunes <= 0 {
		maxRunes = defaultMaxBatchRunes
	}
	hasSentenceEnd := false
	for _, r := range ttsSentenceEnd {
		if strings.ContainsRune(s, r) {
			hasSentenceEnd = true
			break
		}
	}
	// Flush only when we have enough content: full batch (120 runes) or sentence end with at least minBatchRunes (avoids 1–2 token stutter)
	flush := runeCount >= maxRunes || (hasSentenceEnd && runeCount >= minBatchRunes)
	if flush {
		return p.flush(ctx)
	}
	return nil
}

func (p *TTSProcessor) flush(ctx context.Context) error {
	if p.buf.Len() == 0 {
		return nil
	}
	text := strings.TrimSpace(p.buf.String())
	p.buf.Reset()
	if text == "" {
		return nil
	}
	return p.speak(ctx, text)
}

func (p *TTSProcessor) speak(ctx context.Context, text string) error {
	runes := []rune(text)
	preview := text
	if len(runes) > 80 {
		preview = string(runes[:80]) + "..."
	}
	logger.Info("TTS: received text to speak: %d runes (batched), preview=%q", len(runes), preview)

	audioFrames, err := p.TTS.Speak(ctx, text, p.SampleRate)
	if err != nil {
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(err.Error(), false, p.Name()))
		return nil
	}
	for i, af := range audioFrames {
		if !p.botSpeaking {
			p.botSpeaking = true
			_ = p.PushDownstream(ctx, frames.NewBotStartedSpeakingFrame())
		}
		_ = p.PushDownstream(ctx, af)
		if i == len(audioFrames)-1 {
			_ = p.PushDownstream(ctx, frames.NewBotStoppedSpeakingFrame())
		}
	}
	logger.Info("pipeline (TTS): sent %d audio frame(s) to output", len(audioFrames))
	return nil
}
