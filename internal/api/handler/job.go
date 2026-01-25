package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// JobResponse represents job status response.
type JobResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// GetJob handles job status requests.
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "job ID is required")
		return
	}

	j, err := h.jobStore.Get(r.Context(), jobID)
	if err != nil {
		WriteError(w, http.StatusNotFound, ErrCodeNotFound, "job not found")
		return
	}

	WriteJSON(w, http.StatusOK, JobResponse{
		ID:        j.ID,
		Type:      string(j.Type),
		Status:    string(j.Status),
		Progress:  j.Progress,
		Error:     j.Error,
		CreatedAt: j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: j.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

