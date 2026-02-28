// Package observers provides turn tracking observer for conversation flow monitoring.
package observers

import (
	"sync"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

const defaultMaxFrames = 100
const defaultTurnEndTimeoutSecs = 2.5

// TurnTrackingObserver tracks conversation turns: turn start (StartFrame or user started),
// turn end (bot stopped + timeout or user started again or CancelFrame).
// Implements Observer; safe for concurrent use.
type TurnTrackingObserver struct {
	mu sync.Mutex

	turnCount        int
	isTurnActive     bool
	isBotSpeaking    bool
	hasBotSpoken     bool
	turnStartTimeNs  int64
	turnEndTimeout   time.Duration
	endTurnTimer     *time.Timer
	onTurnStarted    func(turnCount int)
	onTurnEnded      func(turnCount int, durationSecs float64, interrupted bool)
	processedIDs     map[uint64]struct{}
	idHistory        []uint64
	maxFrames        int
}

// TurnTrackingOption configures TurnTrackingObserver.
type TurnTrackingOption func(*TurnTrackingObserver)

// TurnEndTimeout sets the timeout after bot stops speaking before ending the turn (default 2.5s).
func TurnEndTimeout(d time.Duration) TurnTrackingOption {
	return func(o *TurnTrackingObserver) {
		o.turnEndTimeout = d
	}
}

// MaxFrames sets the max frame IDs to keep for duplicate detection (default 100).
func MaxFrames(n int) TurnTrackingOption {
	return func(o *TurnTrackingObserver) {
		o.maxFrames = n
	}
}

// OnTurnStarted sets the callback when a new turn starts.
func OnTurnStarted(f func(turnCount int)) TurnTrackingOption {
	return func(o *TurnTrackingObserver) {
		o.onTurnStarted = f
	}
}

// OnTurnEnded sets the callback when a turn ends (durationSecs, interrupted).
func OnTurnEnded(f func(turnCount int, durationSecs float64, interrupted bool)) TurnTrackingOption {
	return func(o *TurnTrackingObserver) {
		o.onTurnEnded = f
	}
}

// NewTurnTrackingObserver returns a new TurnTrackingObserver.
func NewTurnTrackingObserver(opts ...TurnTrackingOption) *TurnTrackingObserver {
	o := &TurnTrackingObserver{
		turnEndTimeout: time.Duration(defaultTurnEndTimeoutSecs * float64(time.Second)),
		processedIDs:   make(map[uint64]struct{}),
		idHistory:      make([]uint64, 0, defaultMaxFrames),
		maxFrames:     defaultMaxFrames,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Ensure TurnTrackingObserver implements Observer.
var _ Observer = (*TurnTrackingObserver)(nil)

func (o *TurnTrackingObserver) seen(id uint64) bool {
	_, ok := o.processedIDs[id]
	return ok
}

func (o *TurnTrackingObserver) addProcessed(id uint64) {
	if _, ok := o.processedIDs[id]; ok {
		return
	}
	o.processedIDs[id] = struct{}{}
	o.idHistory = append(o.idHistory, id)
	for len(o.idHistory) > o.maxFrames {
		old := o.idHistory[0]
		o.idHistory = o.idHistory[1:]
		delete(o.processedIDs, old)
	}
}

func (o *TurnTrackingObserver) cancelTurnEndTimer() {
	if o.endTurnTimer != nil {
		o.endTurnTimer.Stop()
		o.endTurnTimer = nil
	}
}

func (o *TurnTrackingObserver) scheduleTurnEnd() {
	// Called with o.mu held. Schedule callback that will call endTurnLocked(false).
	o.cancelTurnEndTimer()
	timeout := o.turnEndTimeout
	timer := time.AfterFunc(timeout, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.isTurnActive && !o.isBotSpeaking {
			o.endTurnLocked(time.Now().UnixNano(), false)
		}
		o.endTurnTimer = nil
	})
	o.endTurnTimer = timer
}

func (o *TurnTrackingObserver) endTurnLocked(nowNs int64, interrupted bool) {
	if !o.isTurnActive {
		return
	}
	var durationSecs float64
	if o.turnStartTimeNs != 0 && nowNs != 0 {
		durationSecs = float64(nowNs-o.turnStartTimeNs) / 1e9
	} else {
		durationSecs = 0
	}
	o.isTurnActive = false
	count := o.turnCount
	onEnded := o.onTurnEnded
	o.mu.Unlock()
	if onEnded != nil {
		onEnded(count, durationSecs, interrupted)
	}
	o.mu.Lock()
}

func (o *TurnTrackingObserver) startTurnLocked(nowNs int64) {
	o.isTurnActive = true
	o.hasBotSpoken = false
	o.turnCount++
	o.turnStartTimeNs = nowNs
	count := o.turnCount
	onStarted := o.onTurnStarted
	o.mu.Unlock()
	if onStarted != nil {
		onStarted(count)
	}
	o.mu.Lock()
}

// OnFrameProcessed implements Observer.
func (o *TurnTrackingObserver) OnFrameProcessed(processorName string, f frames.Frame, dir processors.Direction) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if f == nil {
		return
	}
	id := f.ID()
	if o.seen(id) {
		return
	}
	o.addProcessed(id)
	nowNs := time.Now().UnixNano()

	switch f.(type) {
	case *frames.StartFrame:
		if o.turnCount == 0 {
			o.startTurnLocked(nowNs)
		}
	case *frames.UserStartedSpeakingFrame:
		if o.isBotSpeaking {
			o.cancelTurnEndTimer()
			o.endTurnLocked(nowNs, true)
			o.isBotSpeaking = false
			o.startTurnLocked(nowNs)
		} else if o.isTurnActive && o.hasBotSpoken {
			o.cancelTurnEndTimer()
			o.endTurnLocked(nowNs, false)
			o.startTurnLocked(nowNs)
		} else if !o.isTurnActive {
			o.startTurnLocked(nowNs)
		}
	case *frames.BotStartedSpeakingFrame:
		o.isBotSpeaking = true
		o.hasBotSpoken = true
		o.cancelTurnEndTimer()
	case *frames.BotStoppedSpeakingFrame:
		if o.isBotSpeaking {
			o.isBotSpeaking = false
			o.scheduleTurnEnd()
		}
	case *frames.CancelFrame:
		if o.isTurnActive {
			o.cancelTurnEndTimer()
			o.endTurnLocked(nowNs, true)
		}
	}
}
