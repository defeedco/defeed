package lib

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type cacheEntry struct {
	value      any
	expiration time.Time
}

type Cache struct {
	logger  *zerolog.Logger
	entries map[string]cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

func NewCache(ttl time.Duration, logger *zerolog.Logger) *Cache {
	return &Cache{
		logger:  logger,
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiration) {
		return nil, false
	}

	c.logger.Trace().
		Str("key", key).
		Msg("cache hit")

	return entry.value, true
}

func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]cacheEntry)
}

func HashParams(params ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(params, ",")))
	return fmt.Sprintf("%x", hash)
}
