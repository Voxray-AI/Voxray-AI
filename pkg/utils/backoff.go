// Package utils provides shared utilities (backoff, etc.).
package utils

import (
	"time"
)

const (
	// BackoffCap is the maximum duration returned by ExponentialBackoff.
	BackoffCap = 60 * time.Second
)

// ExponentialBackoff returns a duration for the given attempt (1-based).
// Duration is 2^attempt seconds, capped at BackoffCap.
// Used for reconnection backoff in websocket and other retry logic.
func ExponentialBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > BackoffCap {
		return BackoffCap
	}
	return d
}
