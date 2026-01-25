package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

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
	if err := h.jobQueue.Submit(r.Context(), j); err != nil {
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
}

// DocumentInfo represents document information.
type DocumentInfo struct {
	ID     string `json:"id"`
	Source string `json:"source"`
}

// ListDocuments handles list documents requests.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement document listing from vector store
	WriteJSON(w, http.StatusOK, ListDocumentsResponse{
		Documents: []DocumentInfo{},
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

