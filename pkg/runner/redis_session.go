// Package runner provides Redis-backed session store for horizontal scaling.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultKeyPrefix    = "voila:session:"
	defaultRedisTimeout = 10 * time.Second
)

// RedisSessionStore stores sessions in Redis with a configurable TTL and key prefix.
type RedisSessionStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// RedisSessionStoreOptions configures a RedisSessionStore.
type RedisSessionStoreOptions struct {
	// Prefix is prepended to session IDs for Redis keys (default: "voila:session:").
	Prefix string
	// TTL is how long sessions live in Redis (default: 1 hour).
	TTL time.Duration
}

// NewRedisSessionStore creates a session store backed by the given Redis client.
func NewRedisSessionStore(client *redis.Client, opts RedisSessionStoreOptions) *RedisSessionStore {
	prefix := opts.Prefix
	if prefix == "" {
		prefix = defaultKeyPrefix
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &RedisSessionStore{client: client, prefix: prefix, ttl: ttl}
}

func (s *RedisSessionStore) key(id string) string {
	return s.prefix + id
}

// Put stores a session by id with TTL. Overwrites if id exists.
func (s *RedisSessionStore) Put(id string, sess *Session) error {
	if sess == nil {
		return fmt.Errorf("session cannot be nil")
	}
	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTimeout)
	defer cancel()
	if err := s.client.Set(ctx, s.key(id), data, s.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

// Get returns the session for id, or (nil, nil) if not found.
func (s *RedisSessionStore) Get(id string) (*Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTimeout)
	defer cancel()
	data, err := s.client.Get(ctx, s.key(id)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &sess, nil
}

// Delete removes the session for id. Idempotent.
func (s *RedisSessionStore) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisTimeout)
	defer cancel()
	if err := s.client.Del(ctx, s.key(id)).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

// Ping checks Redis connectivity. Used for readiness probes when session_store is redis.
func (s *RedisSessionStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}
