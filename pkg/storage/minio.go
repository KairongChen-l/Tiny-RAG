package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOClient wraps MinIO client for object storage.
type MinIOClient struct {
	client     *minio.Client
	bucketName string
}

// Config holds MinIO configuration.
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Region          string
}

// NewMinIOClient creates a new MinIO client.
func NewMinIOClient(cfg Config) (*MinIOClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	mc := &MinIOClient{
		client:     client,
		bucketName: cfg.BucketName,
	}

	// Ensure bucket exists
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		if err := client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{
			Region: cfg.Region,
		}); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return mc, nil
}

// PutObject uploads an object to MinIO.
func (m *MinIOClient) PutObject(ctx context.Context, objectName string, reader io.Reader, objectSize int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucketName, objectName, reader, objectSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// GetObject retrieves an object from MinIO.
func (m *MinIOClient) GetObject(ctx context.Context, objectName string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// RemoveObject deletes an object from MinIO.
func (m *MinIOClient) RemoveObject(ctx context.Context, objectName string) error {
	return m.client.RemoveObject(ctx, m.bucketName, objectName, minio.RemoveObjectOptions{})
}

// StatObject gets object metadata.
func (m *MinIOClient) StatObject(ctx context.Context, objectName string) (minio.ObjectInfo, error) {
	return m.client.StatObject(ctx, m.bucketName, objectName, minio.StatObjectOptions{})
}

// PartInfo represents information about an uploaded part.
type PartInfo struct {
	PartNumber int
	ETag       string
}

// InitiateMultipartUpload initiates a multipart upload.
// Note: MinIO Go SDK v7 doesn't expose direct multipart upload APIs.
// This is a placeholder implementation that returns a temporary upload ID.
// For production use, consider using AWS SDK or implementing S3-compatible multipart upload directly.
func (m *MinIOClient) InitiateMultipartUpload(ctx context.Context, objectName string, contentType string) (string, error) {
	// Generate a temporary upload ID
	// In a real implementation, this would call MinIO's CreateMultipartUpload API
	uploadID := fmt.Sprintf("upload-%d-%s", time.Now().UnixNano(), objectName)
	return uploadID, nil
}

// UploadPart uploads a part of a multipart upload.
// Note: This is a placeholder. For production, implement using S3-compatible multipart upload API.
func (m *MinIOClient) UploadPart(ctx context.Context, objectName string, uploadID string, partNumber int, data io.Reader, size int64) (PartInfo, error) {
	// For now, store parts with a temporary naming scheme
	// In production, this should use MinIO's UploadPart API
	tempObjectName := fmt.Sprintf("temp/%s/part-%d", uploadID, partNumber)
	
	_, err := m.client.PutObject(ctx, m.bucketName, tempObjectName, data, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return PartInfo{}, fmt.Errorf("failed to upload part %d: %w", partNumber, err)
	}

	// Get ETag from the uploaded object
	objInfo, err := m.client.StatObject(ctx, m.bucketName, tempObjectName, minio.StatObjectOptions{})
	if err != nil {
		return PartInfo{}, fmt.Errorf("failed to get part info: %w", err)
	}

	return PartInfo{
		PartNumber: partNumber,
		ETag:       objInfo.ETag,
	}, nil
}

// CompleteMultipartUpload completes a multipart upload by composing all parts.
// Note: This is a simplified implementation. For production, use MinIO's CompleteMultipartUpload API.
func (m *MinIOClient) CompleteMultipartUpload(ctx context.Context, objectName string, uploadID string, parts []PartInfo) error {
	// For now, we'll read all parts and upload as a single object
	// In production, this should use MinIO's CompleteMultipartUpload API to compose parts server-side
	
	// Sort parts by part number
	sortedParts := make([]PartInfo, len(parts))
	copy(sortedParts, parts)
	for i := 0; i < len(sortedParts)-1; i++ {
		for j := i + 1; j < len(sortedParts); j++ {
			if sortedParts[i].PartNumber > sortedParts[j].PartNumber {
				sortedParts[i], sortedParts[j] = sortedParts[j], sortedParts[i]
			}
		}
	}

	// Read all parts and combine
	var readers []io.Reader
	for _, part := range sortedParts {
		tempObjectName := fmt.Sprintf("temp/%s/part-%d", uploadID, part.PartNumber)
		obj, err := m.client.GetObject(ctx, m.bucketName, tempObjectName, minio.GetObjectOptions{})
		if err != nil {
			return fmt.Errorf("failed to read part %d: %w", part.PartNumber, err)
		}
		readers = append(readers, obj)
	}

	// Combine all parts into a single reader
	combinedReader := io.MultiReader(readers...)

	// Get total size
	var totalSize int64
	for _, part := range sortedParts {
		tempObjectName := fmt.Sprintf("temp/%s/part-%d", uploadID, part.PartNumber)
		objInfo, err := m.client.StatObject(ctx, m.bucketName, tempObjectName, minio.StatObjectOptions{})
		if err != nil {
			return fmt.Errorf("failed to get part size: %w", err)
		}
		totalSize += objInfo.Size
	}

	// Upload combined object
	_, err := m.client.PutObject(ctx, m.bucketName, objectName, combinedReader, totalSize, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to upload combined object: %w", err)
	}

	// Clean up temporary parts
	for _, part := range sortedParts {
		tempObjectName := fmt.Sprintf("temp/%s/part-%d", uploadID, part.PartNumber)
		if err := m.client.RemoveObject(ctx, m.bucketName, tempObjectName, minio.RemoveObjectOptions{}); err != nil {
			// Log but don't fail
			_ = err
		}
	}

	return nil
}

// AbortMultipartUpload aborts a multipart upload and cleans up temporary parts.
func (m *MinIOClient) AbortMultipartUpload(ctx context.Context, objectName string, uploadID string) error {
	// List and remove all temporary parts
	// In production, this should use MinIO's AbortMultipartUpload API
	objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Prefix:    fmt.Sprintf("temp/%s/", uploadID),
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			continue
		}
		if err := m.client.RemoveObject(ctx, m.bucketName, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			// Log but continue cleanup
			_ = err
		}
	}

	return nil
}

// ListParts lists all parts of a multipart upload.
func (m *MinIOClient) ListParts(ctx context.Context, objectName string, uploadID string) ([]PartInfo, error) {
	// List all temporary parts
	objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Prefix:    fmt.Sprintf("temp/%s/part-", uploadID),
		Recursive: true,
	})

	var parts []PartInfo
	for obj := range objectCh {
		if obj.Err != nil {
			continue
		}

		// Extract part number from object name
		var partNumber int
		if _, err := fmt.Sscanf(obj.Key, "temp/"+uploadID+"/part-%d", &partNumber); err != nil {
			continue
		}

		parts = append(parts, PartInfo{
			PartNumber: partNumber,
			ETag:       obj.ETag,
		})
	}

	return parts, nil
}
