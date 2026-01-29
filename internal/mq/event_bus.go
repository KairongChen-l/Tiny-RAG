// Package mq provides message queue abstractions for event publishing and consumption.
package mq

import (
	"context"

	"github.com/krc/rag/internal/common"
)

// EventBus provides a business-friendly interface for publishing lifecycle events.
type EventBus interface {
	// DocumentUploaded publishes a document uploaded event.
	DocumentUploaded(ctx context.Context, documentID string, metadata map[string]any) error

	// DocumentIngested publishes a document ingested event.
	DocumentIngested(ctx context.Context, documentID string, metadata map[string]any) error

	// DocumentFailed publishes a document processing failed event.
	DocumentFailed(ctx context.Context, documentID string, errorMsg string) error
}

// KafkaEventBus is a Kafka-based implementation of EventBus.
type KafkaEventBus struct {
	publisher common.EventPublisher
	cfg       EventBusConfig
}

// EventBusConfig holds event bus configuration.
type EventBusConfig struct {
	TopicDocumentsUploaded  string
	TopicDocumentsIngested   string
	TopicDocumentsFailed     string
}

// NewKafkaEventBus creates a new Kafka event bus.
func NewKafkaEventBus(publisher common.EventPublisher, cfg EventBusConfig) *KafkaEventBus {
	return &KafkaEventBus{
		publisher: publisher,
		cfg:       cfg,
	}
}

// DocumentUploaded publishes a document uploaded event.
func (b *KafkaEventBus) DocumentUploaded(ctx context.Context, documentID string, metadata map[string]any) error {
	if b.publisher == nil || b.cfg.TopicDocumentsUploaded == "" {
		return nil
	}
	return b.publisher.Publish(ctx, b.cfg.TopicDocumentsUploaded, documentID, metadata)
}

// DocumentIngested publishes a document ingested event.
func (b *KafkaEventBus) DocumentIngested(ctx context.Context, documentID string, metadata map[string]any) error {
	if b.publisher == nil || b.cfg.TopicDocumentsIngested == "" {
		return nil
	}
	return b.publisher.Publish(ctx, b.cfg.TopicDocumentsIngested, documentID, metadata)
}

// DocumentFailed publishes a document processing failed event.
func (b *KafkaEventBus) DocumentFailed(ctx context.Context, documentID string, errorMsg string) error {
	if b.publisher == nil || b.cfg.TopicDocumentsFailed == "" {
		return nil
	}
	metadata := map[string]any{
		"document_id": documentID,
		"error":       errorMsg,
	}
	return b.publisher.Publish(ctx, b.cfg.TopicDocumentsFailed, documentID, metadata)
}


