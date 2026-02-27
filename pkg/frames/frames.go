// Package frames defines frame types for the Voila pipeline (audio, text, system, control).
package frames

import "sync/atomic"

var idCounter uint64

func nextID() uint64 { return atomic.AddUint64(&idCounter, 1) }

// Frame is the interface implemented by all pipeline frames.
type Frame interface {
	FrameType() string
	ID() uint64
	PTS() *int64
	Metadata() map[string]any
}

// Base holds common frame fields. Embed in concrete frame types.
type Base struct {
	id       uint64
	pts      *int64
	metadata map[string]any
}

// NewBase returns a Base with a new ID.
func NewBase() Base {
	return Base{id: nextID(), metadata: make(map[string]any)}
}

// ID returns the frame ID.
func (b *Base) ID() uint64 { return b.id }

// PTS returns presentation timestamp (nanoseconds); may be nil.
func (b *Base) PTS() *int64 { return b.pts }

// SetPTS sets the presentation timestamp.
func (b *Base) SetPTS(ns int64) { b.pts = &ns }

// Metadata returns the metadata map.
func (b *Base) Metadata() map[string]any {
	if b.metadata == nil {
		b.metadata = make(map[string]any)
	}
	return b.metadata
}

// SystemFrame is a high-priority frame (e.g. Start, Cancel, Error).
type SystemFrame struct{ Base }

func (*SystemFrame) FrameType() string { return "SystemFrame" }

// DataFrame is processed in order and carries data (text, audio, etc.).
type DataFrame struct{ Base }

func (*DataFrame) FrameType() string { return "DataFrame" }

// ControlFrame carries control information (pause, resume, etc.).
type ControlFrame struct{ Base }

func (*ControlFrame) FrameType() string { return "ControlFrame" }

// StartFrame initializes the pipeline (first frame pushed).
type StartFrame struct {
	SystemFrame
	AudioInSampleRate   int  `json:"audio_in_sample_rate"`
	AudioOutSampleRate  int  `json:"audio_out_sample_rate"`
	AllowInterruptions  bool `json:"allow_interruptions"`
	EnableMetrics       bool `json:"enable_metrics"`
	EnableUsageMetrics  bool `json:"enable_usage_metrics"`
}

func (*StartFrame) FrameType() string { return "StartFrame" }

// NewStartFrame creates a StartFrame with defaults.
func NewStartFrame() *StartFrame {
	return &StartFrame{
		SystemFrame:         SystemFrame{Base: NewBase()},
		AudioInSampleRate:  16000,
		AudioOutSampleRate: 24000,
	}
}

// CancelFrame signals the pipeline to stop.
type CancelFrame struct {
	SystemFrame
	Reason any `json:"reason,omitempty"`
}

func (*CancelFrame) FrameType() string { return "CancelFrame" }

// NewCancelFrame creates a CancelFrame.
func NewCancelFrame(reason any) *CancelFrame {
	return &CancelFrame{SystemFrame: SystemFrame{Base: NewBase()}, Reason: reason}
}

// ErrorFrame notifies upstream of an error.
type ErrorFrame struct {
	SystemFrame
	Error     string `json:"error"`
	Fatal     bool   `json:"fatal"`
	Processor string `json:"processor,omitempty"`
}

func (*ErrorFrame) FrameType() string { return "ErrorFrame" }

// NewErrorFrame creates an ErrorFrame.
func NewErrorFrame(err string, fatal bool, processor string) *ErrorFrame {
	return &ErrorFrame{SystemFrame: SystemFrame{Base: NewBase()}, Error: err, Fatal: fatal, Processor: processor}
}

// TextFrame carries text (e.g. from LLM or for TTS).
type TextFrame struct {
	DataFrame
	Text                   string `json:"text"`
	SkipTTS                *bool  `json:"skip_tts,omitempty"`
	IncludesInterFrameSpace bool   `json:"includes_inter_frame_spaces"`
	AppendToContext        bool   `json:"append_to_context"`
}

func (*TextFrame) FrameType() string { return "TextFrame" }

// NewTextFrame creates a TextFrame with a new Base.
func NewTextFrame(text string) *TextFrame {
	t := &TextFrame{DataFrame: DataFrame{Base: NewBase()}, Text: text, AppendToContext: true}
	return t
}

// TranscriptionFrame is STT output (user speech).
type TranscriptionFrame struct {
	TextFrame
	UserID    string `json:"user_id"`
	Timestamp string `json:"timestamp"`
	Language  string `json:"language,omitempty"`
	Finalized bool   `json:"finalized"`
}

func (*TranscriptionFrame) FrameType() string { return "TranscriptionFrame" }

// NewTranscriptionFrame creates a TranscriptionFrame.
func NewTranscriptionFrame(text, userID, timestamp string, finalized bool) *TranscriptionFrame {
	return &TranscriptionFrame{
		TextFrame:  TextFrame{DataFrame: DataFrame{Base: NewBase()}, Text: text, AppendToContext: true},
		UserID:     userID,
		Timestamp:  timestamp,
		Finalized:  finalized,
	}
}

// AudioRawFrame holds raw PCM audio.
type AudioRawFrame struct {
	DataFrame
	Audio       []byte `json:"-"` // not JSON-encoded in wire format typically; use base64 if needed
	SampleRate  int    `json:"sample_rate"`
	NumChannels int    `json:"num_channels"`
	NumFrames   int    `json:"num_frames"`
}

func (*AudioRawFrame) FrameType() string { return "AudioRawFrame" }

// NewAudioRawFrame creates an AudioRawFrame; num_frames is derived from len(audio)/(channels*2) if 0.
func NewAudioRawFrame(audio []byte, sampleRate, numChannels int, numFrames int) *AudioRawFrame {
	if numFrames == 0 && numChannels > 0 {
		numFrames = len(audio) / (numChannels * 2)
	}
	return &AudioRawFrame{
		DataFrame:   DataFrame{Base: NewBase()},
		Audio:       audio,
		SampleRate:  sampleRate,
		NumChannels: numChannels,
		NumFrames:   numFrames,
	}
}

// OutputAudioRawFrame is audio destined for transport output.
type OutputAudioRawFrame struct {
	AudioRawFrame
	TransportDestination string `json:"transport_destination,omitempty"`
}

func (*OutputAudioRawFrame) FrameType() string { return "OutputAudioRawFrame" }

// TTSAudioRawFrame is TTS-generated audio.
type TTSAudioRawFrame struct {
	OutputAudioRawFrame
}

func (*TTSAudioRawFrame) FrameType() string { return "TTSAudioRawFrame" }

// NewTTSAudioRawFrame creates a TTS audio frame (mono 16-bit PCM).
func NewTTSAudioRawFrame(audio []byte, sampleRate int) *TTSAudioRawFrame {
	n := len(audio) / 2
	base := NewAudioRawFrame(audio, sampleRate, 1, n)
	return &TTSAudioRawFrame{OutputAudioRawFrame: OutputAudioRawFrame{AudioRawFrame: *base}}
}
