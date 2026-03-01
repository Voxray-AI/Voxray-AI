// Package runner provides Pipecat-style runner types and session management.
package runner

import (
	"sync"
)

// Session holds data for a Pipecat-style session (created by POST /start).
type Session struct {
	// Body is the optional request body from /start.
	Body map[string]interface{}
	// EnableDefaultIceServers is set when the client requested default ICE config in /start.
	EnableDefaultIceServers bool
}

// SessionStore is an in-memory, concurrency-safe store for sessions by ID.
type SessionStore struct {
	mu   sync.RWMutex
	sess map[string]*Session
}

// NewSessionStore returns a new session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{sess: make(map[string]*Session)}
}

// Put stores a session by id. Overwrites if id exists.
func (s *SessionStore) Put(id string, sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess[id] = sess
}

// Get returns the session for id, or nil if not found.
func (s *SessionStore) Get(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sess[id]
}

// Delete removes the session for id. Idempotent.
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sess, id)
}
