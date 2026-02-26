// Package storage provides storage management with multipart upload support.
package storage

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// ShardTracker tracks the state of individual shards in a multipart upload
// using Redis Bitmap operations for efficient storage and lookup.
type ShardTracker interface {
	// InitUpload initialises tracking state for a new upload with the given total shard count.
	InitUpload(ctx context.Context, uploadID string, totalShards int) error

	// MarkShardComplete marks a single shard as successfully uploaded.
	MarkShardComplete(ctx context.Context, uploadID string, shardIndex int) error

	// IsShardComplete reports whether the given shard has been uploaded.
	IsShardComplete(ctx context.Context, uploadID string, shardIndex int) (bool, error)

	// GetCompletedCount returns the number of shards that have been uploaded so far.
	GetCompletedCount(ctx context.Context, uploadID string) (int, error)

	// IsUploadComplete reports whether every shard in the upload has been received.
	IsUploadComplete(ctx context.Context, uploadID string) (bool, error)

	// GetUploadProgress returns a detailed progress snapshot for the upload.
	GetUploadProgress(ctx context.Context, uploadID string) (*UploadProgress, error)

	// CleanupUpload removes all tracking state associated with the upload.
	CleanupUpload(ctx context.Context, uploadID string) error
}

// UploadProgress holds a point-in-time snapshot of shard upload progress.
type UploadProgress struct {
	UploadID       string
	TotalShards    int
	CompletedShards int
	IsComplete     bool
	PendingShards  []int
}

// RedisShardTracker implements ShardTracker using Redis Bitmap operations.
// Each shard index maps to a bit offset in a Redis bitmap key, providing
// O(1) mark/check operations and O(N) counting via BITCOUNT.
type RedisShardTracker struct {
	client *redis.Client
}

// NewRedisShardTracker creates a new RedisShardTracker backed by the given Redis client.
func NewRedisShardTracker(client *redis.Client) *RedisShardTracker {
	return &RedisShardTracker{client: client}
}

// shardBitmapKey returns the Redis key used to store the shard bitmap.
func shardBitmapKey(uploadID string) string {
	return fmt.Sprintf("upload:%s:shards", uploadID)
}

// shardTotalKey returns the Redis key used to store the total shard count.
func shardTotalKey(uploadID string) string {
	return fmt.Sprintf("upload:%s:total", uploadID)
}

// InitUpload stores the total shard count for a new upload. The bitmap key is
// implicitly created when the first shard is marked complete.
func (r *RedisShardTracker) InitUpload(ctx context.Context, uploadID string, totalShards int) error {
	if totalShards <= 0 {
		return fmt.Errorf("totalShards must be positive, got %d", totalShards)
	}
	return r.client.Set(ctx, shardTotalKey(uploadID), strconv.Itoa(totalShards), 0).Err()
}

// MarkShardComplete sets the bit at shardIndex to 1 in the bitmap.
func (r *RedisShardTracker) MarkShardComplete(ctx context.Context, uploadID string, shardIndex int) error {
	totalShards, err := r.getTotalShards(ctx, uploadID)
	if err != nil {
		return err
	}
	if shardIndex < 0 || shardIndex >= totalShards {
		return fmt.Errorf("shardIndex %d out of range [0, %d)", shardIndex, totalShards)
	}
	return r.client.SetBit(ctx, shardBitmapKey(uploadID), int64(shardIndex), 1).Err()
}

// IsShardComplete returns true if the bit at shardIndex is 1.
func (r *RedisShardTracker) IsShardComplete(ctx context.Context, uploadID string, shardIndex int) (bool, error) {
	val, err := r.client.GetBit(ctx, shardBitmapKey(uploadID), int64(shardIndex)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to get shard status: %w", err)
	}
	return val == 1, nil
}

// GetCompletedCount returns the number of bits set to 1 in the bitmap.
func (r *RedisShardTracker) GetCompletedCount(ctx context.Context, uploadID string) (int, error) {
	count, err := r.client.BitCount(ctx, shardBitmapKey(uploadID), nil).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count completed shards: %w", err)
	}
	return int(count), nil
}

// IsUploadComplete returns true when every shard has been marked complete.
func (r *RedisShardTracker) IsUploadComplete(ctx context.Context, uploadID string) (bool, error) {
	totalShards, err := r.getTotalShards(ctx, uploadID)
	if err != nil {
		return false, err
	}

	completed, err := r.GetCompletedCount(ctx, uploadID)
	if err != nil {
		return false, err
	}

	return completed >= totalShards, nil
}

// GetUploadProgress builds a full progress snapshot including pending shard indices.
func (r *RedisShardTracker) GetUploadProgress(ctx context.Context, uploadID string) (*UploadProgress, error) {
	totalShards, err := r.getTotalShards(ctx, uploadID)
	if err != nil {
		return nil, err
	}

	completed := 0
	var pending []int
	for i := 0; i < totalShards; i++ {
		done, err := r.IsShardComplete(ctx, uploadID, i)
		if err != nil {
			return nil, err
		}
		if done {
			completed++
		} else {
			pending = append(pending, i)
		}
	}

	return &UploadProgress{
		UploadID:        uploadID,
		TotalShards:     totalShards,
		CompletedShards: completed,
		IsComplete:      completed >= totalShards,
		PendingShards:   pending,
	}, nil
}

// CleanupUpload deletes both the bitmap and total-count keys for the upload.
func (r *RedisShardTracker) CleanupUpload(ctx context.Context, uploadID string) error {
	return r.client.Del(ctx, shardBitmapKey(uploadID), shardTotalKey(uploadID)).Err()
}

// getTotalShards reads and parses the stored total shard count.
func (r *RedisShardTracker) getTotalShards(ctx context.Context, uploadID string) (int, error) {
	val, err := r.client.Get(ctx, shardTotalKey(uploadID)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get total shards for upload %s: %w", uploadID, err)
	}
	total, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid total shards value %q: %w", val, err)
	}
	return total, nil
}
