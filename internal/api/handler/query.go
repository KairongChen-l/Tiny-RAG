package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/retrieval"
	"github.com/krc/rag/pkg/cache"
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

	// Check cache if available
	if h.queryCache != nil {
		cacheKey := cache.CacheKey("query", req.Query, map[string]interface{}{
			"top_k":        req.Options.TopK,
			"filter":       req.Options.Filter,
			"enable_rerank": req.Options.EnableRerank,
			"provider":     req.Options.Provider,
		})

		cached, found, err := h.queryCache.Get(r.Context(), cacheKey)
		if err == nil && found {
			// Return cached response
			var cachedResp QueryResponse
			if err := json.Unmarshal(cached, &cachedResp); err == nil {
				cachedResp.Cached = true
				WriteJSON(w, http.StatusOK, cachedResp)
				return
			}
		}
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

	// Generate response with timeout
	llmTimeout := 60 * time.Second // Default timeout
	if h.config != nil {
		// Use server read timeout if available, otherwise use default
		if h.config.Server.ReadTimeout > 0 {
			llmTimeout = h.config.Server.ReadTimeout
		}
	}

	llmCtx, cancel := context.WithTimeout(r.Context(), llmTimeout)
	defer cancel()

	llmStart := time.Now()
	if h.metrics != nil {
		h.metrics.LLMRequests.Inc()
	}
	resp, err := llmClient.Generate(llmCtx, p)
	llmDuration := time.Since(llmStart).Seconds()

	if err != nil {
		if h.metrics != nil {
			h.metrics.LLMErrors.Inc()
		}

		// Check if error is due to timeout
		if llmCtx.Err() == context.DeadlineExceeded {
			h.logger.Error("LLM generation timeout",
				zap.String("provider", llmClient.Name()),
				zap.Duration("timeout", llmTimeout),
			)
			WriteErrorWithContext(w, r.Context(), http.StatusRequestTimeout, ErrCodeTimeout, "LLM generation timeout", map[string]interface{}{
				"provider": llmClient.Name(),
				"timeout":  llmTimeout.String(),
			})
			return
		}

		h.logger.Error("generation failed",
			zap.Error(err),
			zap.String("provider", llmClient.Name()),
			zap.Duration("duration", time.Since(llmStart)),
		)
		// Return more detailed error message to client
		WriteErrorWithContext(w, r.Context(), http.StatusInternalServerError, ErrCodeInternalError, fmt.Sprintf("LLM generation failed: %v", err), map[string]interface{}{
			"provider": llmClient.Name(),
			"duration": time.Since(llmStart).String(),
		})
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

	queryResp := QueryResponse{
		Answer:     resp.Answer,
		Citations:  citations,
		TokensUsed: resp.TokensUsed,
		Cached:     false,
	}

	// Cache the response if cache is available
	if h.queryCache != nil {
		cacheKey := cache.CacheKey("query", req.Query, map[string]interface{}{
			"top_k":        req.Options.TopK,
			"filter":       req.Options.Filter,
			"enable_rerank": req.Options.EnableRerank,
			"provider":     req.Options.Provider,
		})

		respData, err := json.Marshal(queryResp)
		if err == nil {
			// Cache in background, don't block response
			go func() {
				if err := h.queryCache.Set(context.Background(), cacheKey, respData); err != nil {
					h.logger.Warn("failed to cache query result", zap.Error(err))
				}
			}()
		}
	}

	WriteJSON(w, http.StatusOK, queryResp)
}
