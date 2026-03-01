package filters

import (
	"context"
	"encoding/json"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// STTMuteStrategy determines when STT is muted.
type STTMuteStrategy string

const (
	STTMuteFirstSpeech              STTMuteStrategy = "first_speech"                // mute until first bot speech detected
	STTMuteUntilFirstBotComplete     STTMuteStrategy = "mute_until_first_bot_complete" // mute until first bot speech completes
	STTMuteAlways                    STTMuteStrategy = "always"                      // mute whenever bot is speaking
	STTMuteCustom                    STTMuteStrategy = "custom"                      // use ShouldMuteCallback
)

// STTMuteConfig configures STT muting. FirstSpeech and MuteUntilFirstBotComplete should not be used together.
type STTMuteConfig struct {
	Strategies        []STTMuteStrategy `json:"strategies"`
	ShouldMuteCallback func(ctx context.Context, filter *STTMuteFilter) (bool, error) `json:"-"` // for Custom; set programmatically
}

// STTMuteFilterOptions is the JSON shape for plugin_options["stt_mute_filter"].
type STTMuteFilterOptions struct {
	Strategies []string `json:"strategies"` // e.g. ["first_speech", "always"]
}

// STTMuteFilter suppresses VAD/STT-related frames when muted (e.g. during bot speech or until first bot complete).
// When muted, VAD, transcription, interruption, and input audio frames are dropped.
type STTMuteFilter struct {
	*processors.BaseProcessor
	Config STTMuteConfig

	mu                 sync.Mutex
	firstSpeechHandled bool
	botIsSpeaking      bool
	isMuted            bool
}

// NewSTTMuteFilter returns an STT mute filter with the given config.
func NewSTTMuteFilter(name string, config STTMuteConfig) *STTMuteFilter {
	if name == "" {
		name = "STTMuteFilter"
	}
	return &STTMuteFilter{
		BaseProcessor: processors.NewBaseProcessor(name),
		Config:        config,
	}
}

// NewSTTMuteFilterFromOptions builds from plugin_options. Custom strategy and callback require programmatic setup.
func NewSTTMuteFilterFromOptions(name string, opts json.RawMessage) *STTMuteFilter {
	var o STTMuteFilterOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	strategies := make([]STTMuteStrategy, 0, len(o.Strategies))
	for _, s := range o.Strategies {
		strategies = append(strategies, STTMuteStrategy(s))
	}
	return NewSTTMuteFilter(name, STTMuteConfig{Strategies: strategies})
}

func (p *STTMuteFilter) shouldMute(ctx context.Context) bool {
	for _, strategy := range p.Config.Strategies {
		switch strategy {
		case STTMuteAlways:
			if p.botIsSpeaking {
				return true
			}
		case STTMuteFirstSpeech:
			if p.botIsSpeaking && !p.firstSpeechHandled {
				return true
			}
		case STTMuteUntilFirstBotComplete:
			if !p.firstSpeechHandled {
				return true
			}
		case STTMuteCustom:
			if p.botIsSpeaking && p.Config.ShouldMuteCallback != nil {
				muted, _ := p.Config.ShouldMuteCallback(ctx, p)
				if muted {
					return true
				}
			}
		}
	}
	return false
}

func (p *STTMuteFilter) isSTTOrVADFrame(f frames.Frame) bool {
	switch f.(type) {
	case *frames.VADUserStartedSpeakingFrame, *frames.VADUserStoppedSpeakingFrame,
		*frames.UserStartedSpeakingFrame, *frames.UserStoppedSpeakingFrame,
		*frames.TranscriptionFrame, *frames.InterruptionFrame:
		return true
	}
	// InputAudioRawFrame: we don't have a separate type; AudioRawFrame is used for both. Omit raw input audio from suppress list to avoid breaking non-STT audio.
	return false
}

// ProcessFrame updates mute state from bot/session frames and suppresses STT/VAD frames when muted.
func (p *STTMuteFilter) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	p.mu.Lock()
	shouldMute := p.isMuted

	switch f.(type) {
	case *frames.BotStartedSpeakingFrame:
		p.botIsSpeaking = true
		if !p.firstSpeechHandled {
			p.firstSpeechHandled = true
		}
		shouldMute = p.shouldMute(ctx)
	case *frames.BotStoppedSpeakingFrame:
		p.botIsSpeaking = false
		shouldMute = p.shouldMute(ctx)
	default:
		if _, ok := f.(*frames.StartFrame); ok {
			shouldMute = p.shouldMute(ctx)
		}
	}

	if shouldMute != p.isMuted {
		p.isMuted = shouldMute
	}
	muted := p.isMuted
	p.mu.Unlock()

	if p.isSTTOrVADFrame(f) {
		if muted {
			return nil
		}
	}
	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}

var _ processors.Processor = (*STTMuteFilter)(nil)
