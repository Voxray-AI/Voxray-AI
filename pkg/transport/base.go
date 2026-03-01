// Package transport defines an optional base for transports with common fields (name, logger).
package transport

import (
	"log"
)

// Logger interface for optional transport logging (e.g. *log.Logger).
type Logger interface {
	Printf(format string, v ...interface{})
}

// Base holds common transport fields. Embed in concrete transport implementations.
type Base struct {
	Name string
	// Log can be nil; if set, used for transport-level logging.
	Log Logger
}

// SetName sets the transport name.
func (b *Base) SetName(name string) { b.Name = name }

// GetName returns the transport name.
func (b *Base) GetName() string { return b.Name }

// Logf logs if Log is set; otherwise no-op. Pass a *log.Logger for Log to use default logging.
func (b *Base) Logf(format string, v ...interface{}) {
	if b.Log != nil {
		b.Log.Printf(format, v...)
	}
}

var _ Logger = (*log.Logger)(nil)
