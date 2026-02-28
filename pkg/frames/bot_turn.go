package frames

// BotStartedSpeakingFrame signals that the bot has started speaking
// (e.g., first TTS audio is being sent). Used by observers for turn and latency tracking.
type BotStartedSpeakingFrame struct {
	ControlFrame
}

func (*BotStartedSpeakingFrame) FrameType() string { return "BotStartedSpeakingFrame" }

// NewBotStartedSpeakingFrame creates a BotStartedSpeakingFrame.
func NewBotStartedSpeakingFrame() *BotStartedSpeakingFrame {
	return &BotStartedSpeakingFrame{ControlFrame: ControlFrame{Base: NewBase()}}
}

// BotStoppedSpeakingFrame signals that the bot has stopped speaking
// (e.g., TTS has finished a segment). Used by observers for turn tracking.
type BotStoppedSpeakingFrame struct {
	ControlFrame
}

func (*BotStoppedSpeakingFrame) FrameType() string { return "BotStoppedSpeakingFrame" }

// NewBotStoppedSpeakingFrame creates a BotStoppedSpeakingFrame.
func NewBotStoppedSpeakingFrame() *BotStoppedSpeakingFrame {
	return &BotStoppedSpeakingFrame{ControlFrame: ControlFrame{Base: NewBase()}}
}
