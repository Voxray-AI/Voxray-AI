// Package logger provides minimal logging for Voila. Uses std log; supports log level and optional JSON output.
package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

var (
	// Default is the standard logger; Info and Debug write to it unless disabled.
	Default = log.New(os.Stderr, "[Voila] ", log.LstdFlags)
	// DebugEnabled when true allows Debug logs. Set by Configure or directly.
	DebugEnabled = false
	// logLevel is the current minimum level: "debug", "info", "error". Empty means info.
	logLevel = "info"
	// jsonLogs when true outputs one JSON object per line.
	jsonLogs = false
)

// Configure sets log level and JSON mode. Level is "debug", "info", or "error" (case-insensitive).
// When json is true, each log line is a JSON object: {"level":"info","msg":"..."}.
func Configure(level string, json bool) {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "debug", "info", "error":
		logLevel = level
	default:
		if level != "" {
			logLevel = "info"
		}
	}
	DebugEnabled = logLevel == "debug"
	jsonLogs = json
}

// Info logs an informational message (unless level is "error").
func Info(format string, args ...interface{}) {
	if logLevel == "error" {
		return
	}
	write("info", format, args...)
}

// Debug logs a debug message only if level is "debug".
func Debug(format string, args ...interface{}) {
	if !DebugEnabled {
		return
	}
	write("debug", format, args...)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	write("error", format, args...)
}

func write(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if jsonLogs {
		out, _ := json.Marshal(struct {
			Level string `json:"level"`
			Msg   string `json:"msg"`
		}{Level: level, Msg: msg})
		Default.Output(2, string(out)+"\n")
	} else {
		Default.Output(2, "["+strings.ToUpper(level)+"] "+msg+"\n")
	}
}
