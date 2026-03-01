package filters

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// WakeCheckFilterOptions is the JSON shape for plugin_options["wake_check_filter"].
type WakeCheckFilterOptions struct {
	WakePhrases    []string  `json:"wake_phrases"`
	KeepaliveSecs  float64   `json:"keepalive_secs"`
}

const defaultKeepaliveSecs = 3

type wakeState int

const (
	wakeStateIdle wakeState = iota
	wakeStateAwake
)

type participantState struct {
	state      wakeState
	wakeTimer  time.Time
	accumulator string
}

// WakeCheckFilter passes TranscriptionFrames only after a wake phrase has been detected for that participant.
// Other frame types always pass. After wake, frames pass for keepaliveTimeout; then state resets to idle.
type WakeCheckFilter struct {
	*processors.BaseProcessor
	patterns         []*regexp.Regexp
	keepaliveTimeout time.Duration
	mu               sync.Mutex
	participants     map[string]*participantState
}

// NewWakeCheckFilter returns a wake-phrase filter. wakePhrases are matched as word-boundary phrases (case-insensitive).
func NewWakeCheckFilter(name string, wakePhrases []string, keepaliveTimeout time.Duration) *WakeCheckFilter {
	if name == "" {
		name = "WakeCheckFilter"
	}
	if keepaliveTimeout <= 0 {
		keepaliveTimeout = defaultKeepaliveSecs * time.Second
	}
	patterns := make([]*regexp.Regexp, 0, len(wakePhrases))
	for _, phrase := range wakePhrases {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		words := strings.Fields(phrase)
		parts := make([]string, 0, len(words))
		for _, w := range words {
			parts = append(parts, regexp.QuoteMeta(w))
		}
		re, err := regexp.Compile(`(?i)\b` + strings.Join(parts, `\s*`) + `\b`)
		if err != nil {
			continue
		}
		patterns = append(patterns, re)
	}
	return &WakeCheckFilter{
		BaseProcessor:     processors.NewBaseProcessor(name),
		patterns:           patterns,
		keepaliveTimeout:   keepaliveTimeout,
		participants:       make(map[string]*participantState),
	}
}

// NewWakeCheckFilterFromOptions builds from plugin_options.
func NewWakeCheckFilterFromOptions(name string, opts json.RawMessage) *WakeCheckFilter {
	var o WakeCheckFilterOptions
	if len(opts) > 0 {
		_ = json.Unmarshal(opts, &o)
	}
	if o.KeepaliveSecs <= 0 {
		o.KeepaliveSecs = defaultKeepaliveSecs
	}
	return NewWakeCheckFilter(name, o.WakePhrases, time.Duration(o.KeepaliveSecs*float64(time.Second)))
}

// ProcessFrame forwards non-TranscriptionFrame as-is. For TranscriptionFrame, forwards only after wake for that user_id, with keepalive.
func (p *WakeCheckFilter) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	tf, ok := f.(*frames.TranscriptionFrame)
	if !ok {
		return p.BaseProcessor.ProcessFrame(ctx, f, dir)
	}

	p.mu.Lock()
	ps, ok := p.participants[tf.UserID]
	if !ok {
		ps = &participantState{}
		p.participants[tf.UserID] = ps
	}

	if ps.state == wakeStateAwake {
		if time.Since(ps.wakeTimer) < p.keepaliveTimeout {
			ps.wakeTimer = time.Now()
			p.mu.Unlock()
			return p.BaseProcessor.ProcessFrame(ctx, f, dir)
		}
		ps.state = wakeStateIdle
	}

	ps.accumulator += tf.Text
	matched := false
	for _, re := range p.patterns {
		loc := re.FindStringIndex(ps.accumulator)
		if loc != nil {
			matched = true
			ps.state = wakeStateAwake
			ps.wakeTimer = time.Now()
			tf.Text = ps.accumulator[loc[1]:]
			ps.accumulator = ""
			break
		}
	}
	p.mu.Unlock()
	if !matched {
		return nil
	}
	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}
