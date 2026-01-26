package embedding

import (
	"context"
	"sync"
	"time"
)

// MemoryCache implements Cache using in-memory storage.
type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
	maxSize int // Maximum number of entries
}

// cacheEntry represents a cached embedding entry.
type cacheEntry struct {
	vector    []float32
	expiresAt time.Time
}

// MemoryCacheConfig holds configuration for memory cache.
type MemoryCacheConfig struct {
	TTL     time.Duration // Time to live (default: 24 hours)
	MaxSize int          // Maximum number of entries (default: 10000)
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(cfg MemoryCacheConfig) *MemoryCache {
	if cfg.TTL == 0 {
		cfg.TTL = 24 * time.Hour
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 10000
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

// Get retrieves cached embedding.
func (m *MemoryCache) Get(ctx context.Context, key string) ([]float32, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.entries[key]
	if !ok {
		return nil, false, nil
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		// Entry expired, but don't delete here (cleanup goroutine will handle it)
		return nil, false, nil
	}

	// Return copy of vector
	result := make([]float32, len(entry.vector))
	copy(result, entry.vector)
	return result, true, nil
}

// Set stores an embedding in cache.
func (m *MemoryCache) Set(ctx context.Context, key string, vector []float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we need to evict entries
	if len(m.entries) >= m.maxSize {
		m.evictOldest()
	}

	// Store entry
	m.entries[key] = &cacheEntry{
		vector:    vector,
		expiresAt: time.Now().Add(m.ttl),
	}

	return nil
}

// Clear clears all cached embeddings.
func (m *MemoryCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make(map[string]*cacheEntry)
	return nil
}

// evictOldest removes the oldest entry (simple FIFO eviction).
func (m *MemoryCache) evictOldest() {
	// Simple eviction: remove first entry (not perfect but fast)
	for key := range m.entries {
		delete(m.entries, key)
		break
	}
}

// cleanup periodically removes expired entries.
func (m *MemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
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

