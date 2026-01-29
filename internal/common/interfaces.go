// Package common provides shared interfaces used across multiple packages.
package common

import (
	"context"
	"io"
)

// ObjectStore is a minimal abstraction for object storage operations.
// Used by handlers and workers to persist uploaded files.
type ObjectStore interface {
	Put(ctx context.Context, objectName string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, objectName string) (io.ReadCloser, error)
	Delete(ctx context.Context, objectName string) error
}

// EventPublisher is used to publish lifecycle events (e.g., to Kafka).
type EventPublisher interface {
	Publish(ctx context.Context, topic string, key string, value any) error
}


