// Package bootstrap provides application lifecycle management.
package bootstrap

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/api"
	"github.com/krc/rag/internal/api/handler"
	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/common"
	"github.com/krc/rag/internal/conversation"
	"github.com/krc/rag/internal/embedding"
	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/ingestion"
	"github.com/krc/rag/internal/job"
	"github.com/krc/rag/internal/metrics"
	"github.com/krc/rag/internal/mq"
	"github.com/krc/rag/internal/processor"
	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/repository"
	"github.com/krc/rag/internal/retrieval"
	"github.com/krc/rag/internal/service"
	"github.com/krc/rag/internal/ws"
	"github.com/krc/rag/pkg/cache"
	"github.com/krc/rag/pkg/config"
	"github.com/krc/rag/pkg/search"
	"time"
)

// InfrastructureComponents holds infrastructure components (DB, Redis, ES, Kafka, MinIO).
type InfrastructureComponents struct {
	VectorStore    index.VectorStore
	ESResource     *ElasticsearchResource
	MinIOResource  *MinIOResource
	KafkaResource  *KafkaProducerResource
	ObjectStore    common.ObjectStore
	EventPublisher common.EventPublisher
	EventBus       mq.EventBus
}

// BusinessComponents holds business logic components (Parser, Chunker, Embedder, LLM).
type BusinessComponents struct {
	ParserRegistry *ingestion.ParserRegistry
	Chunker        chunking.Chunker
	Embedder       embedding.Embedder
	LLMRegistry    *generation.LLMRegistry
	Retriever      retrieval.Retriever
	PromptBuilder  prompt.PromptBuilder
}

// ServiceComponents holds service layer components.
type ServiceComponents struct {
	IngestionService    *service.IngestionService
	SearchService       *service.SearchService
	ConversationService *service.ConversationService
	ConsumerRegistry    *mq.ConsumerRegistry
}

// APILayerComponents holds API layer components.
type APILayerComponents struct {
	Handler    *handler.Handler
	Router     *api.Router
	HTTPServer *HTTPServer
	WSHub      *ws.Hub
	WSHandler  *ws.Handler
}

// initInfrastructure initializes infrastructure components (DB, Redis, ES, Kafka, MinIO).
func initInfrastructure(cfg *config.Config, logger *zap.Logger, resourceMgr *ResourceManager) (*InfrastructureComponents, error) {
	// Initialize vector store
	vectorStore, err := initVectorStore(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize vector store: %w", err)
	}
	resourceMgr.Register(vectorStore)

	// Initialize optional integrations
	var objStore common.ObjectStore
	var eventPublisher common.EventPublisher
	minioResource := initObjectStore(cfg, logger, &objStore)
	if minioResource != nil {
		resourceMgr.Register(minioResource)
	}

	kafkaResource := initEventBus(cfg, logger, &eventPublisher)
	if kafkaResource != nil {
		resourceMgr.Register(kafkaResource)
	}

	// Create EventBus wrapper
	var eventBus mq.EventBus
	if eventPublisher != nil {
		eventBus = mq.NewKafkaEventBus(eventPublisher, mq.EventBusConfig{
			TopicDocumentsUploaded: cfg.Messaging.Kafka.TopicDocumentsUploaded,
			TopicDocumentsIngested:  cfg.Messaging.Kafka.TopicDocumentsIngested,
			TopicDocumentsFailed:     cfg.Messaging.Kafka.TopicDocumentsFailed,
		})
		logger.Info("initialized EventBus", zap.String("topic_uploaded", cfg.Messaging.Kafka.TopicDocumentsUploaded))
	}

	return &InfrastructureComponents{
		VectorStore:    vectorStore,
		ESResource:     nil, // Will be set in business components
		MinIOResource:  minioResource,
		KafkaResource:  kafkaResource,
		ObjectStore:    objStore,
		EventPublisher: eventPublisher,
		EventBus:       eventBus,
	}, nil
}

