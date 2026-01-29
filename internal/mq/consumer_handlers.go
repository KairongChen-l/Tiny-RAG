// Package mq provides message queue consumer handlers.
package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/processor"
	jobpayloads "github.com/krc/rag/internal/job/payloads"
)

// DocumentIngestHandler handles document ingestion events from Kafka.
// This handler processes messages from the documents.ingested topic.
func DocumentIngestHandler(
	processor processor.TaskProcessor,
	logger *zap.Logger,
) ConsumerHandler {
	return func(ctx context.Context, key string, value []byte) error {
		logger.Info("received document ingestion event",
			zap.String("key", key),
			zap.Int("value_size", len(value)),
		)

		// Parse payload
		var payload jobpayloads.DocumentIngestPayload
		if err := json.Unmarshal(value, &payload); err != nil {
			logger.Error("failed to parse document ingestion payload",
				zap.Error(err),
				zap.ByteString("value", value),
			)
			return fmt.Errorf("failed to parse payload: %w", err)
		}

		// Process using TaskProcessor
		// Note: We don't have a progress callback here since this is async processing
		if err := processor.Process(ctx, payload, nil); err != nil {
			logger.Error("failed to process document ingestion",
				zap.Error(err),
				zap.String("filename", payload.Filename),
			)
			return fmt.Errorf("failed to process document: %w", err)
		}

		logger.Info("successfully processed document ingestion",
			zap.String("filename", payload.Filename),
		)

		return nil
	}
}

// DocumentUploadedHandler handles document uploaded events from Kafka.
// This handler can be used for post-upload processing or notifications.
func DocumentUploadedHandler(logger *zap.Logger) ConsumerHandler {
	return func(ctx context.Context, key string, value []byte) error {
		logger.Info("received document uploaded event",
			zap.String("key", key),
			zap.Int("value_size", len(value)),
		)

		// Parse event data
		var eventData map[string]interface{}
		if err := json.Unmarshal(value, &eventData); err != nil {
			logger.Warn("failed to parse document uploaded event",
				zap.Error(err),
			)
			// Don't fail - this is a notification event
			return nil
		}

		logger.Info("document uploaded",
			zap.Any("event_data", eventData),
		)

		// Additional processing can be added here (e.g., notifications, analytics)

		return nil
	}
}

// DocumentFailedHandler handles document processing failure events from Kafka.
func DocumentFailedHandler(logger *zap.Logger) ConsumerHandler {
	return func(ctx context.Context, key string, value []byte) error {
		logger.Info("received document failed event",
			zap.String("key", key),
			zap.Int("value_size", len(value)),
		)

		// Parse event data
		var eventData map[string]interface{}
		if err := json.Unmarshal(value, &eventData); err != nil {
			logger.Warn("failed to parse document failed event",
				zap.Error(err),
			)
			// Don't fail - this is a notification event
			return nil
		}

		logger.Warn("document processing failed",
			zap.Any("event_data", eventData),
		)

		// Additional processing can be added here (e.g., alerting, retry logic)

		return nil
	}
}

