package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestMemoryCache_GetSet(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 100,
	})

	ctx := context.Background()
	key := "test-key"
	value := map[string]interface{}{
		"answer": "test answer",
		"tokens": 100,
	}

	// Set value
	data, _ := json.Marshal(value)
	if err := cache.Set(ctx, key, data); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Get value
	cached, found, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to get cache: %v", err)
	}
	if !found {
		t.Error("expected cache hit")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(cached, &result); err != nil {
		t.Fatalf("failed to unmarshal cached value: %v", err)
	}

	if result["answer"] != "test answer" {
		t.Errorf("expected answer 'test answer', got %v", result["answer"])
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     100 * time.Millisecond,
		MaxSize: 100,
	})

	ctx := context.Background()
	key := "test-key"
	value := []byte("test value")

	// Set value
	if err := cache.Set(ctx, key, value); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Get immediately (should be found)
	_, found, _ := cache.Get(ctx, key)
	if !found {
		t.Error("expected cache hit before expiration")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Get after expiration (should not be found)
	_, found, _ = cache.Get(ctx, key)
	if found {
		t.Error("expected cache miss after expiration")
	}
}

func TestMemoryCache_MaxSize(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 3,
	})

	ctx := context.Background()

	// Fill cache to max size
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		if err := cache.Set(ctx, key, value); err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}
	}

	// Add one more (should evict oldest)
	if err := cache.Set(ctx, "key-3", []byte("value-3")); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// First key should be evicted
	_, found, _ := cache.Get(ctx, "key-0")
	if found {
		t.Error("expected key-0 to be evicted")
	}

	// Last key should still be there
	_, found, _ = cache.Get(ctx, "key-3")
	if !found {
		t.Error("expected key-3 to be in cache")
	}
}

func TestCacheKey_Consistent(t *testing.T) {
	key1 := CacheKey("query", "test query", map[string]interface{}{
		"top_k": 5,
		"filter": map[string]string{"source": "doc1"},
	})
	key2 := CacheKey("query", "test query", map[string]interface{}{
		"top_k": 5,
		"filter": map[string]string{"source": "doc1"},
	})

	if key1 != key2 {
		t.Error("expected same cache key for same inputs")
	}
}

func TestCacheKey_Different(t *testing.T) {
	key1 := CacheKey("query", "test query", map[string]interface{}{
		"top_k": 5,
	})
	key2 := CacheKey("query", "test query", map[string]interface{}{
		"top_k": 10,
	})

	if key1 == key2 {
		t.Error("expected different cache keys for different inputs")
	}
}

