package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Cache defines the interface for query result cache.
type Cache interface {
	// Get retrieves cached query result.
	Get(ctx context.Context, key string) ([]byte, bool, error)

	// Set stores a query result in cache.
	Set(ctx context.Context, key string, value []byte) error

	// Clear clears all cached results.
	Clear(ctx context.Context) error
}

// CacheKey generates a cache key for a query.
func CacheKey(queryType, query string, options map[string]interface{}) string {
	// Serialize options to JSON for consistent hashing
	optsJSON, _ := json.Marshal(options)
	
	// Create hash of query and options
	data := fmt.Sprintf("%s:%s:%s", queryType, query, string(optsJSON))
	hash := sha256.Sum256([]byte(data))
	
	return fmt.Sprintf("query:%s:%s", queryType, hex.EncodeToString(hash[:]))
}

// MemoryCacheConfig holds configuration for memory cache.
type MemoryCacheConfig struct {
	TTL     time.Duration // Time to live (default: 1 hour)
	MaxSize int          // Maximum number of entries (default: 1000)
}

// cacheEntry represents a cached entry.
type cacheEntry struct {
	value     []byte
	expiresAt time.Time
	createdAt time.Time
}

// MemoryCache implements Cache using in-memory storage.
type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
	maxSize int
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(cfg MemoryCacheConfig) *MemoryCache {
	if cfg.TTL == 0 {
		cfg.TTL = 1 * time.Hour
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 1000
	}

	cache := &MemoryCache{
		entries: make(map[string]*cacheEntry),
		ttl:     cfg.TTL,
		maxSize: cfg.MaxSize,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves cached query result.
func (m *MemoryCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.entries[key]
	if !ok {
		return nil, false, nil
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		return nil, false, nil
	}

	// Return copy of value
	result := make([]byte, len(entry.value))
	copy(result, entry.value)
	return result, true, nil
}

// Set stores a query result in cache.
func (m *MemoryCache) Set(ctx context.Context, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we need to evict entries
	if len(m.entries) >= m.maxSize {
		m.evictOldest()
	}

	// Store entry
	m.entries[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(m.ttl),
		createdAt: time.Now(),
	}

	return nil
}

// Clear clears all cached results.
func (m *MemoryCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make(map[string]*cacheEntry)
	return nil
}

// evictOldest evicts the oldest entry (LRU-like).
func (m *MemoryCache) evictOldest() {
	if len(m.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range m.entries {
		if first || entry.createdAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.createdAt
			first = false
		}
	}

	if oldestKey != "" {
		delete(m.entries, oldestKey)
	}
}

// cleanup periodically removes expired entries.
func (m *MemoryCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for key, entry := range m.entries {
			if now.After(entry.expiresAt) {
				delete(m.entries, key)
			}
		}
		m.mu.Unlock()
	}
}

