// Package bootstrap provides application initialization and resource management.
package bootstrap

import (
	"context"
	"io"
	"sync"

	"go.uber.org/zap"

	"github.com/krc/rag/pkg/messaging"
	"github.com/krc/rag/pkg/search"
	"github.com/krc/rag/pkg/storage"
)

// ResourceManager manages application resources that need graceful shutdown.
type ResourceManager struct {
	mu            sync.Mutex
	resources     []Resource
	logger        *zap.Logger
	shutdownHooks []func(ctx context.Context) error
}

// Resource represents a closable resource.
type Resource interface {
	Close() error
}

// NewResourceManager creates a new resource manager.
func NewResourceManager(logger *zap.Logger) *ResourceManager {
	return &ResourceManager{
		resources: make([]Resource, 0),
		logger:    logger,
	}
}

// Register adds a resource to be managed.
func (rm *ResourceManager) Register(resource Resource) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.resources = append(rm.resources, resource)
}

// RegisterShutdownHook adds a shutdown hook that will be called during graceful shutdown.
func (rm *ResourceManager) RegisterShutdownHook(hook func(ctx context.Context) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.shutdownHooks = append(rm.shutdownHooks, hook)
}

// Close closes all registered resources in reverse order.
func (rm *ResourceManager) Close(ctx context.Context) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Execute shutdown hooks first
	for i := len(rm.shutdownHooks) - 1; i >= 0; i-- {
		if err := rm.shutdownHooks[i](ctx); err != nil {
			rm.logger.Warn("shutdown hook failed", zap.Error(err))
		}
	}

	// Close resources in reverse order
	var lastErr error
	for i := len(rm.resources) - 1; i >= 0; i-- {
		if err := rm.resources[i].Close(); err != nil {
			rm.logger.Warn("failed to close resource", zap.Error(err))
			lastErr = err
		}
	}

	return lastErr
}

// ElasticsearchResource wraps Elasticsearch client for resource management.
type ElasticsearchResource struct {
	client *search.ElasticsearchClient
}

// NewElasticsearchResource creates a new Elasticsearch resource wrapper.
func NewElasticsearchResource(client *search.ElasticsearchClient) *ElasticsearchResource {
	return &ElasticsearchResource{client: client}
}

// Close closes the Elasticsearch client.
func (r *ElasticsearchResource) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Client returns the underlying Elasticsearch client.
func (r *ElasticsearchResource) Client() *search.ElasticsearchClient {
	return r.client
}

// KafkaProducerResource wraps Kafka producer for resource management.
type KafkaProducerResource struct {
	producer *messaging.KafkaProducer
}

// NewKafkaProducerResource creates a new Kafka producer resource wrapper.
func NewKafkaProducerResource(producer *messaging.KafkaProducer) *KafkaProducerResource {
	return &KafkaProducerResource{producer: producer}
}

// Close closes the Kafka producer.
func (r *KafkaProducerResource) Close() error {
	if r.producer != nil {
		return r.producer.Close()
	}
	return nil
}

// Producer returns the underlying Kafka producer.
func (r *KafkaProducerResource) Producer() *messaging.KafkaProducer {
	return r.producer
}

// MinIOResource wraps MinIO client for resource management.
type MinIOResource struct {
	client *storage.MinIOClient
}

// NewMinIOResource creates a new MinIO resource wrapper.
func NewMinIOResource(client *storage.MinIOClient) *MinIOResource {
	return &MinIOResource{client: client}
}

// Close closes the MinIO client (no-op for MinIO, but kept for consistency).
func (r *MinIOResource) Close() error {
	// MinIO client doesn't have explicit Close, but we keep this for consistency
	return nil
}

// Client returns the underlying MinIO client.
func (r *MinIOResource) Client() *storage.MinIOClient {
	return r.client
}

// ObjectStoreAdapter adapts MinIO client to handler.ObjectStore interface.
type ObjectStoreAdapter struct {
	client *storage.MinIOClient
}

// NewObjectStoreAdapter creates a new ObjectStore adapter.
func NewObjectStoreAdapter(client *storage.MinIOClient) *ObjectStoreAdapter {
	return &ObjectStoreAdapter{client: client}
}

// Put uploads an object.
func (a *ObjectStoreAdapter) Put(ctx context.Context, objectName string, r io.Reader, size int64, contentType string) error {
	return a.client.PutObject(ctx, objectName, r, size, contentType)
}

// Get retrieves an object.
func (a *ObjectStoreAdapter) Get(ctx context.Context, objectName string) (io.ReadCloser, error) {
	return a.client.GetObject(ctx, objectName)
}

// Delete deletes an object.
func (a *ObjectStoreAdapter) Delete(ctx context.Context, objectName string) error {
	return a.client.RemoveObject(ctx, objectName)
}

// EventPublisherAdapter adapts Kafka producer to handler.EventPublisher interface.
type EventPublisherAdapter struct {
	producer *messaging.KafkaProducer
}

// NewEventPublisherAdapter creates a new EventPublisher adapter.
func NewEventPublisherAdapter(producer *messaging.KafkaProducer) *EventPublisherAdapter {
	return &EventPublisherAdapter{producer: producer}
}

// Publish publishes an event.
func (a *EventPublisherAdapter) Publish(ctx context.Context, topic string, key string, value any) error {
	return a.producer.SendMessage(ctx, topic, key, value)
}

