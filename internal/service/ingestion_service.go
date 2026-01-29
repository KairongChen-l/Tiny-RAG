// Package service provides business logic services.
package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/job"
	jobpayloads "github.com/krc/rag/internal/job/payloads"
)

// IngestionService handles document ingestion business logic.
type IngestionService struct {
	jobQueue job.Worker
	logger   *zap.Logger
}

// NewIngestionService creates a new ingestion service.
func NewIngestionService(jobQueue job.Worker, logger *zap.Logger) *IngestionService {
	return &IngestionService{
		jobQueue: jobQueue,
		logger:   logger,
	}
}

// StartIngestionRequest represents a request to start document ingestion.
type StartIngestionRequest struct {
	LocalPath string
	ObjectKey string
	Filename  string
	Metadata  map[string]string
}

// StartIngestionResponse represents the response for starting ingestion.
type StartIngestionResponse struct {
	JobID   string
	Status  string
	Message string
}

// StartIngestion starts a document ingestion job.
func (s *IngestionService) StartIngestion(ctx context.Context, req StartIngestionRequest) (*StartIngestionResponse, error) {
	// Create job payload
	payload := jobpayloads.DocumentIngestPayload{
		LocalPath: req.LocalPath,
		ObjectKey: req.ObjectKey,
		Filename:  req.Filename,
		Metadata:  req.Metadata,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Create job
	jobID := uuid.New().String()
	j := job.NewJob(jobID, job.TypeDocumentIngest, payloadBytes)

	// Submit job
	if err := s.jobQueue.Submit(ctx, j); err != nil {
		s.logger.Error("failed to submit ingestion job", zap.Error(err))
		return nil, err
	}

	return &StartIngestionResponse{
		JobID:   jobID,
		Status:  string(job.StatusPending),
		Message: "Document processing started",
	}, nil
}

