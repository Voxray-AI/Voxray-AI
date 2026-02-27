package turn

// VADUserTurnStartStrategy starts a turn as soon as VAD reports speech.
// It is intentionally simple for now; more advanced heuristics (e.g. min
// duration, transcription-based triggers) can be layered in later.
type VADUserTurnStartStrategy struct {
	triggered bool
}

func (s *VADUserTurnStartStrategy) Reset() {
	s.triggered = false
}

func (s *VADUserTurnStartStrategy) OnUserStartedSpeaking() {
	s.triggered = true
}

func (s *VADUserTurnStartStrategy) ShouldStartTurn() bool {
	return s.triggered
}

// SilenceUserTurnStopStrategy stops a turn when VAD reports that the user
// has stopped speaking. More sophisticated behavior (e.g. using the turn
// analyzer state, additional silence windows) can be added later.
type SilenceUserTurnStopStrategy struct {
	stopped bool
}

func (s *SilenceUserTurnStopStrategy) Reset() {
	s.stopped = false
}

func (s *SilenceUserTurnStopStrategy) OnUserStoppedSpeaking() {
	s.stopped = true
}

func (s *SilenceUserTurnStopStrategy) ShouldStopTurn() bool {
	return s.stopped
}

