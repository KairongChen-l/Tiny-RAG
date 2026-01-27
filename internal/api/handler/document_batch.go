package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/ingestion"
	"github.com/krc/rag/internal/job"
)

// BatchUploadDocumentsRequest represents a batch upload request.
type BatchUploadDocumentsRequest struct {
	Files []BatchUploadFile `json:"files,omitempty"` // For JSON API (optional)
}

// BatchUploadFile represents a file in batch upload.
type BatchUploadFile struct {
	Filename string            `json:"filename"`
	Content  []byte            `json:"content"` // Base64 encoded
	Metadata map[string]string `json:"metadata,omitempty"`
}

// BatchUploadDocumentsResponse represents a batch upload response.
type BatchUploadDocumentsResponse struct {
	Jobs      []BatchUploadJobResult `json:"jobs"`
	Total     int                    `json:"total"`
	Success   int                    `json:"success"`
	Failed    int                    `json:"failed"`
	FailedIDs []string               `json:"failed_ids,omitempty"`
	Errors    map[string]string      `json:"errors,omitempty"`
}

// BatchUploadJobResult represents a single job result in batch upload.
type BatchUploadJobResult struct {
	JobID    string `json:"job_id"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

// BatchUploadDocuments handles batch document upload requests.
// Supports both multipart/form-data (multiple files) and ZIP file upload.
func (h *Handler) BatchUploadDocuments(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	maxMemory := int64(100 << 20) // 100MB for batch uploads
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "failed to parse form")
		return
	}

	var results []BatchUploadJobResult
	var failedIDs []string
	errors := make(map[string]string)

	// Check if it's a ZIP file
	zipFile, zipHeader, err := r.FormFile("zip")
	if err == nil && zipHeader != nil {
		// Handle ZIP file upload
		defer zipFile.Close()
		results, failedIDs, errors = h.handleZIPUpload(r.Context(), zipFile, zipHeader)
	} else {
		// Handle multiple files upload
		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "no files provided. Use 'files' field for multiple files or 'zip' for ZIP archive")
			return
		}

		// Limit batch size
		if len(files) > 100 {
			WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "maximum 100 files per batch")
			return
		}

		results, failedIDs, errors = h.handleMultipleFilesUpload(r.Context(), files, r)
	}

	// Count success and failures
	success := 0
	for _, result := range results {
		if result.Status == "pending" {
			success++
		} else {
			failedIDs = append(failedIDs, result.JobID)
		}
	}

	response := BatchUploadDocumentsResponse{
		Jobs:      results,
		Total:     len(results),
		Success:   success,
		Failed:    len(failedIDs),
		FailedIDs: failedIDs,
		Errors:    errors,
	}

	// Return appropriate status code
	status := http.StatusOK
	if len(failedIDs) > 0 && success == 0 {
		status = http.StatusInternalServerError
	} else if len(failedIDs) > 0 {
		status = http.StatusMultiStatus // 207
	}

	WriteJSON(w, status, response)
}

// handleMultipleFilesUpload processes multiple files from multipart form.
func (h *Handler) handleMultipleFilesUpload(ctx context.Context, files []*multipart.FileHeader, r *http.Request) ([]BatchUploadJobResult, []string, map[string]string) {
	var results []BatchUploadJobResult
	var failedIDs []string
	errors := make(map[string]string)

	// Parse metadata if provided
	var globalMetadata map[string]string
	if metaStr := r.FormValue("metadata"); metaStr != "" {
		json.Unmarshal([]byte(metaStr), &globalMetadata)
	} else {
		globalMetadata = make(map[string]string)
	}

	for _, fileHeader := range files {
		result := BatchUploadJobResult{
			Filename: fileHeader.Filename,
		}

		// Open file
		file, err := fileHeader.Open()
		if err != nil {
			result.Status = "failed"
			result.Error = "failed to open file"
			result.JobID = fileHeader.Filename
			results = append(results, result)
			failedIDs = append(failedIDs, result.JobID)
			errors[result.JobID] = err.Error()
			continue
		}

		// Process single file (reuse UploadDocument logic)
		jobID, err := h.processSingleFile(ctx, file, fileHeader, globalMetadata)
		file.Close()

		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.JobID = fileHeader.Filename
			results = append(results, result)
			failedIDs = append(failedIDs, result.JobID)
			errors[result.JobID] = err.Error()
			continue
		}

		result.JobID = jobID
		result.Status = "pending"
		results = append(results, result)
	}

	return results, failedIDs, errors
}

// handleZIPUpload processes a ZIP file upload.
func (h *Handler) handleZIPUpload(ctx context.Context, zipFile multipart.File, zipHeader *multipart.FileHeader) ([]BatchUploadJobResult, []string, map[string]string) {
	var results []BatchUploadJobResult
	var failedIDs []string
	errors := make(map[string]string)

	// Read ZIP file into memory
	zipData, err := io.ReadAll(zipFile)
	if err != nil {
		result := BatchUploadJobResult{
			Filename: zipHeader.Filename,
			Status:   "failed",
			Error:    "failed to read ZIP file",
			JobID:    zipHeader.Filename,
		}
		results = append(results, result)
		failedIDs = append(failedIDs, result.JobID)
		errors[result.JobID] = err.Error()
		return results, failedIDs, errors
	}

	// Create a reader for the ZIP data
	zipDataReader := bytes.NewReader(zipData)

	// Open ZIP archive
	zipReader, err := zip.NewReader(zipDataReader, int64(len(zipData)))
	if err != nil {
		result := BatchUploadJobResult{
			Filename: zipHeader.Filename,
			Status:   "failed",
			Error:    "invalid ZIP file",
			JobID:    zipHeader.Filename,
		}
		results = append(results, result)
		failedIDs = append(failedIDs, result.JobID)
		errors[result.JobID] = err.Error()
		return results, failedIDs, errors
	}

	// Create temp directory for extracted files
	tempDir := filepath.Join(os.TempDir(), "rag-uploads", uuid.New().String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		result := BatchUploadJobResult{
			Filename: zipHeader.Filename,
			Status:   "failed",
			Error:    "failed to create temp directory",
			JobID:    zipHeader.Filename,
		}
		results = append(results, result)
		failedIDs = append(failedIDs, result.JobID)
		errors[result.JobID] = err.Error()
		return results, failedIDs, errors
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Extract and process each file in ZIP
	for _, file := range zipReader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		// Limit file count
		if len(results) >= 100 {
			result := BatchUploadJobResult{
				Filename: file.Name,
				Status:   "failed",
				Error:    "maximum 100 files per ZIP",
				JobID:    file.Name,
			}
			results = append(results, result)
			failedIDs = append(failedIDs, result.JobID)
			errors[result.JobID] = "batch size limit exceeded"
			continue
		}

		result := BatchUploadJobResult{
			Filename: file.Name,
		}

		// Open file from ZIP
		rc, err := file.Open()
		if err != nil {
			result.Status = "failed"
			result.Error = "failed to open file in ZIP"
			result.JobID = file.Name
			results = append(results, result)
			failedIDs = append(failedIDs, result.JobID)
			errors[result.JobID] = err.Error()
			continue
		}

		// Save extracted file temporarily
		extractedPath := filepath.Join(tempDir, filepath.Base(file.Name))
		extractedFile, err := os.Create(extractedPath)
		if err != nil {
			rc.Close()
			result.Status = "failed"
			result.Error = "failed to create temp file"
			result.JobID = file.Name
			results = append(results, result)
			failedIDs = append(failedIDs, result.JobID)
			errors[result.JobID] = err.Error()
			continue
		}

		if _, err := io.Copy(extractedFile, rc); err != nil {
			rc.Close()
			extractedFile.Close()
			os.Remove(extractedPath)
			result.Status = "failed"
			result.Error = "failed to extract file"
			result.JobID = file.Name
			results = append(results, result)
			failedIDs = append(failedIDs, result.JobID)
			errors[result.JobID] = err.Error()
			continue
		}
		rc.Close()
		extractedFile.Close()

		// Process extracted file
		jobID, err := h.processExtractedFile(ctx, extractedPath, file.Name)
		if err != nil {
			os.Remove(extractedPath)
			result.Status = "failed"
			result.Error = err.Error()
			result.JobID = file.Name
			results = append(results, result)
			failedIDs = append(failedIDs, result.JobID)
			errors[result.JobID] = err.Error()
			continue
		}

		result.JobID = jobID
		result.Status = "pending"
		results = append(results, result)
	}

	return results, failedIDs, errors
}

// processExtractedFile processes an extracted file from ZIP.
func (h *Handler) processExtractedFile(ctx context.Context, filePath, filename string) (string, error) {
	// Detect format
	format, err := ingestion.DetectFormat(filename)
	if err != nil {
		return "", fmt.Errorf("unsupported format: %w", err)
	}

	// Check if parser is available
	if _, err := h.parserRegistry.GetParser(format); err != nil {
		return "", fmt.Errorf("no parser available for format: %w", err)
	}

	// Create job
	jobID := uuid.New().String()
	payload, _ := json.Marshal(DocumentIngestPayload{
		FilePath: filePath,
		Metadata: make(map[string]string),
	})

	j := job.NewJob(jobID, job.TypeDocumentIngest, payload)

	// Submit job
	h.metrics.JobSubmitted.Inc()
	if err := h.jobQueue.Submit(ctx, j); err != nil {
		h.metrics.JobFailed.Inc()
		return "", fmt.Errorf("failed to queue document: %w", err)
	}

	return jobID, nil
}

// processSingleFile processes a single file and returns job ID.
func (h *Handler) processSingleFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, metadata map[string]string) (string, error) {
	// Detect format
	format, err := ingestion.DetectFormat(header.Filename)
	if err != nil {
		return "", fmt.Errorf("unsupported format: %w", err)
	}

	// Check if parser is available
	if _, err := h.parserRegistry.GetParser(format); err != nil {
		return "", fmt.Errorf("no parser available for format: %w", err)
	}

	// Save file temporarily
	tempDir := filepath.Join(os.TempDir(), "rag-uploads")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	tempPath := filepath.Join(tempDir, uuid.New().String()+filepath.Ext(header.Filename))
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(tempFile, file); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to copy file: %w", err)
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
	if err := h.jobQueue.Submit(ctx, j); err != nil {
		h.metrics.JobFailed.Inc()
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to queue document: %w", err)
	}

	return jobID, nil
}

// BatchDeleteDocumentsRequest represents a batch delete request.
type BatchDeleteDocumentsRequest struct {
	DocumentIDs []string `json:"document_ids"`
	Hard        bool     `json:"hard"` // If true, perform hard delete
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

// BatchDeleteDocuments handles batch document deletion requests.
func (h *Handler) BatchDeleteDocuments(w http.ResponseWriter, r *http.Request) {
	var req BatchDeleteDocumentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	if len(req.DocumentIDs) == 0 {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "document_ids is required")
		return
	}

	// Limit batch size
	if len(req.DocumentIDs) > 100 {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "maximum 100 documents per batch")
		return
	}

	deleted := make([]string, 0)
	failed := make([]string, 0)
	errors := make(map[string]string)

	// Delete each document
	for _, docID := range req.DocumentIDs {
		if docID == "" {
			continue
		}

		var err error
		if req.Hard {
			err = h.vectorStore.HardDeleteDocument(r.Context(), docID)
		} else {
			err = h.vectorStore.SoftDeleteDocument(r.Context(), docID)
		}

		if err != nil {
			h.logger.Error("failed to delete document in batch", zap.String("document_id", docID), zap.Error(err))
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

