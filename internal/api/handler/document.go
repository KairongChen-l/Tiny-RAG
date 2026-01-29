package handler

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/ingestion"
	"github.com/krc/rag/internal/job"
	jobpayloads "github.com/krc/rag/internal/job/payloads"
)

// UploadDocumentResponse represents document upload response.
type UploadDocumentResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// UploadDocument handles document upload requests.
func (h *Handler) UploadDocument(c *gin.Context) {
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		WriteError(c, 400, ErrCodeBadRequest, "failed to parse form")
		return
	}

	// Get file
	fileHeader := form.File["file"]
	if len(fileHeader) == 0 {
		WriteError(c, 400, ErrCodeBadRequest, "file is required")
		return
	}

	header := fileHeader[0]
	file, err := header.Open()
	if err != nil {
		WriteError(c, 400, ErrCodeBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Detect format
	format, err := ingestion.DetectFormat(header.Filename)
	if err != nil {
		WriteError(c, 400, ErrCodeUnsupportedFormat, err.Error())
		return
	}

	// Check if parser is available
	if _, err := h.parserRegistry.GetParser(format); err != nil {
		WriteError(c, 400, ErrCodeUnsupportedFormat, "no parser available for format")
		return
	}

	// Parse metadata
	var metadata map[string]string
	if metaStr := c.PostForm("metadata"); metaStr != "" {
		if err := json.Unmarshal([]byte(metaStr), &metadata); err != nil {
			WriteError(c, 400, ErrCodeBadRequest, "invalid metadata JSON")
			return
		}
	} else {
		metadata = make(map[string]string)
	}

	// Extract title from form if provided
	if title := c.PostForm("title"); title != "" {
		metadata["title"] = title
	}

	// Decide storage: object store (MinIO) if configured, otherwise local temp file.
	var localPath string
	var objectKey string
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		// Fallback for multipart forms where content-type might be missing
		contentType = "application/octet-stream"
	}

	if h.objectStore != nil {
		objectKey = uuid.New().String() + filepath.Ext(header.Filename)
		if err := h.objectStore.Put(c.Request.Context(), objectKey, file, header.Size, contentType); err != nil {
			h.logger.Error("failed to upload to object store", zap.Error(err))
			WriteError(c, 500, ErrCodeInternalError, "failed to save file")
			return
		}
		// Reset reader not possible; but we already uploaded and won't need local copy here.
	} else {
		// Save file temporarily
		tempDir := filepath.Join(os.TempDir(), "rag-uploads")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			h.logger.Error("failed to create temp dir", zap.Error(err))
			WriteError(c, 500, ErrCodeInternalError, "failed to save file")
			return
		}

		localPath = filepath.Join(tempDir, uuid.New().String()+filepath.Ext(header.Filename))
		tempFile, err := os.Create(localPath)
		if err != nil {
			h.logger.Error("failed to create temp file", zap.Error(err))
			WriteError(c, 500, ErrCodeInternalError, "failed to save file")
			return
		}

		if _, err := io.Copy(tempFile, file); err != nil {
			tempFile.Close()
			os.Remove(localPath)
			h.logger.Error("failed to copy file", zap.Error(err))
			WriteError(c, 500, ErrCodeInternalError, "failed to save file")
			return
		}
		tempFile.Close()
	}

	// Emit upload event (best-effort)
	if h.events != nil {
		_ = h.events.Publish(c.Request.Context(), "rag.documents.uploaded", "", map[string]any{
			"filename":  header.Filename,
			"objectKey": objectKey,
			"localPath": localPath,
			"metadata":  metadata,
		})
	}

	// Create job
	jobID := uuid.New().String()
	payloadBytes, _ := json.Marshal(jobpayloads.DocumentIngestPayload{
		LocalPath: localPath,
		ObjectKey: objectKey,
		Filename:  header.Filename,
		Metadata:  metadata,
	})

	j := job.NewJob(jobID, job.TypeDocumentIngest, payloadBytes)

	// Submit job
	h.metrics.JobSubmitted.Inc()
	if err := h.jobQueue.Submit(c.Request.Context(), j); err != nil {
		h.metrics.JobFailed.Inc()
		// Cleanup temp file / object if job submission failed.
		if localPath != "" {
			os.Remove(localPath)
		}
		if objectKey != "" && h.objectStore != nil {
			_ = h.objectStore.Delete(c.Request.Context(), objectKey)
		}
		h.logger.Error("failed to submit job", zap.Error(err))
		WriteError(c, 500, ErrCodeInternalError, "failed to queue document")
		return
	}

	WriteJSON(c, 202, UploadDocumentResponse{
		JobID:   jobID,
		Status:  string(job.StatusPending),
		Message: "Document processing started",
	})
}

