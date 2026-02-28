package logger

import "testing"

func TestInfo(t *testing.T) {
	// Info should not panic.
	Info("test %s", "message")
}

func TestError(t *testing.T) {
	// Error should not panic.
	Error("error %s", "message")
}

func TestDebug_WhenDisabled(t *testing.T) {
	// Debug should no-op when DebugEnabled is false (default).
	orig := DebugEnabled
	defer func() { DebugEnabled = orig }()
	DebugEnabled = false
	Debug("should not log")
}

func TestDebug_WhenEnabled(t *testing.T) {
	// Debug should not panic when enabled.
	orig := DebugEnabled
	defer func() { DebugEnabled = orig }()
	DebugEnabled = true
	Debug("debug %s", "message")
}
