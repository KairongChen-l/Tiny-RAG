// Package storage provides storage management with multipart upload support.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"

	"github.com/krc/rag/pkg/storage"
)

// StorageManager manages object storage with multipart upload support.
type StorageManager interface {
	// UploadPart uploads a part of a multipart upload.
	UploadPart(ctx context.Context, uploadID string, partNumber int, data io.Reader, size int64) (string, error)

	// ComposeParts completes a multipart upload by composing all parts.
	ComposeParts(ctx context.Context, uploadID string, parts []PartInfo) error

	// CleanupParts aborts a multipart upload and cleans up all parts.
	CleanupParts(ctx context.Context, uploadID string) error

	// GetObject retrieves an object.
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)

	// InitiateMultipartUpload initiates a new multipart upload.
	InitiateMultipartUpload(ctx context.Context, objectName string, contentType string) (string, error)

	// ListParts lists all parts of a multipart upload.
	ListParts(ctx context.Context, uploadID string) ([]PartInfo, error)
}

// PartInfo represents information about an uploaded part.
type PartInfo struct {
	PartNumber int
	ETag       string
}

// MinIOStorageManager implements StorageManager using MinIO.
type MinIOStorageManager struct {
	client     *storage.MinIOClient
	objectName string
	logger     *zap.Logger
	uploads    map[string]*MultipartUpload // Track active uploads
}

// NewMinIOStorageManager creates a new MinIO storage manager.
func NewMinIOStorageManager(client *storage.MinIOClient, objectName string, logger *zap.Logger) *MinIOStorageManager {
	return &MinIOStorageManager{
		client:     client,
		objectName: objectName,
		logger:     logger,
		uploads:    make(map[string]*MultipartUpload),
	}
}

// InitiateMultipartUpload initiates a new multipart upload.
func (m *MinIOStorageManager) InitiateMultipartUpload(ctx context.Context, objectName string, contentType string) (string, error) {
	uploadID, err := m.client.InitiateMultipartUpload(ctx, objectName, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	// Track the upload
	m.uploads[uploadID] = NewMultipartUpload(m.client, objectName, uploadID, m.logger)

	m.logger.Info("initiated multipart upload",
		zap.String("object_name", objectName),
		zap.String("upload_id", uploadID),
	)

	return uploadID, nil
}

// UploadPart uploads a part of a multipart upload.
func (m *MinIOStorageManager) UploadPart(ctx context.Context, uploadID string, partNumber int, data io.Reader, size int64) (string, error) {
	// Get the upload session
	upload, ok := m.uploads[uploadID]
	if !ok {
		return "", fmt.Errorf("upload ID not found: %s", uploadID)
	}

	// Upload the part
	partInfo, err := m.client.UploadPart(ctx, upload.objectName, uploadID, partNumber, data, size)
	if err != nil {
		return "", fmt.Errorf("failed to upload part: %w", err)
	}

	// Add to upload session
	upload.AddPart(partInfo)

	m.logger.Debug("uploaded part",
		zap.String("upload_id", uploadID),
		zap.Int("part_number", partNumber),
		zap.String("etag", partInfo.ETag),
	)

	return partInfo.ETag, nil
}

// ComposeParts completes a multipart upload by composing all parts.
func (m *MinIOStorageManager) ComposeParts(ctx context.Context, uploadID string, parts []PartInfo) error {
	// Get the upload session
	upload, ok := m.uploads[uploadID]
	if !ok {
		return fmt.Errorf("upload ID not found: %s", uploadID)
	}

	// Convert PartInfo to storage.PartInfo
	storageParts := make([]storage.PartInfo, len(parts))
	for i, part := range parts {
		storageParts[i] = storage.PartInfo{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}

	// Complete the upload
	if err := m.client.CompleteMultipartUpload(ctx, upload.objectName, uploadID, storageParts); err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	// Remove from tracking
	delete(m.uploads, uploadID)

	m.logger.Info("completed multipart upload",
		zap.String("upload_id", uploadID),
		zap.String("object_name", upload.objectName),
		zap.Int("parts_count", len(parts)),
	)

	return nil
}

// CleanupParts aborts a multipart upload and cleans up all parts.
func (m *MinIOStorageManager) CleanupParts(ctx context.Context, uploadID string) error {
	// Get the upload session
	upload, ok := m.uploads[uploadID]
	if !ok {
		// Try to abort anyway (might be an orphaned upload)
		return m.client.AbortMultipartUpload(ctx, m.objectName, uploadID)
	}

	// Abort the upload
	if err := upload.Abort(ctx); err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}

	// Remove from tracking
	delete(m.uploads, uploadID)

	m.logger.Info("aborted multipart upload",
		zap.String("upload_id", uploadID),
		zap.String("object_name", upload.objectName),
	)

	return nil
}

// GetObject retrieves an object.
func (m *MinIOStorageManager) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return m.client.GetObject(ctx, key)
}

// ListParts lists all parts of a multipart upload.
func (m *MinIOStorageManager) ListParts(ctx context.Context, uploadID string) ([]PartInfo, error) {
	// Get the upload session
	upload, ok := m.uploads[uploadID]
	if !ok {
		return nil, fmt.Errorf("upload ID not found: %s", uploadID)
	}

	// List parts from MinIO
	storageParts, err := m.client.ListParts(ctx, upload.objectName, uploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to list parts: %w", err)
	}

	// Convert to PartInfo
	parts := make([]PartInfo, len(storageParts))
	for i, part := range storageParts {
		parts[i] = PartInfo{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}

	return parts, nil
}

// StartCleanupTask starts a background task to clean up expired multipart uploads.
func (m *MinIOStorageManager) StartCleanupTask(ctx context.Context, interval time.Duration, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Clean up expired uploads
				now := time.Now()
				for uploadID, upload := range m.uploads {
					// TODO: Track upload creation time and clean up if expired
					_ = upload
					_ = uploadID
					_ = now
					// For now, we'll rely on MinIO's lifecycle policies or manual cleanup
				}
			}
		}
	}()
}
