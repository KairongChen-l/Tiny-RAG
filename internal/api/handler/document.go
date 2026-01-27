package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/ingestion"
	"github.com/krc/rag/internal/job"
)

// UploadDocumentResponse represents document upload response.
type UploadDocumentResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// DocumentIngestPayload is the job payload for document ingestion.
type DocumentIngestPayload struct {
	FilePath string            `json:"file_path"`
	Metadata map[string]string `json:"metadata"`
}

// UploadDocument handles document upload requests.
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "failed to parse form")
		return
	}

	// Get file
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Detect format
	format, err := ingestion.DetectFormat(header.Filename)
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeUnsupportedFormat, err.Error())
		return
	}

	// Check if parser is available
	if _, err := h.parserRegistry.GetParser(format); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeUnsupportedFormat, "no parser available for format")
		return
	}

	// Parse metadata
	var metadata map[string]string
	if metaStr := r.FormValue("metadata"); metaStr != "" {
		if err := json.Unmarshal([]byte(metaStr), &metadata); err != nil {
			WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid metadata JSON")
			return
		}
	} else {
		metadata = make(map[string]string)
	}

	// Extract title from form if provided
	if title := r.FormValue("title"); title != "" {
		metadata["title"] = title
	}

	// Save file temporarily
	tempDir := filepath.Join(os.TempDir(), "rag-uploads")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		h.logger.Error("failed to create temp dir", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to save file")
		return
	}

	tempPath := filepath.Join(tempDir, uuid.New().String()+filepath.Ext(header.Filename))
	tempFile, err := os.Create(tempPath)
	if err != nil {
		h.logger.Error("failed to create temp file", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to save file")
		return
	}

	if _, err := io.Copy(tempFile, file); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		h.logger.Error("failed to copy file", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to save file")
		return
	}
	tempFile.Close()

	// Create job
	jobID := uuid.New().String()
	payload, _ := json.Marshal(DocumentIngestPayload{
		FilePath: tempPath,
		Metadata: metadata,
	})

	j := job.NewJob(jobID, job.TypeDocumentIngest, payload)

	// Submit job
	h.metrics.JobSubmitted.Inc()
	if err := h.jobQueue.Submit(r.Context(), j); err != nil {
		h.metrics.JobFailed.Inc()
		os.Remove(tempPath)
		h.logger.Error("failed to submit job", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to queue document")
		return
	}

	WriteJSON(w, http.StatusAccepted, UploadDocumentResponse{
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
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := index.ListOptions{
		Limit:  50,
		Offset: 0,
		SortBy: "created_at",
		Order:  "desc",
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			opts.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	if sortBy := r.URL.Query().Get("sort_by"); sortBy != "" {
		// Validate sort_by values
		switch sortBy {
		case "created_at", "updated_at", "title":
			opts.SortBy = sortBy
		}
	}

	if order := r.URL.Query().Get("order"); order != "" {
		// Validate order values
		switch order {
		case "asc", "desc":
			opts.Order = order
		}
	}

	result, err := h.vectorStore.ListDocuments(r.Context(), opts)
	if err != nil {
		h.logger.Error("failed to list documents", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list documents")
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

	WriteJSON(w, http.StatusOK, ListDocumentsResponse{
		Documents: docInfos,
		Total:     result.Total,
		Limit:     result.Limit,
		Offset:    result.Offset,
	})
}

// DeleteDocument handles document deletion requests.
func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	if docID == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "document ID is required")
		return
	}

	if err := h.vectorStore.DeleteByDocument(r.Context(), docID); err != nil {
		h.logger.Error("failed to delete document", zap.Error(err), zap.String("document_id", docID))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to delete document")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "document deleted",
	})
}

// BatchDeleteDocumentsRequest represents a batch delete request.
type BatchDeleteDocumentsRequest struct {
	DocumentIDs []string `json:"document_ids"`
}

// BatchDeleteDocumentsResponse represents a batch delete response.
type BatchDeleteDocumentsResponse struct {
	Deleted     []string          `json:"deleted"`
	Failed      []string          `json:"failed"`
	Errors      map[string]string `json:"errors,omitempty"`
	Total       int               `json:"total"`
	Success     int               `json:"success"`
	FailedCount int               `json:"failed_count"`
}

// GetDocumentStats handles document statistics requests.
func (h *Handler) GetDocumentStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.vectorStore.GetStats(r.Context())
	if err != nil {
		h.logger.Error("failed to get document stats", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to get document stats")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"total_documents": stats.TotalDocuments,
		"total_chunks":    stats.TotalChunks,
		"total_size":      stats.TotalSize,
	})
}

// BatchDeleteDocuments handles batch document deletion requests.
func (h *Handler) BatchDeleteDocuments(w http.ResponseWriter, r *http.Request) {
	var req BatchDeleteDocumentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	if len(req.DocumentIDs) == 0 {
		WriteError(w, http.StatusBadRequest, ErrCodeValidation, "document_ids is required and cannot be empty")
		return
	}

	if len(req.DocumentIDs) > 100 {
		WriteError(w, http.StatusBadRequest, ErrCodeValidation, "cannot delete more than 100 documents at once")
		return
	}

	// Track results
	deleted := make([]string, 0)
	failed := make([]string, 0)
	errors := make(map[string]string)

	// Delete each document
	for _, docID := range req.DocumentIDs {
		if docID == "" {
			continue
		}

		if err := h.vectorStore.DeleteByDocument(r.Context(), docID); err != nil {
			h.logger.Error("failed to delete document in batch", zap.Error(err), zap.String("document_id", docID))
			failed = append(failed, docID)
			errors[docID] = err.Error()
		} else {
			deleted = append(deleted, docID)
		}
	}

	// If all failed, return error status
	if len(deleted) == 0 && len(failed) > 0 {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "all documents failed to delete")
		return
	}

	// Return partial success if some failed
	status := http.StatusOK
	if len(failed) > 0 {
		status = http.StatusMultiStatus // 207
	}

	WriteJSON(w, status, BatchDeleteDocumentsResponse{
		Deleted:     deleted,
		Failed:      failed,
		Errors:      errors,
		Total:       len(req.DocumentIDs),
		Success:     len(deleted),
		FailedCount: len(failed),
	})
}
