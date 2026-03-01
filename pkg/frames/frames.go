// Package frames defines frame types for the Voxray pipeline (audio, text, system, control).
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

// NewBaseWithID returns a Base with the given ID (e.g. when restoring from wire).
func NewBaseWithID(id uint64) Base {
	return Base{id: id, metadata: make(map[string]any)}
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

// SyncFrame is a control frame used to synchronize parallel pipeline processing.
// When a branch emits SyncFrame it signals that it has finished processing the current batch.
type SyncFrame struct {
	ControlFrame
}

func (*SyncFrame) FrameType() string { return "SyncFrame" }

// NewSyncFrame creates a SyncFrame.
func NewSyncFrame() *SyncFrame {
	return &SyncFrame{ControlFrame: ControlFrame{Base: NewBase()}}
}

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

// EndFrame signals that the pipeline has ended normally (all processing complete).
type EndFrame struct {
	SystemFrame
}

func (*EndFrame) FrameType() string { return "EndFrame" }

// NewEndFrame creates an EndFrame.
func NewEndFrame() *EndFrame {
	return &EndFrame{SystemFrame: SystemFrame{Base: NewBase()}}
}

// StopFrame signals the pipeline to stop (processors keep connections open).
type StopFrame struct {
	SystemFrame
}

func (*StopFrame) FrameType() string { return "StopFrame" }

// InterruptionFrame signals the transport to clear the playback buffer (barge-in).
// Used by telephony serializers (Twilio, Telnyx, Plivo, Vonage, Exotel) for "clear" events.
type InterruptionFrame struct {
	ControlFrame
}

func (*InterruptionFrame) FrameType() string { return "InterruptionFrame" }

// NewInterruptionFrame creates an InterruptionFrame.
func NewInterruptionFrame() *InterruptionFrame {
	return &InterruptionFrame{ControlFrame: ControlFrame{Base: NewBase()}}
}

// NewStopFrame creates a StopFrame.
func NewStopFrame() *StopFrame {
	return &StopFrame{SystemFrame: SystemFrame{Base: NewBase()}}
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

// AggregatedTextFrame is text emitted after aggregation (e.g. by IVR pattern aggregator); AggregatedBy names the aggregation type.
type AggregatedTextFrame struct {
	TextFrame
	AggregatedBy string `json:"aggregated_by,omitempty"`
}

func (*AggregatedTextFrame) FrameType() string { return "AggregatedTextFrame" }

// NewAggregatedTextFrame creates an AggregatedTextFrame.
func NewAggregatedTextFrame(text, aggregatedBy string) *AggregatedTextFrame {
	t := &AggregatedTextFrame{
		TextFrame:     TextFrame{DataFrame: DataFrame{Base: NewBase()}, Text: text, AppendToContext: true},
		AggregatedBy:  aggregatedBy,
	}
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

// TransportMessageFrame carries a generic transport message (e.g. for provider protocols).
// Used for binary MessageFrame wire format and JSON envelope round-trip.
type TransportMessageFrame struct {
	DataFrame
	Message map[string]any `json:"message"`
}

func (*TransportMessageFrame) FrameType() string { return "TransportMessageFrame" }

// NewTransportMessageFrame creates a TransportMessageFrame with the given message.
func NewTransportMessageFrame(msg map[string]any) *TransportMessageFrame {
	return &TransportMessageFrame{
		DataFrame: DataFrame{Base: NewBase()},
		Message:   msg,
	}
}

// Service switcher frame types for runtime service switching (e.g. STT/LLM/TTS).

// ManuallySwitchServiceFrame requests switching to a service by name.
type ManuallySwitchServiceFrame struct {
	SystemFrame
	ServiceName string `json:"service_name"`
}

func (*ManuallySwitchServiceFrame) FrameType() string { return "ManuallySwitchServiceFrame" }

// NewManuallySwitchServiceFrame creates a frame to request switching to the named service.
func NewManuallySwitchServiceFrame(serviceName string) *ManuallySwitchServiceFrame {
	return &ManuallySwitchServiceFrame{SystemFrame: SystemFrame{Base: NewBase()}, ServiceName: serviceName}
}

// ServiceMetadataFrame carries metadata from a service (e.g. capabilities).
type ServiceMetadataFrame struct {
	SystemFrame
	ServiceName string         `json:"service_name"`
	Meta        map[string]any `json:"metadata,omitempty"`
}

func (*ServiceMetadataFrame) FrameType() string { return "ServiceMetadataFrame" }

// NewServiceMetadataFrame creates a ServiceMetadataFrame.
func NewServiceMetadataFrame(serviceName string, meta map[string]any) *ServiceMetadataFrame {
	return &ServiceMetadataFrame{SystemFrame: SystemFrame{Base: NewBase()}, ServiceName: serviceName, Meta: meta}
}

// ServiceSwitcherRequestMetadataFrame requests metadata from a specific service.
type ServiceSwitcherRequestMetadataFrame struct {
	SystemFrame
	ServiceName string `json:"service_name"`
}

func (*ServiceSwitcherRequestMetadataFrame) FrameType() string { return "ServiceSwitcherRequestMetadataFrame" }

// NewServiceSwitcherRequestMetadataFrame creates a request for the named service's metadata.
func NewServiceSwitcherRequestMetadataFrame(serviceName string) *ServiceSwitcherRequestMetadataFrame {
	return &ServiceSwitcherRequestMetadataFrame{SystemFrame: SystemFrame{Base: NewBase()}, ServiceName: serviceName}
}
