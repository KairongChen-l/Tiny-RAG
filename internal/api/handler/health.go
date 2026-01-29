package handler

import (
	"github.com/gin-gonic/gin"
)

// HealthResponse represents health check response.
type HealthResponse struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
}

// Health handles health check requests.
func (h *Handler) Health(c *gin.Context) {
	components := make(map[string]string)

	// Check database
	if h.vectorStore != nil {
		components["database"] = "ok"
	} else {
		components["database"] = "not_configured"
	}

	// Check embedding
	if h.embedder != nil {
		components["embedding"] = "ok"
	} else {
		components["embedding"] = "not_configured"
	}

	// Check LLM
	if h.llmRegistry != nil {
		if _, err := h.llmRegistry.GetDefault(); err == nil {
			components["llm"] = "ok"
		} else {
			components["llm"] = "no_default"
		}
	} else {
		components["llm"] = "not_configured"
	}

	status := "healthy"
	for _, v := range components {
		if v != "ok" {
			status = "degraded"
			break
		}
	}

	WriteJSON(c, 200, HealthResponse{
		Status:     status,
		Components: components,
	})
}
