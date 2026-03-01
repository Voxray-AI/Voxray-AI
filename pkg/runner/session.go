// Package runner provides runner types and session management.
package runner

import (
	"sync"
)

// Session holds data for a runner session (created by POST /start).
type Session struct {
	// Body is the optional request body from /start.
	Body map[string]interface{}
	// EnableDefaultIceServers is set when the client requested default ICE config in /start.
	EnableDefaultIceServers bool
}

// SessionStore is the interface for storing and retrieving sessions by ID.
// Implementations may be in-memory (single instance) or backed by Redis (horizontal scaling).
type SessionStore interface {
	Put(id string, sess *Session) error
	Get(id string) (*Session, error)
	Delete(id string) error
}

// MemorySessionStore is an in-memory, concurrency-safe store for sessions by ID.
type MemorySessionStore struct {
	mu   sync.RWMutex
	sess map[string]*Session
}

// NewMemorySessionStore returns a new in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sess: make(map[string]*Session)}
}

// Put stores a session by id. Overwrites if id exists.
func (s *MemorySessionStore) Put(id string, sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess[id] = sess
	return nil
}

// Get returns the session for id, or nil if not found.
func (s *MemorySessionStore) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sess[id], nil
}

// Delete removes the session for id. Idempotent.
func (s *MemorySessionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sess, id)
	return nil
}
