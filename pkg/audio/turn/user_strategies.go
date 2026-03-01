package turn

// UserTurnStartStrategy decides when a user turn starts based on incoming frames.
type UserTurnStartStrategy interface {
	Reset()
	// OnUserStartedSpeaking should be called when a VAD- or externally-driven
	// event indicates that the user has started speaking.
	OnUserStartedSpeaking()
	// ShouldStartTurn returns true when this strategy believes a user turn
	// should be started given current internal state.
	ShouldStartTurn() bool
}

// UserTurnStopStrategy decides when a user turn stops based on incoming frames.
type UserTurnStopStrategy interface {
	Reset()
	// OnUserStoppedSpeaking should be called when VAD or other signals indicate
	// the user has stopped speaking.
	OnUserStoppedSpeaking()
	// ShouldStopTurn returns true when this strategy believes the user turn
	// should be stopped (e.g. after sufficient silence).
	ShouldStopTurn() bool
}

