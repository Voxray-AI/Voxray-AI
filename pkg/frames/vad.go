// Package frames defines VAD/turn-related control frames.
package frames

// VADParamsUpdateFrame updates VAD/turn parameters (e.g. stop_secs for IVR mode).
// Processors such as TurnProcessor apply these to their Analyzer when received.
type VADParamsUpdateFrame struct {
	ControlFrame
	// StopSecs is silence duration in seconds to end turn (e.g. 2.0 for IVR).
	StopSecs float64 `json:"stop_secs"`
	// StartSecs is VAD start trigger time in seconds (pre-speech padding).
	StartSecs float64 `json:"start_secs,omitempty"`
}

func (*VADParamsUpdateFrame) FrameType() string { return "VADParamsUpdateFrame" }

// NewVADParamsUpdateFrame creates a VADParamsUpdateFrame.
func NewVADParamsUpdateFrame(stopSecs, startSecs float64) *VADParamsUpdateFrame {
	return &VADParamsUpdateFrame{
		ControlFrame: ControlFrame{Base: NewBase()},
		StopSecs:     stopSecs,
		StartSecs:    startSecs,
	}
}

// VADUserStartedSpeakingFrame is emitted by VADProcessor when speech is detected.
// StartSecs may carry the analyzer's pre-speech padding (for context).
type VADUserStartedSpeakingFrame struct {
	ControlFrame
	StartSecs float64 `json:"start_secs,omitempty"`
}

func (*VADUserStartedSpeakingFrame) FrameType() string { return "VADUserStartedSpeakingFrame" }

// NewVADUserStartedSpeakingFrame creates a VADUserStartedSpeakingFrame.
func NewVADUserStartedSpeakingFrame(startSecs float64) *VADUserStartedSpeakingFrame {
	return &VADUserStartedSpeakingFrame{
		ControlFrame: ControlFrame{Base: NewBase()},
		StartSecs:   startSecs,
	}
}

// VADUserStoppedSpeakingFrame is emitted by VADProcessor when speech ends.
// StopSecs may carry the analyzer's silence duration that triggered the stop.
type VADUserStoppedSpeakingFrame struct {
	ControlFrame
	StopSecs float64 `json:"stop_secs,omitempty"`
}

func (*VADUserStoppedSpeakingFrame) FrameType() string { return "VADUserStoppedSpeakingFrame" }

// NewVADUserStoppedSpeakingFrame creates a VADUserStoppedSpeakingFrame.
func NewVADUserStoppedSpeakingFrame(stopSecs float64) *VADUserStoppedSpeakingFrame {
	return &VADUserStoppedSpeakingFrame{
		ControlFrame: ControlFrame{Base: NewBase()},
		StopSecs:    stopSecs,
	}
}

// UserSpeakingFrame is emitted periodically by VADProcessor while speech is detected.
type UserSpeakingFrame struct {
	ControlFrame
}

func (*UserSpeakingFrame) FrameType() string { return "UserSpeakingFrame" }

// NewUserSpeakingFrame creates a UserSpeakingFrame.
func NewUserSpeakingFrame() *UserSpeakingFrame {
	return &UserSpeakingFrame{ControlFrame: ControlFrame{Base: NewBase()}}
}
