package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/service"
)

// QueryRequest represents a RAG query request.
type QueryRequest struct {
	Query   string       `json:"query"`
	Options QueryOptions `json:"options,omitempty"`
}

// QueryOptions represents query options.
type QueryOptions struct {
	TopK         int               `json:"top_k,omitempty"`
	Filter       map[string]string `json:"filter,omitempty"`
	EnableRerank bool              `json:"enable_rerank,omitempty"`
	Provider     string            `json:"provider,omitempty"`
}

// QueryResponse represents a RAG query response.
type QueryResponse struct {
	Answer     string         `json:"answer"`
	Citations  []CitationInfo `json:"citations"`
	TokensUsed int            `json:"tokens_used"`
	Cached     bool           `json:"cached,omitempty"` // Indicates if result was from cache
}

// CitationInfo represents citation information.
type CitationInfo struct {
	ID      int    `json:"id"`
	Source  string `json:"source"`
	Section string `json:"section"`
	Preview string `json:"preview"`
}

// Query handles RAG query requests.
func (h *Handler) Query(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteError(c, 400, ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		WriteError(c, 400, ErrCodeValidation, "query is required")
		return
	}

	ctx := c.Request.Context()

	// Use SearchService if available
	if h.searchService != nil {
		searchReq := service.SearchRequest{
			Query:    req.Query,
			TopK:     req.Options.TopK,
			Filter:   req.Options.Filter,
			Rerank:   req.Options.EnableRerank,
			Provider: req.Options.Provider,
		}

		searchResp, err := h.searchService.Search(ctx, searchReq)
		if err != nil {
			h.logger.Error("search failed", zap.Error(err))
			WriteError(c, 500, ErrCodeInternalError, "search failed")
			return
		}

		// Convert citations
		citations := make([]CitationInfo, len(searchResp.Citations))
		for i, cit := range searchResp.Citations {
			citations[i] = CitationInfo{
				ID:      cit.ID,
				Source:  cit.Source,
				Section: cit.Section,
				Preview: cit.Preview,
			}
		}

		WriteJSON(c, 200, QueryResponse{
			Answer:     searchResp.Answer,
			Citations:  citations,
			TokensUsed: searchResp.TokensUsed,
			Cached:     searchResp.Cached,
		})
		return
	}

	// Fallback to legacy implementation if SearchService is not available
	// This maintains backward compatibility during transition
	// Check if retriever is available
	if h.retriever == nil {
		WriteError(c, 503, ErrCodeInternalError, "retriever not configured")
		return
	}

	// Use legacy implementation (simplified for testing)
	WriteError(c, 503, ErrCodeInternalError, "search service not configured, please use SearchService")
}
