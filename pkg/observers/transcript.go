package observers

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
	"strings"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/transcripts"
)

const transcriptWriterQueueSize = 64

// transcriptMsg is a single message to be written by the async writer.
type transcriptMsg struct {
	role string
	text string
	at   time.Time
	seq  int64
}

// TranscriptObserver records user and assistant messages for a session.
// When started with Start(ctx), DB writes are done asynchronously so the pipeline
// never blocks on SaveMessage; the session context allows cancellation on session end.
type TranscriptObserver struct {
	store     transcripts.Store
	sessionID string

	mu     sync.Mutex
	seq    int64
	botBuf strings.Builder

	// Async writer: when non-nil, OnFrameProcessed enqueues here instead of calling SaveMessage.
	msgCh   chan transcriptMsg
	ctx     context.Context
	cancel  context.CancelFunc
	worker  int32 // 1 when async worker is running
	closed  int32
}

// NewTranscriptObserver creates a new TranscriptObserver for a session.
// For async DB writes (recommended), call Start(ctx) with the session context so
// SaveMessage runs in a worker and does not block the pipeline; call Close() when the session ends.
func NewTranscriptObserver(store transcripts.Store, sessionID string) *TranscriptObserver {
	if store == nil || sessionID == "" {
		return nil
	}
	return &TranscriptObserver{
		store:     store,
		sessionID: sessionID,
	}
}

// NewTranscriptObserverWithContext creates a TranscriptObserver that writes to the store
// asynchronously using the given context. The context should be the session context so
// that when the session ends, pending writes are cancelled. Call Close() when the session ends.
func NewTranscriptObserverWithContext(store transcripts.Store, sessionID string, ctx context.Context) *TranscriptObserver {
	if store == nil || sessionID == "" || ctx == nil {
		return nil
	}
	obs := &TranscriptObserver{
		store:     store,
		sessionID: sessionID,
		msgCh:     make(chan transcriptMsg, transcriptWriterQueueSize),
	}
	obs.ctx, obs.cancel = context.WithCancel(ctx)
	if atomic.CompareAndSwapInt32(&obs.worker, 0, 1) {
		go obs.runWriter()
	}
	return obs
}

// Start starts the async writer with the given session context. SaveMessage will run in
// a background goroutine so the pipeline is not blocked. Call Close() when the session ends.
// If Start is not called, writes are done synchronously (legacy behavior).
func (o *TranscriptObserver) Start(ctx context.Context) {
	if o == nil || ctx == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.msgCh != nil {
		return // already started
	}
	o.msgCh = make(chan transcriptMsg, transcriptWriterQueueSize)
	o.ctx, o.cancel = context.WithCancel(ctx)
	if atomic.CompareAndSwapInt32(&o.worker, 0, 1) {
		go o.runWriter()
	}
}

func (o *TranscriptObserver) runWriter() {
	defer atomic.StoreInt32(&o.worker, 0)
	for {
		select {
		case <-o.ctx.Done():
			return
		case m, ok := <-o.msgCh:
			if !ok {
				return
			}
			_ = o.store.SaveMessage(o.ctx, o.sessionID, m.role, m.text, m.at, m.seq)
		}
	}
}

// Close stops the async writer and waits for pending writes to drain (or context cancel).
// Safe to call multiple times. No-op if Start was never called.
func (o *TranscriptObserver) Close() error {
	if o == nil {
		return nil
	}
	if !atomic.CompareAndSwapInt32(&o.closed, 0, 1) {
		return nil
	}
	if o.cancel != nil {
		o.cancel()
	}
	if o.msgCh != nil {
		close(o.msgCh)
		o.msgCh = nil
	}
	// Wait for worker to exit (brief spin; runWriter exits on ctx.Done or channel close)
	for atomic.LoadInt32(&o.worker) == 1 {
		// yield
	}
	return nil
}

// Ensure TranscriptObserver implements Observer.
var _ Observer = (*TranscriptObserver)(nil)

// OnFrameProcessed implements Observer.
func (o *TranscriptObserver) OnFrameProcessed(processorName string, f frames.Frame, dir processors.Direction) {
	if o == nil || o.store == nil {
		return
	}
	if dir != processors.Downstream {
		return
	}
	if f == nil {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	switch t := f.(type) {
	case *frames.TranscriptionFrame:
		if !t.Finalized || t.Text == "" {
			return
		}
		o.seq++
		o.enqueueLocked("user", t.Text, time.Now().UTC(), o.seq)
	case *frames.LLMTextFrame:
		if t.Text == "" {
			return
		}
		o.botBuf.WriteString(t.Text)
	case *frames.TTSSpeakFrame:
		o.flushAssistantLocked()
	case *frames.EndFrame, *frames.CancelFrame:
		o.flushAssistantLocked()
	}
}

func (o *TranscriptObserver) enqueueLocked(role, text string, at time.Time, seq int64) {
	if o.msgCh != nil && atomic.LoadInt32(&o.closed) == 0 {
		select {
		case o.msgCh <- transcriptMsg{role: role, text: text, at: at, seq: seq}:
		case <-o.ctx.Done():
			// Session ended, skip write
		}
		return
	}
	// Sync path when Start was not called
	_ = o.store.SaveMessage(context.Background(), o.sessionID, role, text, at, seq)
}

func (o *TranscriptObserver) flushAssistantLocked() {
	if o.botBuf.Len() == 0 {
		return
	}
	text := o.botBuf.String()
	o.botBuf.Reset()
	if text == "" {
		return
	}
	o.seq++
	o.enqueueLocked("assistant", text, time.Now().UTC(), o.seq)
}

