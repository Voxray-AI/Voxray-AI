// Package transport defines the transport interface for pipeline input/output.
package transport

import (
	"context"

	"voxray-go/pkg/frames"
)

// Transport provides frame input and output for a pipeline (e.g. WebSocket client).
type Transport interface {
	Input() <-chan frames.Frame
	Output() chan<- frames.Frame
	Start(ctx context.Context) error
	Close() error
}
