package handler

import (
	"context"
	"io"
)

// ObjectStore is a minimal abstraction used by handlers/workers to persist uploaded files.
// It is intentionally small to avoid bleeding storage concerns into core logic.
type ObjectStore interface {
	Put(ctx context.Context, objectName string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, objectName string) (io.ReadCloser, error)
	Delete(ctx context.Context, objectName string) error
}

// EventPublisher is used to publish lifecycle events (e.g., to Kafka).
type EventPublisher interface {
	Publish(ctx context.Context, topic string, key string, value any) error
}


