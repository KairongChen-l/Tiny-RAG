package handler

import (
	"github.com/gin-gonic/gin"
)

// ProgressEntryResponse represents a progress entry in the response.
type ProgressEntryResponse struct {
	Progress     int    `json:"progress"`
	Stage        string `json:"stage"`
	StageMessage string `json:"stage_message"`
	Timestamp    string `json:"timestamp"`
}

// JobResponse represents job status response.
type JobResponse struct {
	ID              string                  `json:"id"`
	Type            string                  `json:"type"`
	Status          string                  `json:"status"`
	Progress        int                     `json:"progress"`
	CurrentStage    string                  `json:"current_stage,omitempty"`
	StageMessage    string                  `json:"stage_message,omitempty"`
	ProgressHistory []ProgressEntryResponse `json:"progress_history,omitempty"`
	Error           string                  `json:"error,omitempty"`
	CreatedAt       string                  `json:"created_at"`
	UpdatedAt       string                  `json:"updated_at"`
}

// GetJob handles job status requests.
func (h *Handler) GetJob(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		WriteError(c, 400, ErrCodeBadRequest, "job ID is required")
		return
	}

	j, err := h.jobStore.Get(c.Request.Context(), jobID)
	if err != nil {
		WriteError(c, 404, ErrCodeNotFound, "job not found")
		return
	}

	// Convert progress history
	history := make([]ProgressEntryResponse, 0, len(j.ProgressHistory))
	for _, entry := range j.ProgressHistory {
		history = append(history, ProgressEntryResponse{
			Progress:     entry.Progress,
			Stage:        entry.Stage,
			StageMessage: entry.StageMessage,
			Timestamp:    entry.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	WriteJSON(c, 200, JobResponse{
		ID:              j.ID,
		Type:            string(j.Type),
		Status:          string(j.Status),
		Progress:        j.Progress,
		CurrentStage:    j.CurrentStage,
		StageMessage:    j.StageMessage,
		ProgressHistory: history,
		Error:           j.Error,
		CreatedAt:       j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       j.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}
