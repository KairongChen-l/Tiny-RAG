package service

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/retrieval"
	"github.com/krc/rag/pkg/cache"
)

// SearchService handles RAG search and generation business logic.
type SearchService struct {
	retriever     retrieval.Retriever
	promptBuilder prompt.PromptBuilder
	llmRegistry   *generation.LLMRegistry
	queryCache    cache.Cache
	logger        *zap.Logger
}

// NewSearchService creates a new search service.
func NewSearchService(
	retriever retrieval.Retriever,
	promptBuilder prompt.PromptBuilder,
	llmRegistry *generation.LLMRegistry,
	queryCache cache.Cache,
	logger *zap.Logger,
) *SearchService {
	return &SearchService{
		retriever:     retriever,
		promptBuilder: promptBuilder,
		llmRegistry:   llmRegistry,
		queryCache:    queryCache,
		logger:        logger,
	}
}

// SearchRequest represents a search request.
type SearchRequest struct {
	Query   string
	TopK    int
	Filter  map[string]string
	Rerank  bool
	Provider string
}

// SearchResponse represents a search response.
type SearchResponse struct {
	Answer     string
	Citations  []CitationInfo
	TokensUsed int
	Cached     bool
}

// CitationInfo represents citation information.
type CitationInfo struct {
	ID      int
	Source  string
	Section string
	Preview string
}

// Search performs a RAG search and generates an answer.
func (s *SearchService) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	// Check cache if available
	if s.queryCache != nil {
		cacheKey := cache.CacheKey("query", req.Query, map[string]interface{}{
			"top_k":         req.TopK,
			"filter":        req.Filter,
			"enable_rerank": req.Rerank,
			"provider":      req.Provider,
		})

		cached, found, err := s.queryCache.Get(ctx, cacheKey)
		if err == nil && found {
			var cachedResp SearchResponse
			if err := json.Unmarshal(cached, &cachedResp); err == nil {
				cachedResp.Cached = true
				return &cachedResp, nil
			}
		}
	}

	// Check if retriever is available
	if s.retriever == nil {
		return nil, fmt.Errorf("retriever not configured")
	}

	// Build retrieval options
	retrieveOpts := retrieval.DefaultRetrieveOptions()
	if req.TopK > 0 {
		retrieveOpts.TopK = req.TopK
	}
	if req.Filter != nil {
		retrieveOpts.MetadataFilter = req.Filter
	}
	retrieveOpts.EnableRerank = req.Rerank

	// Retrieve relevant chunks
	retrievalResult, err := s.retriever.Retrieve(ctx, req.Query, retrieveOpts)
	if err != nil {
		return nil, fmt.Errorf("retrieval failed: %w", err)
	}

	// Get LLM provider
	var llm generation.LLM
	if req.Provider != "" {
		var err error
		llm, err = s.llmRegistry.Get(req.Provider)
		if err != nil {
			s.logger.Warn("failed to get LLM provider, using default", zap.String("provider", req.Provider), zap.Error(err))
			llm, _ = s.llmRegistry.GetDefault()
		}
	} else {
		var err error
		llm, err = s.llmRegistry.GetDefault()
		if err != nil {
			return nil, fmt.Errorf("no LLM configured: %w", err)
		}
	}

	// Build prompt
	promptOpts := prompt.DefaultPromptOptions()
	p, err := s.promptBuilder.Build(ctx, req.Query, retrievalResult.Chunks, promptOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Generate answer
	response, err := llm.Generate(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Convert citations
	citations := make([]CitationInfo, len(p.Citations))
	for i, cit := range p.Citations {
		citations[i] = CitationInfo{
			ID:      cit.ID,
			Source:  cit.Source,
			Section: cit.SectionPath,
			Preview: cit.Preview,
		}
	}

	result := &SearchResponse{
		Answer:     response.Answer,
		Citations:  citations,
		TokensUsed: response.TokensUsed,
		Cached:     false,
	}

	// Cache result if cache is available
	if s.queryCache != nil {
		resultBytes, err := json.Marshal(result)
		if err == nil {
			cacheKey := cache.CacheKey("query", req.Query, map[string]interface{}{
				"top_k":         req.TopK,
				"filter":        req.Filter,
				"enable_rerank": req.Rerank,
				"provider":      req.Provider,
			})
			_ = s.queryCache.Set(ctx, cacheKey, resultBytes)
		}
	}

	return result, nil
}

// StreamChunk represents a chunk of streaming response.
type StreamChunk struct {
	Text        string // Text content of this chunk
	Done        bool   // Whether this is the final chunk
	TokensUsed  int    // Total tokens used so far
	FinishReason string // Finish reason (if done)
}

// StreamCallback is called for each chunk in a streaming response.
type StreamCallback func(chunk StreamChunk) error

// SearchStream performs a streaming RAG search and generates an answer incrementally.
func (s *SearchService) SearchStream(ctx context.Context, req SearchRequest, callback StreamCallback) error {
	// Check if retriever is available
	if s.retriever == nil {
		return fmt.Errorf("retriever not configured")
	}

	// Build retrieval options
	retrieveOpts := retrieval.DefaultRetrieveOptions()
	if req.TopK > 0 {
		retrieveOpts.TopK = req.TopK
	}
	if req.Filter != nil {
		retrieveOpts.MetadataFilter = req.Filter
	}
	retrieveOpts.EnableRerank = req.Rerank

	// Retrieve relevant chunks
	retrievalResult, err := s.retriever.Retrieve(ctx, req.Query, retrieveOpts)
	if err != nil {
		return fmt.Errorf("retrieval failed: %w", err)
	}

	// Get LLM provider
	var llm generation.LLM
	if req.Provider != "" {
		var err error
		llm, err = s.llmRegistry.Get(req.Provider)
		if err != nil {
			s.logger.Warn("failed to get LLM provider, using default", zap.String("provider", req.Provider), zap.Error(err))
			llm, _ = s.llmRegistry.GetDefault()
		}
	} else {
		var err error
		llm, err = s.llmRegistry.GetDefault()
		if err != nil {
			return fmt.Errorf("no LLM configured: %w", err)
		}
	}

	// Check if LLM supports streaming
	streamableLLM, ok := llm.(generation.StreamableLLM)
	if !ok {
		// Fallback to non-streaming
		response, err := s.Search(ctx, req)
		if err != nil {
			return err
		}
		// Send as single chunk
		return callback(StreamChunk{
			Text:        response.Answer,
			Done:        true,
			TokensUsed:  response.TokensUsed,
			FinishReason: "stop",
		})
	}

	// Build prompt
	promptOpts := prompt.DefaultPromptOptions()
	p, err := s.promptBuilder.Build(ctx, req.Query, retrievalResult.Chunks, promptOpts)
	if err != nil {
		return fmt.Errorf("failed to build prompt: %w", err)
	}

	// Generate streaming response
	chunkChan, err := streamableLLM.GenerateStream(ctx, p)
	if err != nil {
		return fmt.Errorf("LLM streaming failed: %w", err)
	}

	// Forward chunks to callback
	for chunk := range chunkChan {
		if err := callback(StreamChunk{
			Text:        chunk.Text,
			Done:        chunk.Done,
			TokensUsed:  chunk.TokensUsed,
			FinishReason: chunk.FinishReason,
		}); err != nil {
			return err
		}
		if chunk.Done {
			break
		}
	}

	return nil
}


