// Package observers provides user-to-bot latency observer.
package observers

import (
	"sync"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// UserBotLatencyObserver measures time from user stopped speaking to bot started speaking.
// Implements Observer; only considers downstream frames. Safe for concurrent use.
type UserBotLatencyObserver struct {
	mu              sync.Mutex
	userStoppedTime *time.Time
	processedIDs    map[uint64]struct{}
	onLatency       func(latencySecs float64)
}

// UserBotLatencyOption configures UserBotLatencyObserver.
type UserBotLatencyOption func(*UserBotLatencyObserver)

// OnLatencyMeasured sets the callback when user-to-bot latency is measured.
func OnLatencyMeasured(f func(latencySecs float64)) UserBotLatencyOption {
	return func(o *UserBotLatencyObserver) {
		o.onLatency = f
	}
}

// NewUserBotLatencyObserver returns a new UserBotLatencyObserver.
func NewUserBotLatencyObserver(opts ...UserBotLatencyOption) *UserBotLatencyObserver {
	o := &UserBotLatencyObserver{
		processedIDs: make(map[uint64]struct{}),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Ensure UserBotLatencyObserver implements Observer.
var _ Observer = (*UserBotLatencyObserver)(nil)

// OnFrameProcessed implements Observer. Only downstream frames are considered.
func (o *UserBotLatencyObserver) OnFrameProcessed(processorName string, f frames.Frame, dir processors.Direction) {
	if dir != processors.Downstream {
		return
	}
	if f == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	id := f.ID()
	if _, seen := o.processedIDs[id]; seen {
		return
	}
	o.processedIDs[id] = struct{}{}
	// Keep set bounded (simple eviction: cap size)
	if len(o.processedIDs) > 500 {
		o.processedIDs = make(map[uint64]struct{})
	}

	now := time.Now()
	switch f.(type) {
	case *frames.UserStoppedSpeakingFrame:
		o.userStoppedTime = &now
	case *frames.UserStartedSpeakingFrame:
		o.userStoppedTime = nil
	case *frames.BotStartedSpeakingFrame:
		if o.userStoppedTime != nil {
			latencySecs := now.Sub(*o.userStoppedTime).Seconds()
			o.userStoppedTime = nil
			if o.onLatency != nil {
				o.onLatency(latencySecs)
			}
		}
	}
}
