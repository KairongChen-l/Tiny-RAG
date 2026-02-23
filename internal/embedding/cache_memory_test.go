package embedding

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_BasicSetGet(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 100,
	})

	ctx := context.Background()
	key := "test-key"
	vec := []float32{1.0, 2.0, 3.0}

	if err := cache.Set(ctx, key, vec); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, found, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("expected to find cached entry")
	}
	if len(got) != len(vec) {
		t.Fatalf("expected vector length %d, got %d", len(vec), len(got))
	}
	for i := range vec {
		if got[i] != vec[i] {
			t.Errorf("vector[%d] = %f, expected %f", i, got[i], vec[i])
		}
	}
}

func TestMemoryCache_VectorIsolation(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 100,
	})

	ctx := context.Background()
	key := "test-key"
	vec := []float32{1.0, 2.0, 3.0}

	if err := cache.Set(ctx, key, vec); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Mutate original vector
	vec[0] = 999.0

	// Cached value should not be affected
	got, found, _ := cache.Get(ctx, key)
	if !found {
		t.Fatal("expected to find cached entry")
	}
	if got[0] == 999.0 {
		t.Error("cache entry was mutated by external change to input vector")
	}
	if got[0] != 1.0 {
		t.Errorf("expected cached vector[0] = 1.0, got %f", got[0])
	}

	// Mutate returned vector
	got[0] = 888.0

	// Second get should still return original
	got2, found2, _ := cache.Get(ctx, key)
	if !found2 {
		t.Fatal("expected to find cached entry on second get")
	}
	if got2[0] != 1.0 {
		t.Errorf("cache entry was mutated by external change to returned vector: got %f", got2[0])
	}
}

func TestMemoryCache_Eviction(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 3,
	})

	ctx := context.Background()

	// Fill cache to max
	for i := 0; i < 3; i++ {
		key := string(rune('a' + i))
		vec := []float32{float32(i)}
		if err := cache.Set(ctx, key, vec); err != nil {
			t.Fatalf("Set failed for key %s: %v", key, err)
		}
	}

	// Adding one more should evict one
	if err := cache.Set(ctx, "d", []float32{3.0}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should have exactly maxSize entries
	cache.mu.RLock()
	count := len(cache.entries)
	cache.mu.RUnlock()

	if count != 3 {
		t.Errorf("expected %d entries after eviction, got %d", 3, count)
	}

	// The new entry should be present
	_, found, _ := cache.Get(ctx, "d")
	if !found {
		t.Error("newly added entry should be present")
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     50 * time.Millisecond,
		MaxSize: 100,
	})

	ctx := context.Background()
	if err := cache.Set(ctx, "key", []float32{1.0}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should be found immediately
	_, found, _ := cache.Get(ctx, "key")
	if !found {
		t.Error("entry should be found immediately after set")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, found, _ = cache.Get(ctx, "key")
	if found {
		t.Error("entry should be expired after TTL")
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 100,
	})

	ctx := context.Background()
	_ = cache.Set(ctx, "a", []float32{1.0})
	_ = cache.Set(ctx, "b", []float32{2.0})

	if err := cache.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	_, found, _ := cache.Get(ctx, "a")
	if found {
		t.Error("cache should be empty after clear")
	}
}

func TestMemoryCache_EvictsOldest(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 2,
	})

	ctx := context.Background()

	// Add first entry with earlier expiration
	_ = cache.Set(ctx, "first", []float32{1.0})
	// Manually set earlier expiration to ensure deterministic eviction
	cache.mu.Lock()
	cache.entries["first"].expiresAt = time.Now().Add(10 * time.Minute)
	cache.mu.Unlock()

	// Add second entry with later expiration
	_ = cache.Set(ctx, "second", []float32{2.0})

	// Add third entry - should evict "first" (earliest expiration)
	_ = cache.Set(ctx, "third", []float32{3.0})

	_, foundFirst, _ := cache.Get(ctx, "first")
	_, foundSecond, _ := cache.Get(ctx, "second")
	_, foundThird, _ := cache.Get(ctx, "third")

	if foundFirst {
		t.Error("oldest entry (first) should have been evicted")
	}
	if !foundSecond {
		t.Error("second entry should still be present")
	}
	if !foundThird {
		t.Error("third entry should be present")
	}
}
