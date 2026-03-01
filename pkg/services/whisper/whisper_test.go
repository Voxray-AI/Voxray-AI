package whisper

import "testing"

func TestBuild(t *testing.T) {
	// Ensures the whisper package compiles and NewService can be called.
	_ = NewService("", "")
}
