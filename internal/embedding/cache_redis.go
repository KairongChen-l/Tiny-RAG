package embedding

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Cache using Redis.
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

// RedisCacheConfig holds configuration for Redis cache.
type RedisCacheConfig struct {
	Addr     string        // Redis server address (default: "localhost:6379")
	Password string        // Redis password (optional)
	DB       int           // Redis database number (default: 0)
	TTL      time.Duration // Time to live (default: 24 hours)
}

// NewRedisCache creates a new Redis cache.
func NewRedisCache(cfg RedisCacheConfig) (*RedisCache, error) {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:6379"
	}
	if cfg.TTL == 0 {
		cfg.TTL = 24 * time.Hour
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		ttl:    cfg.TTL,
	}, nil
}

// Get retrieves cached embedding.
func (r *RedisCache) Get(ctx context.Context, key string) ([]float32, bool, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil // Key not found
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get error: %w", err)
	}

	// Deserialize vector
	var vector []float32
	if err := json.Unmarshal([]byte(val), &vector); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal vector: %w", err)
	}

	return vector, true, nil
}

// Set stores an embedding in cache.
func (r *RedisCache) Set(ctx context.Context, key string, vector []float32) error {
	// Serialize vector
	data, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("failed to marshal vector: %w", err)
	}

	// Store with TTL
	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}

	return nil
}

// Clear clears all cached embeddings.
func (r *RedisCache) Clear(ctx context.Context) error {
	// Delete all keys matching the pattern
	iter := r.client.Scan(ctx, 0, "embed:*", 0).Iterator()
	for iter.Next(ctx) {
		if err := r.client.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("failed to delete key %s: %w", iter.Val(), err)
		}
	}
	return iter.Err()
}

// Close closes the Redis connection.
func (r *RedisCache) Close() error {
	return r.client.Close()
}

