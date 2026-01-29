// Package handler provides HTTP request handlers.
package handler

import (
	"go.uber.org/zap"

	"github.com/krc/rag/internal/conversation"
	"github.com/krc/rag/internal/embedding"
	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/ingestion"
	"github.com/krc/rag/internal/job"
	"github.com/krc/rag/internal/metrics"
	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/retrieval"
	"github.com/krc/rag/internal/service"
	"github.com/krc/rag/pkg/cache"
	"github.com/krc/rag/pkg/config"
	"github.com/krc/rag/pkg/search"
)

// Handler holds all HTTP handlers and dependencies.
// After refactoring, Handler should primarily use Service layer.
type Handler struct {
	// Service layer dependencies
	ingestionService    *service.IngestionService
	searchService       *service.SearchService
	conversationService *service.ConversationService

	// Legacy dependencies (kept for backward compatibility during transition)
	config         *config.Config
	logger         *zap.Logger
	parserRegistry *ingestion.ParserRegistry
	vectorStore    index.VectorStore
	embedder       embedding.Embedder
	retriever      retrieval.Retriever
	promptBuilder  prompt.PromptBuilder
	llmRegistry    *generation.LLMRegistry
	jobQueue       job.Worker
	jobStore       job.Store
	convStore      conversation.Store
	metrics        *metrics.Metrics
	queryCache     cache.Cache // Query result cache
	objectStore    ObjectStore // Optional (nil = local temp files)
	events         EventPublisher // Optional (nil = disabled)
	esClient       *search.ElasticsearchClient // Optional Elasticsearch client
}

// Config holds handler dependencies.
type Config struct {
	// Service layer dependencies
	IngestionService    *service.IngestionService
	SearchService       *service.SearchService
	ConversationService *service.ConversationService

	// Legacy dependencies (kept for backward compatibility during transition)
	Config         *config.Config
	Logger         *zap.Logger
	ParserRegistry *ingestion.ParserRegistry
	VectorStore    index.VectorStore
	Embedder       embedding.Embedder
	Retriever      retrieval.Retriever
	PromptBuilder  prompt.PromptBuilder
	LLMRegistry    *generation.LLMRegistry
	JobQueue       job.Worker
	JobStore       job.Store
	ConvStore      conversation.Store
	Metrics        *metrics.Metrics
	QueryCache     cache.Cache // Optional query result cache
	ObjectStore    ObjectStore // Optional
	Events         EventPublisher // Optional
	ESClient       *search.ElasticsearchClient // Optional Elasticsearch client
}

// New creates a new Handler.
func New(cfg Config) *Handler {
	convStore := cfg.ConvStore
	if convStore == nil {
		convStore = conversation.NewMemoryStore()
	}

	metricsInstance := cfg.Metrics
	if metricsInstance == nil {
		metricsInstance = metrics.NewMetrics()
	}

	return &Handler{
		// Service layer
		ingestionService:    cfg.IngestionService,
		searchService:       cfg.SearchService,
		conversationService: cfg.ConversationService,

		// Legacy dependencies (for backward compatibility)
		config:         cfg.Config,
		logger:         cfg.Logger,
		parserRegistry: cfg.ParserRegistry,
		vectorStore:    cfg.VectorStore,
		embedder:       cfg.Embedder,
		retriever:      cfg.Retriever,
		promptBuilder:  cfg.PromptBuilder,
		llmRegistry:    cfg.LLMRegistry,
		jobQueue:       cfg.JobQueue,
		jobStore:       cfg.JobStore,
		convStore:      convStore,
		metrics:        metricsInstance,
		queryCache:     cfg.QueryCache,
		objectStore:    cfg.ObjectStore,
		events:         cfg.Events,
		esClient:       cfg.ESClient,
	}
}
