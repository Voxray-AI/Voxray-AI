// Package transport defines the transport interface for pipeline input/output.
package transport

import (
	"context"

	"voxray-go/pkg/frames"
)

// Transport provides frame input and output for a pipeline (e.g. WebSocket client).
// Done returns a channel that is closed when the transport has shut down (for session caps and cleanup).
type Transport interface {
	Input() <-chan frames.Frame
	Output() chan<- frames.Frame
	Start(ctx context.Context) error
	Close() error
	Done() <-chan struct{}
}