// initBusinessComponents initializes business logic components (Parser, Chunker, Embedder, LLM).
func initBusinessComponents(
	cfg *config.Config,
	logger *zap.Logger,
	infra *InfrastructureComponents,
) (*BusinessComponents, error) {
	// Initialize parser registry
	parserRegistry := ingestion.NewParserRegistry()
	parserRegistry.Register(ingestion.NewMarkdownParser())
	parserRegistry.Register(ingestion.NewTextParser())
	parserRegistry.Register(ingestion.NewPDFParser())

	// Register Tika parser if configured
	// Tika parser supports many formats (Word, Excel, PowerPoint, HTML, XML, RTF, ODT, etc.)
	if cfg.Ingestion.Tika.Enabled {
		tikaParser := ingestion.NewTikaParser(ingestion.TikaConfig{
			BaseURL: cfg.Ingestion.Tika.BaseURL,
			Timeout: cfg.Ingestion.Tika.Timeout,
		})
		parserRegistry.Register(tikaParser)
		logger.Info("registered Tika parser", zap.String("base_url", cfg.Ingestion.Tika.BaseURL))
	}

	// Initialize embedding cache and registry
	_, defaultEmbedder, err := initEmbedders(cfg, logger)
	if err != nil {
		logger.Warn("failed to initialize embedders", zap.Error(err))
	}

	// Initialize LLM registry
	llmRegistry, err := initLLMs(cfg, logger)
	if err != nil {
		logger.Warn("failed to initialize LLMs", zap.Error(err))
	}

	// Initialize chunker
	chunker := chunking.NewSemanticChunker(chunking.ChunkerConfig{
		MaxChunkSize:   cfg.Chunking.MaxSize,
		MinChunkSize:   cfg.Chunking.MinSize,
		Overlap:        cfg.Chunking.Overlap,
		RespectBounds:  cfg.Chunking.RespectBounds,
		SplitByHeading: true,
	})

	// Initialize retriever (with embedder and LLM)
	retrieverInstance, esResource, err := initRetriever(cfg, logger, infra.VectorStore, defaultEmbedder, llmRegistry)
	if err != nil {
		logger.Warn("failed to initialize retriever", zap.Error(err))
	}
	// Update ES resource if created
	if esResource != nil {
		infra.ESResource = esResource
	}

	// Initialize prompt builder
	tokenizer := prompt.NewSimpleTokenizer()
	promptBuilder := prompt.NewTemplateBuilder(tokenizer)

	return &BusinessComponents{
		ParserRegistry: parserRegistry,
		Chunker:        chunker,
		Embedder:       defaultEmbedder,
		LLMRegistry:    llmRegistry,
		Retriever:      retrieverInstance,
		PromptBuilder:  promptBuilder,
	}, nil
}

// initServiceLayer initializes service layer components.
func initServiceLayer(
	cfg *config.Config,
	logger *zap.Logger,
	infra *InfrastructureComponents,
	business *BusinessComponents,
	resourceMgr *ResourceManager,
) (*ServiceComponents, error) {
	// Initialize job store
	jobStore, err := initJobStore(cfg, infra.VectorStore)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize job store: %w", err)
	}

	// Initialize job queue
	var esClientForJob *search.ElasticsearchClient
	if infra.ESResource != nil {
		esClientForJob = infra.ESResource.Client()
	}
	jobQueue, err := initJobQueue(
		cfg, logger, jobStore,
		business.ParserRegistry,
		business.Chunker,
		business.Embedder,
		infra.VectorStore,
		infra.ObjectStore,
		infra.EventPublisher,
		esClientForJob,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize job queue: %w", err)
	}
	resourceMgr.RegisterShutdownHook(func(ctx context.Context) error {
		jobQueue.Stop()
		return nil
	})

	// Initialize metrics (will be used in handler)
	_ = metrics.NewMetrics()
	logger.Info("initialized Prometheus metrics")

	// Initialize query cache
	queryCache := cache.NewMemoryCache(cache.MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 1000,
	})
	logger.Info("initialized query result cache", zap.String("type", "memory"))

	// Initialize conversation store and repository
	convStore := conversation.NewMemoryStore()
	convRepo := repository.NewStoreConversationRepository(convStore)

	// Initialize services
	ingestionService := service.NewIngestionService(jobQueue, logger)
	searchService := service.NewSearchService(business.Retriever, business.PromptBuilder, business.LLMRegistry, queryCache, logger)
	conversationService := service.NewConversationService(convRepo, business.Retriever, business.PromptBuilder, business.LLMRegistry, logger)

	// Initialize ConsumerRegistry if Kafka is enabled
	var consumerRegistry *mq.ConsumerRegistry
	if cfg.Messaging.Enabled && cfg.Messaging.Kafka.Enabled {
		consumerRegistry = mq.NewConsumerRegistry(logger)

		// Create TaskProcessor for document ingestion
		taskProcessor := processor.NewDefaultTaskProcessor(
			business.ParserRegistry,
			business.Chunker,
			business.Embedder,
			infra.VectorStore,
			infra.ObjectStore,
			infra.EventPublisher,
			esClientForJob,
			cfg,
		)

		// Register document ingestion consumer
		if cfg.Messaging.Kafka.TopicDocumentsIngested != "" {
			consumerRegistry.Register(mq.ConsumerConfig{
				Brokers:    cfg.Messaging.Kafka.Brokers,
				Topic:      cfg.Messaging.Kafka.TopicDocumentsIngested,
				GroupID:    "rag-document-ingestion-group",
				Handler:    mq.DocumentIngestHandler(taskProcessor, logger),
				AutoCommit: false, // Manual commit for better control
			})
			logger.Info("registered document ingestion consumer",
				zap.String("topic", cfg.Messaging.Kafka.TopicDocumentsIngested),
			)
		}

		// Register document uploaded consumer (optional, for notifications)
		if cfg.Messaging.Kafka.TopicDocumentsUploaded != "" {
			consumerRegistry.Register(mq.ConsumerConfig{
				Brokers:    cfg.Messaging.Kafka.Brokers,
				Topic:      cfg.Messaging.Kafka.TopicDocumentsUploaded,
				GroupID:    "rag-document-uploaded-group",
				Handler:    mq.DocumentUploadedHandler(logger),
				AutoCommit: true, // Auto-commit for notification events
			})
			logger.Info("registered document uploaded consumer",
				zap.String("topic", cfg.Messaging.Kafka.TopicDocumentsUploaded),
			)
		}

		// Register document failed consumer (optional, for error handling)
		if cfg.Messaging.Kafka.TopicDocumentsFailed != "" {
			consumerRegistry.Register(mq.ConsumerConfig{
				Brokers:    cfg.Messaging.Kafka.Brokers,
				Topic:      cfg.Messaging.Kafka.TopicDocumentsFailed,
				GroupID:    "rag-document-failed-group",
				Handler:    mq.DocumentFailedHandler(logger),
				AutoCommit: true, // Auto-commit for notification events
			})
			logger.Info("registered document failed consumer",
				zap.String("topic", cfg.Messaging.Kafka.TopicDocumentsFailed),
			)
		}

		// Register ConsumerRegistry as a resource for graceful shutdown
		resourceMgr.Register(consumerRegistry)
	}

	return &ServiceComponents{
		IngestionService:    ingestionService,
		SearchService:       searchService,
		ConversationService: conversationService,
		ConsumerRegistry:    consumerRegistry,
	}, nil
}