// ListDocumentsResponse represents list documents response.
type ListDocumentsResponse struct {
	Documents []DocumentInfo `json:"documents"`
	Total     int            `json:"total"`
	Limit     int            `json:"limit"`
	Offset    int            `json:"offset"`
}

// DocumentInfo represents document information.
type DocumentInfo struct {
	ID        string            `json:"id"`
	Source    string            `json:"source"`
	Title     string            `json:"title"`
	Format    string            `json:"format"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ListDocuments handles list documents requests.
func (h *Handler) ListDocuments(c *gin.Context) {
	// Parse query parameters
	opts := index.ListOptions{
		Limit:  50,
		Offset: 0,
		SortBy: "created_at",
		Order:  "desc",
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			opts.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	if sortBy := c.Query("sort_by"); sortBy != "" {
		// Validate sort_by values
		switch sortBy {
		case "created_at", "updated_at", "title":
			opts.SortBy = sortBy
		}
	}

	if order := c.Query("order"); order != "" {
		// Validate order values
		switch order {
		case "asc", "desc":
			opts.Order = order
		}
	}

	// Support include_deleted parameter
	if includeDeleted := c.Query("include_deleted"); includeDeleted == "true" {
		opts.IncludeDeleted = true
	}

	result, err := h.vectorStore.ListDocuments(c.Request.Context(), opts)
	if err != nil {
		h.logger.Error("failed to list documents", zap.Error(err))
		WriteError(c, 500, ErrCodeInternalError, "failed to list documents")
		return
	}

	docInfos := make([]DocumentInfo, len(result.Documents))
	for i, doc := range result.Documents {
		docInfos[i] = DocumentInfo{
			ID:        doc.ID,
			Source:    doc.Source,
			Title:     doc.Title,
			Format:    doc.Format,
			CreatedAt: doc.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: doc.UpdatedAt.Format("2006-01-02 15:04:05"),
			Metadata:  doc.Metadata,
		}
	}

	WriteJSON(c, 200, ListDocumentsResponse{
		Documents: docInfos,
		Total:     result.Total,
		Limit:     result.Limit,
		Offset:    result.Offset,
	})
}

// DeleteDocument handles document deletion requests (soft delete by default).
func (h *Handler) DeleteDocument(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		WriteError(c, 400, ErrCodeBadRequest, "document ID is required")
		return
	}

	// Check if hard delete is requested
	hardDelete := c.Query("hard") == "true"

	ctx := c.Request.Context()
	if hardDelete {
		if err := h.vectorStore.HardDeleteDocument(ctx, docID); err != nil {
			h.logger.Error("failed to hard delete document", zap.Error(err), zap.String("document_id", docID))
			WriteError(c, 500, ErrCodeInternalError, "failed to delete document")
			return
		}
		// Best-effort: delete from Elasticsearch if enabled
		h.deleteFromElasticsearch(ctx, docID)
		WriteJSON(c, 200, map[string]string{
			"message": "document permanently deleted",
		})
	} else {
		if err := h.vectorStore.SoftDeleteDocument(ctx, docID); err != nil {
			h.logger.Error("failed to soft delete document", zap.Error(err), zap.String("document_id", docID))
			WriteError(c, 500, ErrCodeInternalError, "failed to delete document")
			return
		}
		WriteJSON(c, 200, map[string]string{
			"message": "document deleted (soft delete)",
		})
	}
}

// RestoreDocument handles document restore requests.
func (h *Handler) RestoreDocument(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		WriteError(c, 400, ErrCodeBadRequest, "document ID is required")
		return
	}

	if err := h.vectorStore.RestoreDocument(c.Request.Context(), docID); err != nil {
		h.logger.Error("failed to restore document", zap.Error(err), zap.String("document_id", docID))
		WriteError(c, 500, ErrCodeInternalError, "failed to restore document")
		return
	}

	WriteJSON(c, 200, map[string]string{
		"message": "document restored",
	})
}

// deleteFromElasticsearch deletes a document from Elasticsearch (best-effort).
func (h *Handler) deleteFromElasticsearch(ctx context.Context, documentID string) {
	if h.esClient == nil {
		return
	}

	if err := h.esClient.DeleteByDocumentID(ctx, documentID); err != nil {
		h.logger.Warn("failed to delete document from Elasticsearch", zap.String("document_id", documentID), zap.Error(err))
	} else {
		h.logger.Debug("deleted document from Elasticsearch", zap.String("document_id", documentID))
	}
}

// GetDocumentStats handles document statistics requests.
func (h *Handler) GetDocumentStats(c *gin.Context) {
	stats, err := h.vectorStore.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get document stats", zap.Error(err))
		WriteError(c, 500, ErrCodeInternalError, "failed to get document stats")
		return
	}

	WriteJSON(c, 200, map[string]interface{}{
		"total_documents": stats.TotalDocuments,
		"total_chunks":    stats.TotalChunks,
		"total_size":      stats.TotalSize,
	})
}
