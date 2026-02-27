// Package logger provides minimal logging for Voila. Uses std log; can be replaced with a leveled logger later.
package logger

import (
	"log"
	"os"
)

var (
	// Default is the standard logger; Info and Debug write to it unless disabled.
	Default = log.New(os.Stderr, "[Voila] ", log.LstdFlags)
	// DebugEnabled when true allows Debug logs.
	DebugEnabled = false
)

// Info logs an informational message.
func Info(format string, args ...interface{}) {
	Default.Printf("[INFO] "+format, args...)
}

// Debug logs a debug message only if DebugEnabled is true.
func Debug(format string, args ...interface{}) {
	if DebugEnabled {
		Default.Printf("[DEBUG] "+format, args...)
	}
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	Default.Printf("[ERROR] "+format, args...)
}
