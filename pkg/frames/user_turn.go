package frames

// UserStartedSpeakingFrame signals that the user has started speaking.
// It is a lightweight control frame that can be used by downstream
// processors to react to user turn start events (e.g., cancel TTS,
// adjust UI state, etc.).
type UserStartedSpeakingFrame struct {
	ControlFrame
}

func (*UserStartedSpeakingFrame) FrameType() string { return "UserStartedSpeakingFrame" }

// NewUserStartedSpeakingFrame creates a UserStartedSpeakingFrame.
func NewUserStartedSpeakingFrame() *UserStartedSpeakingFrame {
	return &UserStartedSpeakingFrame{ControlFrame: ControlFrame{Base: NewBase()}}
}

// UserStoppedSpeakingFrame signals that the user has stopped speaking
// (i.e., the user turn has ended as determined by the turn analyzer
// and strategies).
type UserStoppedSpeakingFrame struct {
	ControlFrame
}

func (*UserStoppedSpeakingFrame) FrameType() string { return "UserStoppedSpeakingFrame" }

// NewUserStoppedSpeakingFrame creates a UserStoppedSpeakingFrame.
func NewUserStoppedSpeakingFrame() *UserStoppedSpeakingFrame {
	return &UserStoppedSpeakingFrame{ControlFrame: ControlFrame{Base: NewBase()}}
}

// UserIdleFrame signals that the user has been idle (not speaking)
// for a configured timeout period after the bot has finished speaking.
// It can be used to trigger follow-up prompts or other behaviors.
type UserIdleFrame struct {
	ControlFrame
}

func (*UserIdleFrame) FrameType() string { return "UserIdleFrame" }

// NewUserIdleFrame creates a UserIdleFrame.
func NewUserIdleFrame() *UserIdleFrame {
	return &UserIdleFrame{ControlFrame: ControlFrame{Base: NewBase()}}
}

