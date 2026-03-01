package turn

import (
	"context"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
)

// UserTurnController manages high-level user turn state: when a user turn
// starts and stops, and when the user has been idle for a configured timeout.
// It is a lightweight analogue of UserTurnController + UserIdleController,
// adapted to the existing Go pipeline.
type UserTurnController struct {
	startStrategy UserTurnStartStrategy
	stopStrategy  UserTurnStopStrategy

	// timeouts (in seconds)
	userTurnStopTimeout float64
	userIdleTimeout     float64

	// state
	userSpeaking bool
	userTurn     bool

	// callbacks into the owning processor
	onPushFrame func(ctx context.Context, f frames.Frame) error

	// timers
	stopTimer *time.Timer
	idleTimer *time.Timer
}

// NewUserTurnController creates a new controller with the given strategies
// and timeouts.
func NewUserTurnController(
	start UserTurnStartStrategy,
	stop UserTurnStopStrategy,
	userTurnStopTimeout float64,
	userIdleTimeout float64,
	onPushFrame func(ctx context.Context, f frames.Frame) error,
) *UserTurnController {
	return &UserTurnController{
		startStrategy:       start,
		stopStrategy:        stop,
		userTurnStopTimeout: userTurnStopTimeout,
		userIdleTimeout:     userIdleTimeout,
		onPushFrame:         onPushFrame,
	}
}

// ProcessVADUpdate should be called by the owning processor when VAD
// indicates whether the user is currently speaking.
func (c *UserTurnController) ProcessVADUpdate(ctx context.Context, isSpeech bool) error {
	if isSpeech {
		if !c.userSpeaking {
			c.userSpeaking = true
			c.startStrategy.OnUserStartedSpeaking()
			// Always emit UserStartedSpeakingFrame on transition to speech so barge-in works
			// even when we are still in a user turn (e.g. user interrupts bot before stop timeout).
			if c.onPushFrame != nil {
				logger.Info("turn: user started speaking (barge-in=%v)", c.userTurn)
				if err := c.onPushFrame(ctx, frames.NewUserStartedSpeakingFrame()); err != nil {
					return err
				}
			}
			if !c.userTurn && c.startStrategy.ShouldStartTurn() {
				if err := c.startTurn(ctx); err != nil {
					return err
				}
			}
		}
		// Any speech activity resets the stop and idle timers.
		c.resetStopTimer(ctx)
		c.stopIdleTimer()
	} else {
		if c.userSpeaking {
			c.userSpeaking = false
			c.stopStrategy.OnUserStoppedSpeaking()
			if c.userTurn && c.stopStrategy.ShouldStopTurn() {
				if err := c.stopTurn(ctx); err != nil {
					return err
				}
			}
			c.resetStopTimer(ctx)
		}
	}
	return nil
}

// NotifyBotStoppedSpeaking should be called when the bot finishes speaking,
// so that idle detection can begin.
func (c *UserTurnController) NotifyBotStoppedSpeaking(ctx context.Context) {
	if c.userIdleTimeout <= 0 {
		return
	}
	if c.userTurn {
		// Do not start idle timer while user turn is in progress.
		return
	}
	c.resetIdleTimer(ctx)
}

// NotifyBotStartedSpeaking cancels any pending idle detection.
func (c *UserTurnController) NotifyBotStartedSpeaking() {
	c.stopIdleTimer()
}

func (c *UserTurnController) startTurn(ctx context.Context) error {
	if c.userTurn {
		return nil
	}
	c.userTurn = true
	c.stopStrategy.Reset()
	c.startStrategy.Reset()
	// Start/refresh stop timer. UserStartedSpeakingFrame is pushed in ProcessVADUpdate on transition to speech.
	c.resetStopTimer(ctx)
	return nil
}

func (c *UserTurnController) stopTurn(ctx context.Context) error {
	if !c.userTurn {
		return nil
	}
	c.userTurn = false
	c.startStrategy.Reset()
	c.stopStrategy.Reset()
	c.stopStopTimer()
	if c.onPushFrame != nil {
		if err := c.onPushFrame(ctx, frames.NewUserStoppedSpeakingFrame()); err != nil {
			return err
		}
	}
	// When a turn fully stops and the bot has already spoken, an idle timer
	// may be scheduled via NotifyBotStoppedSpeaking.
	return nil
}

func (c *UserTurnController) resetStopTimer(ctx context.Context) {
	if c.userTurnStopTimeout <= 0 {
		return
	}
	if c.stopTimer != nil {
		c.stopTimer.Stop()
	}
	timeout := c.userTurnStopTimeout
	c.stopTimer = time.AfterFunc(time.Duration(timeout*float64(time.Second)), func() {
		// On timeout, if user is not speaking but a turn is active, stop it.
		if !c.userSpeaking && c.userTurn {
			_ = c.stopTurn(ctx)
		}
	})
}

func (c *UserTurnController) stopStopTimer() {
	if c.stopTimer != nil {
		c.stopTimer.Stop()
		c.stopTimer = nil
	}
}

func (c *UserTurnController) resetIdleTimer(ctx context.Context) {
	if c.userIdleTimeout <= 0 {
		return
	}
	if c.idleTimer != nil {
		c.idleTimer.Stop()
	}
	timeout := c.userIdleTimeout
	c.idleTimer = time.AfterFunc(time.Duration(timeout*float64(time.Second)), func() {
		if c.onPushFrame != nil {
			_ = c.onPushFrame(ctx, frames.NewUserIdleFrame())
		}
	})
}

func (c *UserTurnController) stopIdleTimer() {
	if c.idleTimer != nil {
		c.idleTimer.Stop()
		c.idleTimer = nil
	}
}

