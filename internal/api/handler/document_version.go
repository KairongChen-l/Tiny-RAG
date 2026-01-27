package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/index"
)

// ListDocumentVersions handles listing document versions.
func (h *Handler) ListDocumentVersions(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	if docID == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "document id is required")
		return
	}

	versions, err := h.vectorStore.ListVersions(r.Context(), docID)
	if err != nil {
		h.logger.Error("failed to list document versions", zap.String("document_id", docID), zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list versions")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"document_id": docID,
		"versions":    versions,
	})
}

// RestoreDocumentVersionRequest represents a restore version request.
type RestoreDocumentVersionRequest struct {
	Version int `json:"version"`
}

// RestoreDocumentVersion handles document version restoration.
func (h *Handler) RestoreDocumentVersion(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	if docID == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "document id is required")
		return
	}

	var req RestoreDocumentVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.Version <= 0 {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "version must be greater than 0")
		return
	}

	err := h.vectorStore.RestoreVersion(r.Context(), docID, req.Version)
	if err != nil {
		h.logger.Error("failed to restore document version",
			zap.String("document_id", docID),
			zap.Int("version", req.Version),
			zap.Error(err),
		)
		// Check if it's a not found error
		if err.Error() != "" && (err.Error() == "version not found" || err.Error() == "document not found") {
			WriteError(w, http.StatusNotFound, ErrCodeNotFound, "version not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to restore version")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":     "document version restored",
		"document_id": docID,
		"version":     req.Version,
	})
}

