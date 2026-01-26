package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/retrieval"
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
}

// CitationInfo represents citation information.
type CitationInfo struct {
	ID      int    `json:"id"`
	Source  string `json:"source"`
	Section string `json:"section"`
	Preview string `json:"preview"`
}

// Query handles RAG query requests.
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeValidation, "query is required")
		return
	}

	// Check if retriever is available
	if h.retriever == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeInternalError, "retriever not configured")
		return
	}

	// Build retrieval options
	retrieveOpts := retrieval.DefaultRetrieveOptions()
	if req.Options.TopK > 0 {
		retrieveOpts.TopK = req.Options.TopK
	}
	if req.Options.Filter != nil {
		retrieveOpts.MetadataFilter = req.Options.Filter
	}
	retrieveOpts.EnableRerank = req.Options.EnableRerank

	// Retrieve relevant chunks
	retrievalStart := time.Now()
	if h.metrics != nil {
		h.metrics.RetrievalRequests.Inc()
	}
	result, err := h.retriever.Retrieve(r.Context(), req.Query, retrieveOpts)
	retrievalDuration := time.Since(retrievalStart).Seconds()

	if err != nil {
		if h.metrics != nil {
			h.metrics.RetrievalErrors.Inc()
		}
		h.logger.Error("retrieval failed", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "retrieval failed")
		return
	}

	if h.metrics != nil {
		h.metrics.RetrievalDuration.Observe(retrievalDuration)
		h.metrics.RetrievalResults.Observe(float64(len(result.Chunks)))
	}

	// Build prompt
	promptOpts := prompt.DefaultPromptOptions()
	promptOpts.MaxContextTokens = h.config.Prompt.MaxContextTokens
	promptOpts.IncludeCitations = h.config.Prompt.IncludeCitations

	p, err := h.promptBuilder.Build(r.Context(), req.Query, result.Chunks, promptOpts)
	if err != nil {
		h.logger.Error("prompt building failed", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "prompt building failed")
		return
	}

	// Get LLM
	var llm = h.llmRegistry
	var llmClient, _ = llm.GetDefault()
	if req.Options.Provider != "" {
		if client, err := llm.Get(req.Options.Provider); err == nil {
			llmClient = client
		}
	}

	if llmClient == nil {
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "no LLM provider available")
		return
	}

	// Generate response
	llmStart := time.Now()
	if h.metrics != nil {
		h.metrics.LLMRequests.Inc()
	}
	resp, err := llmClient.Generate(r.Context(), p)
	llmDuration := time.Since(llmStart).Seconds()

	if err != nil {
		if h.metrics != nil {
			h.metrics.LLMErrors.Inc()
		}
		h.logger.Error("generation failed", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "generation failed")
		return
	}

	if h.metrics != nil {
		h.metrics.LLMDuration.Observe(llmDuration)
		h.metrics.LLMTokensUsed.Add(float64(resp.TokensUsed))
	}

	// Build citations response
	citations := make([]CitationInfo, len(p.Citations))
	for i, c := range p.Citations {
		citations[i] = CitationInfo{
			ID:      c.ID,
			Source:  c.Source,
			Section: c.SectionPath,
			Preview: c.Preview,
		}
	}

	WriteJSON(w, http.StatusOK, QueryResponse{
		Answer:     resp.Answer,
		Citations:  citations,
		TokensUsed: resp.TokensUsed,
	})
}
