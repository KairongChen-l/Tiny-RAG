package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ListDocumentVersions handles listing document versions.
func (h *Handler) ListDocumentVersions(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		WriteError(c, 400, ErrCodeBadRequest, "document id is required")
		return
	}

	versions, err := h.vectorStore.ListVersions(c.Request.Context(), docID)
	if err != nil {
		h.logger.Error("failed to list document versions", zap.String("document_id", docID), zap.Error(err))
		WriteError(c, 500, ErrCodeInternalError, "failed to list versions")
		return
	}

	WriteJSON(c, 200, map[string]interface{}{
		"document_id": docID,
		"versions":    versions,
	})
}

// RestoreDocumentVersionRequest represents a restore version request.
type RestoreDocumentVersionRequest struct {
	Version int `json:"version"`
}

// RestoreDocumentVersion handles document version restoration.
func (h *Handler) RestoreDocumentVersion(c *gin.Context) {
	docID := c.Param("id")
	if docID == "" {
		WriteError(c, 400, ErrCodeBadRequest, "document id is required")
		return
	}

	var req RestoreDocumentVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteError(c, 400, ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.Version <= 0 {
		WriteError(c, 400, ErrCodeBadRequest, "version must be greater than 0")
		return
	}

	err := h.vectorStore.RestoreVersion(c.Request.Context(), docID, req.Version)
	if err != nil {
		h.logger.Error("failed to restore document version",
			zap.String("document_id", docID),
			zap.Int("version", req.Version),
			zap.Error(err),
		)
		// Check if it's a not found error
		if err.Error() != "" && (err.Error() == "version not found" || err.Error() == "document not found") {
			WriteError(c, 404, ErrCodeNotFound, "version not found")
			return
		}
		WriteError(c, 500, ErrCodeInternalError, "failed to restore version")
		return
	}

	WriteJSON(c, 200, map[string]interface{}{
		"message":     "document version restored",
		"document_id": docID,
		"version":     req.Version,
	})
}