// initAPILayer initializes API layer components (Handler, Router, HTTP Server, WebSocket).
func initAPILayer(
	cfg *config.Config,
	logger *zap.Logger,
	infra *InfrastructureComponents,
	business *BusinessComponents,
	services *ServiceComponents,
	resourceMgr *ResourceManager,
) (*APILayerComponents, error) {
	// Initialize WebSocket hub and handler
	wsHub := ws.NewHub(logger)
	wsHandler := ws.NewHandler(wsHub, services.SearchService, services.ConversationService, logger)
	// Register WebSocket hub as resource
	resourceMgr.Register(wsHub)

	// Create handler
	var esClientForHandler *search.ElasticsearchClient
	if infra.ESResource != nil {
		esClientForHandler = infra.ESResource.Client()
	}

	// Initialize metrics and cache (needed for handler)
	ragMetrics := metrics.NewMetrics()
	queryCache := cache.NewMemoryCache(cache.MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 1000,
	})
	// Reuse conversation store from service layer (created in initServiceLayer)
	// For handler compatibility, we still need convStore
	convStore := conversation.NewMemoryStore()

	// Initialize job store (needed for handler)
	jobStore, err := initJobStore(cfg, infra.VectorStore)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize job store: %w", err)
	}

	// Get job queue from services (we need to pass it to handler)
	// Note: This is a temporary solution until handler is refactored to use services
	var jobQueue job.Worker
	// We'll need to extract this from services or create it here
	// For now, we'll create it here to maintain compatibility
	jobQueue, err = initJobQueue(
		cfg, logger, jobStore,
		business.ParserRegistry,
		business.Chunker,
		business.Embedder,
		infra.VectorStore,
		infra.ObjectStore,
		infra.EventPublisher,
		esClientForHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize job queue: %w", err)
	}

	h := handler.New(handler.Config{
		// Service layer dependencies
		IngestionService:    services.IngestionService,
		SearchService:       services.SearchService,
		ConversationService: services.ConversationService,

		// Legacy dependencies (for backward compatibility)
		Config:         cfg,
		Logger:         logger,
		ParserRegistry: business.ParserRegistry,
		VectorStore:    infra.VectorStore,
		Embedder:       business.Embedder,
		Retriever:      business.Retriever,
		PromptBuilder:  business.PromptBuilder,
		LLMRegistry:    business.LLMRegistry,
		JobQueue:       jobQueue,
		JobStore:       jobStore,
		ConvStore:      convStore,
		Metrics:        ragMetrics,
		QueryCache:     queryCache,
		ObjectStore:    infra.ObjectStore,
		Events:         infra.EventPublisher,
		ESClient:       esClientForHandler,
	})

	// Initialize rate limiter
	limiter := initRateLimiter(cfg, logger)

	// Create router
	router := api.NewRouter(api.RouterConfig{
		Handler:   h,
		Logger:    logger,
		Limiter:   limiter,
		WSHandler: wsHandler,
	})

	// Create HTTP server
	httpServer := NewHTTPServer(
		cfg.Server.Port,
		router,
		cfg.Server.ReadTimeout,
		cfg.Server.WriteTimeout,
		logger,
	)

	return &APILayerComponents{
		Handler:    h,
		Router:     router,
		HTTPServer: httpServer,
		WSHub:      wsHub,
		WSHandler:  wsHandler,
	}, nil
}

