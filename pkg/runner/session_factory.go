package runner

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"voxray-go/pkg/config"
)

const defaultSessionTTLSecs = 3600

// NewSessionStoreFromConfig returns a SessionStore based on cfg.
// When session_store is "" or "memory", returns an in-memory store (single instance / vertical scaling).
// When session_store is "redis", requires redis_url and returns a Redis-backed store (horizontal scaling).
func NewSessionStoreFromConfig(cfg *config.Config) (SessionStore, error) {
	if cfg == nil {
		return NewMemorySessionStore(), nil
	}
	storeType := cfg.SessionStore
	if storeType == "" {
		storeType = "memory"
	}
	switch storeType {
	case "memory":
		return NewMemorySessionStore(), nil
	case "redis":
		if cfg.RedisURL == "" {
			return nil, fmt.Errorf("session_store is redis but redis_url is empty")
		}
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis_url: %w", err)
		}
		client := redis.NewClient(opts)
		ttlSecs := cfg.SessionTTLSecs
		if ttlSecs <= 0 {
			ttlSecs = defaultSessionTTLSecs
		}
		return NewRedisSessionStore(client, RedisSessionStoreOptions{
			TTL: time.Duration(ttlSecs) * time.Second,
		}), nil
	default:
		return nil, fmt.Errorf("unknown session_store: %q (use memory or redis)", storeType)
	}
}
