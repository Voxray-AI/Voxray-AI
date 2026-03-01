package logger_test

import (
	"testing"

	"voila-go/pkg/logger"
)

func TestInfo(t *testing.T) {
	// Info should not panic.
	logger.Info("test %s", "message")
}

func TestError(t *testing.T) {
	// Error should not panic.
	logger.Error("error %s", "message")
}

func TestDebug_WhenDisabled(t *testing.T) {
	// Debug should no-op when DebugEnabled is false (default).
	orig := logger.DebugEnabled
	defer func() { logger.DebugEnabled = orig }()
	logger.DebugEnabled = false
	logger.Debug("should not log")
}

func TestDebug_WhenEnabled(t *testing.T) {
	// Debug should not panic when enabled.
	orig := logger.DebugEnabled
	defer func() { logger.DebugEnabled = orig }()
	logger.DebugEnabled = true
	logger.Debug("debug %s", "message")
}
