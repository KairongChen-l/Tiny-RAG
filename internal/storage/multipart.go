// Package storage provides multipart upload utilities.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"

	"github.com/krc/rag/pkg/storage"
)

const (
	// DefaultPartSize is the default size for each part in multipart upload (5MB).
	DefaultPartSize = 5 * 1024 * 1024
	// MaxPartSize is the maximum size for each part (5GB).
	MaxPartSize = 5 * 1024 * 1024 * 1024
	// MinPartSize is the minimum size for each part (5MB).
	MinPartSize = 5 * 1024 * 1024
)

// MultipartUpload manages a multipart upload session.
type MultipartUpload struct {
	client     *storage.MinIOClient
	objectName string
	uploadID   string
	parts      []storage.PartInfo
	logger     *zap.Logger
}

// NewMultipartUpload creates a new multipart upload session.
func NewMultipartUpload(client *storage.MinIOClient, objectName string, uploadID string, logger *zap.Logger) *MultipartUpload {
	return &MultipartUpload{
		client:     client,
		objectName: objectName,
		uploadID:   uploadID,
		parts:      make([]storage.PartInfo, 0),
		logger:     logger,
	}
}

// AddPart adds a part to the multipart upload.
func (m *MultipartUpload) AddPart(part storage.PartInfo) {
	m.parts = append(m.parts, part)
}

// GetParts returns all uploaded parts.
func (m *MultipartUpload) GetParts() []storage.PartInfo {
	return m.parts
}

// Complete completes the multipart upload.
func (m *MultipartUpload) Complete(ctx context.Context) error {
	if len(m.parts) == 0 {
		return fmt.Errorf("no parts to complete")
	}

	// Sort parts by part number
	sortedParts := make([]storage.PartInfo, len(m.parts))
	copy(sortedParts, m.parts)
	for i := 0; i < len(sortedParts)-1; i++ {
		for j := i + 1; j < len(sortedParts); j++ {
			if sortedParts[i].PartNumber > sortedParts[j].PartNumber {
				sortedParts[i], sortedParts[j] = sortedParts[j], sortedParts[i]
			}
		}
	}

	return m.client.CompleteMultipartUpload(ctx, m.objectName, m.uploadID, sortedParts)
}

// Abort aborts the multipart upload.
func (m *MultipartUpload) Abort(ctx context.Context) error {
	return m.client.AbortMultipartUpload(ctx, m.objectName, m.uploadID)
}

// SplitIntoParts splits a reader into parts of the specified size.
func SplitIntoParts(reader io.Reader, partSize int64) ([]io.Reader, error) {
	if partSize < MinPartSize {
		partSize = DefaultPartSize
	}
	if partSize > MaxPartSize {
		partSize = MaxPartSize
	}

	var parts []io.Reader
	buffer := make([]byte, partSize)

	for {
		n, err := io.ReadFull(reader, buffer)
		if err == io.EOF {
			break
		}
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("failed to read data: %w", err)
		}

		// Create a reader for this part
		partData := make([]byte, n)
		copy(partData, buffer[:n])
		parts = append(parts, &partReader{data: partData})

		if err == io.ErrUnexpectedEOF {
			break // Last part
		}
	}

	return parts, nil
}

// partReader is a simple reader that reads from a byte slice.
type partReader struct {
	data []byte
	pos  int
}

func (r *partReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// CleanupExpiredUploads cleans up expired multipart uploads.
// This should be called periodically to clean up abandoned uploads.
func CleanupExpiredUploads(ctx context.Context, client *storage.MinIOClient, bucketName string, maxAge time.Duration, logger *zap.Logger) error {
	// List all multipart uploads
	// Note: MinIO Go SDK doesn't have a direct API to list all multipart uploads
	// This is a placeholder - in production, you might need to track uploads in Redis/DB
	logger.Info("cleaning up expired multipart uploads", zap.Duration("max_age", maxAge))
	// TODO: Implement actual cleanup logic
	return nil
}

