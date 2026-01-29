package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/api"
	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/common"
	"github.com/krc/rag/internal/embedding"
	"github.com/krc/rag/internal/generation"
	genAnthropic "github.com/krc/rag/internal/generation/anthropic"
	genKimi "github.com/krc/rag/internal/generation/kimi"
	genOllama "github.com/krc/rag/internal/generation/ollama"
	genOpenAI "github.com/krc/rag/internal/generation/openai"
	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/index/hybrid"
	"github.com/krc/rag/internal/index/mysql"
	"github.com/krc/rag/internal/index/qdrant"
	"github.com/krc/rag/internal/index/sqlite"
	"github.com/krc/rag/internal/ingestion"
	"github.com/krc/rag/internal/job"
	jobpayloads "github.com/krc/rag/internal/job/payloads"
	"github.com/krc/rag/internal/processor"
	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/retrieval"
	rerankerCohere "github.com/krc/rag/internal/retrieval/reranker"
	"github.com/krc/rag/pkg/config"
	"github.com/krc/rag/pkg/messaging"
	"github.com/krc/rag/pkg/performance"
	"github.com/krc/rag/pkg/ratelimit"
	"github.com/krc/rag/pkg/search"
	"github.com/krc/rag/pkg/storage"
	"encoding/json"
)

// App holds all application components.
type App struct {
	Router         *api.Router
	HTTPServer     *HTTPServer
	JobQueue       job.Worker
	VectorStore    index.VectorStore
	ResourceMgr    *ResourceManager
	logger         *zap.Logger
}

// Config holds bootstrap configuration.
type Config struct {
	Config *config.Config
	Logger *zap.Logger
}

// NewApp initializes all application components.
// This function orchestrates the initialization process by calling stage-specific functions.
func NewApp(cfg Config) (*App, error) {
	logger := cfg.Logger
	config := cfg.Config

	// Validate configuration
	if err := ValidateConfig(config, logger); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Create resource manager
	resourceMgr := NewResourceManager(logger)

	// Stage 1: Initialize infrastructure (DB, Redis, ES, Kafka, MinIO)
	infra, err := initInfrastructure(config, logger, resourceMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize infrastructure: %w", err)
	}

	// Register ES resource if created
	if infra.ESResource != nil {
		resourceMgr.Register(infra.ESResource)
	}

	// Stage 2: Initialize business components (Parser, Chunker, Embedder, LLM)
	business, err := initBusinessComponents(config, logger, infra)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize business components: %w", err)
	}

	// Register ES resource if created in business components
	if infra.ESResource != nil {
		resourceMgr.Register(infra.ESResource)
	}

	// Stage 3: Initialize service layer (Service, Repository)
	services, err := initServiceLayer(config, logger, infra, business, resourceMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service layer: %w", err)
	}

	// Stage 4: Initialize API layer (Handler, Router, HTTP Server, WebSocket)
	apiLayer, err := initAPILayer(config, logger, infra, business, services, resourceMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize API layer: %w", err)
	}

	// Extract job queue from service layer (temporary until handler refactored)
	// Note: This is duplicated in initAPILayer, will be fixed in Phase 2
	jobStore, err := initJobStore(config, infra.VectorStore)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize job store: %w", err)
	}
	var esClientForJob *search.ElasticsearchClient
	if infra.ESResource != nil {
		esClientForJob = infra.ESResource.Client()
	}
	jobQueue, err := initJobQueue(
		config, logger, jobStore,
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

	// Start ConsumerRegistry if enabled
	if services.ConsumerRegistry != nil {
		appCtx := context.Background()
		if err := services.ConsumerRegistry.Start(appCtx); err != nil {
			logger.Warn("failed to start consumer registry", zap.Error(err))
		} else {
			logger.Info("consumer registry started")
		}
	}

	return &App{
		Router:      apiLayer.Router,
		HTTPServer:  apiLayer.HTTPServer,
		JobQueue:    jobQueue,
		VectorStore: infra.VectorStore,
		ResourceMgr: resourceMgr,
		logger:      logger,
	}, nil
}

// Close cleans up all application resources.
func (a *App) Close(ctx context.Context) error {
	// Shutdown HTTP server first
	if a.HTTPServer != nil {
		if err := a.HTTPServer.Shutdown(ctx); err != nil {
			a.logger.Warn("HTTP server shutdown error", zap.Error(err))
		}
	}

	// Close all resources
	if a.ResourceMgr != nil {
		return a.ResourceMgr.Close(ctx)
	}

	return nil
}

// maskDSN masks password in DSN for logging.
func maskDSN(dsn string) string {
	if strings.Contains(dsn, "@") {
		parts := strings.Split(dsn, "@")
		if len(parts) > 0 {
			userPass := parts[0]
			if strings.Contains(userPass, ":") {
				userParts := strings.Split(userPass, ":")
				if len(userParts) >= 2 {
					userParts[1] = "***"
					parts[0] = strings.Join(userParts, ":")
				}
			}
			return strings.Join(parts, "@")
		}
	}
	return dsn
}

// initVectorStore initializes the vector store based on provider.
func initVectorStore(cfg *config.Config, logger *zap.Logger) (index.VectorStore, error) {
	dimensions := cfg.Embedding.OpenAI.Dimensions
	if cfg.Embedding.Provider == "ollama" {
		dimensions = cfg.Embedding.Ollama.Dimensions
	}

	switch cfg.Database.Provider {
	case "mysql+qdrant", "hybrid":
		if cfg.Database.MySQL.DSN == "" {
			return nil, fmt.Errorf("MySQL DSN is required for hybrid provider")
		}
		if cfg.Database.Qdrant.URL == "" {
			cfg.Database.Qdrant.URL = "http://localhost:6333"
		}
		if cfg.Database.Qdrant.Collection == "" {
			cfg.Database.Qdrant.Collection = "rag_chunks"
		}
		store, err := hybrid.New(hybrid.Config{
			MySQLDSN:   cfg.Database.MySQL.DSN,
			QdrantURL:  cfg.Database.Qdrant.URL,
			Collection: cfg.Database.Qdrant.Collection,
			Dimension:  dimensions,
			QdrantKey:  cfg.Database.Qdrant.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize hybrid store: %w", err)
		}
		logger.Info("initialized hybrid store (MySQL + Qdrant)",
			zap.String("mysql_dsn", maskDSN(cfg.Database.MySQL.DSN)),
			zap.String("qdrant_url", cfg.Database.Qdrant.URL),
			zap.String("collection", cfg.Database.Qdrant.Collection),
		)
		return store, nil
	case "qdrant":
		store, err := qdrant.New(qdrant.Config{
			URL:        cfg.Database.Qdrant.URL,
			Collection: cfg.Database.Qdrant.Collection,
			Dimension:  dimensions,
			APIKey:     cfg.Database.Qdrant.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Qdrant vector store: %w", err)
		}
		logger.Info("initialized Qdrant vector store",
			zap.String("url", cfg.Database.Qdrant.URL),
			zap.String("collection", cfg.Database.Qdrant.Collection),
		)
		return store, nil
	default: // sqlite
		var slowQueryLogger *performance.SlowQueryLogger
		if cfg.Server.Performance.LogSlowQueries {
			threshold := 1 * time.Second
			if cfg.Server.Performance.SlowQueryThreshold != "" {
				if parsed, err := time.ParseDuration(cfg.Server.Performance.SlowQueryThreshold); err == nil {
					threshold = parsed
				}
			}
			slowQueryLogger = performance.NewSlowQueryLogger(logger, threshold)
			logger.Info("slow query logging enabled", zap.Duration("threshold", threshold))
		}

		store, err := sqlite.New(sqlite.Config{
			Path:            cfg.Database.Path,
			Dimension:       dimensions,
			SlowQueryLogger: slowQueryLogger,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize SQLite vector store: %w", err)
		}
		logger.Info("initialized SQLite vector store", zap.String("path", cfg.Database.Path))
		return store, nil
	}
}

// initEmbedders initializes embedding cache and registry.
func initEmbedders(cfg *config.Config, logger *zap.Logger) (*embedding.EmbedderRegistry, embedding.Embedder, error) {
	var embeddingCache embedding.Cache
	if cfg.Embedding.Cache.Enabled {
		switch cfg.Embedding.Cache.Type {
		case "redis":
			ttl, err := time.ParseDuration(cfg.Embedding.Cache.TTL)
			if err != nil {
				ttl = 24 * time.Hour
			}
			redisCache, err := embedding.NewRedisCache(embedding.RedisCacheConfig{
				Addr:     cfg.Embedding.Cache.Redis.Addr,
				Password: cfg.Embedding.Cache.Redis.Password,
				DB:       cfg.Embedding.Cache.Redis.DB,
				TTL:      ttl,
			})
			if err != nil {
				logger.Warn("failed to initialize Redis cache, falling back to memory", zap.Error(err))
				ttl, _ := time.ParseDuration(cfg.Embedding.Cache.TTL)
				if ttl == 0 {
					ttl = 24 * time.Hour
				}
				embeddingCache = embedding.NewMemoryCache(embedding.MemoryCacheConfig{
					TTL:     ttl,
					MaxSize: cfg.Embedding.Cache.MaxSize,
				})
			} else {
				embeddingCache = redisCache
				logger.Info("initialized Redis embedding cache", zap.String("addr", cfg.Embedding.Cache.Redis.Addr))
			}
		default: // memory
			ttl, err := time.ParseDuration(cfg.Embedding.Cache.TTL)
			if err != nil {
				ttl = 24 * time.Hour
			}
			embeddingCache = embedding.NewMemoryCache(embedding.MemoryCacheConfig{
				TTL:     ttl,
				MaxSize: cfg.Embedding.Cache.MaxSize,
			})
			logger.Info("initialized memory embedding cache")
		}
	}

	embedderRegistry := embedding.NewEmbedderRegistry()

	if cfg.Embedding.OpenAI.APIKey != "" {
		openaiEmbedder, err := embedding.NewOpenAIEmbedder(embedding.OpenAIConfig{
			APIKey:     cfg.Embedding.OpenAI.APIKey,
			Model:      cfg.Embedding.OpenAI.Model,
			Dimensions: cfg.Embedding.OpenAI.Dimensions,
			BatchSize:  cfg.Embedding.BatchSize,
		})
		if err != nil {
			logger.Warn("failed to initialize OpenAI embedder", zap.Error(err))
		} else {
			var finalEmbedder embedding.Embedder = openaiEmbedder
			if embeddingCache != nil {
				finalEmbedder = embedding.NewCachedEmbedder(openaiEmbedder, embeddingCache)
			}
			embedderRegistry.Register("openai", finalEmbedder)
		}
	}

	ollamaEmbedder, err := embedding.NewOllamaEmbedder(embedding.OllamaConfig{
		BaseURL:    cfg.Embedding.Ollama.BaseURL,
		Model:      cfg.Embedding.Ollama.Model,
		Dimensions: cfg.Embedding.Ollama.Dimensions,
	})
	if err == nil {
		var finalEmbedder embedding.Embedder = ollamaEmbedder
		if embeddingCache != nil {
			finalEmbedder = embedding.NewCachedEmbedder(ollamaEmbedder, embeddingCache)
		}
		embedderRegistry.Register("ollama", finalEmbedder)
	}

	if err := embedderRegistry.SetDefault(cfg.Embedding.Provider); err != nil {
		logger.Warn("failed to set default embedder", zap.Error(err))
	}

	defaultEmbedder, _ := embedderRegistry.GetDefault()
	return embedderRegistry, defaultEmbedder, nil
}

// initLLMs initializes LLM registry.
func initLLMs(cfg *config.Config, logger *zap.Logger) (*generation.LLMRegistry, error) {
	llmRegistry := generation.NewLLMRegistry()

	if cfg.LLM.OpenAI.APIKey != "" {
		openaiLLM, err := genOpenAI.New(genOpenAI.Config{
			APIKey:      cfg.LLM.OpenAI.APIKey,
			Model:       cfg.LLM.OpenAI.Model,
			MaxTokens:   cfg.LLM.OpenAI.MaxTokens,
			Temperature: cfg.LLM.OpenAI.Temperature,
		})
		if err != nil {
			logger.Warn("failed to initialize OpenAI LLM", zap.Error(err))
		} else {
			llmRegistry.Register("openai", openaiLLM)
		}
	}

	if cfg.LLM.Anthropic.APIKey != "" {
		anthropicLLM, err := genAnthropic.New(genAnthropic.Config{
			APIKey:    cfg.LLM.Anthropic.APIKey,
			Model:     cfg.LLM.Anthropic.Model,
			MaxTokens: cfg.LLM.Anthropic.MaxTokens,
		})
		if err != nil {
			logger.Warn("failed to initialize Anthropic LLM", zap.Error(err))
		} else {
			llmRegistry.Register("anthropic", anthropicLLM)
		}
	}

	ollamaLLM, err := genOllama.New(genOllama.Config{
		BaseURL: cfg.LLM.Ollama.BaseURL,
		Model:   cfg.LLM.Ollama.Model,
		Timeout: 300 * time.Second,
	})
	if err == nil {
		llmRegistry.Register("ollama", ollamaLLM)
	}

	if cfg.LLM.Kimi.APIKey != "" {
		kimiLLM, err := genKimi.New(genKimi.Config{
			APIKey:      cfg.LLM.Kimi.APIKey,
			Model:       cfg.LLM.Kimi.Model,
			MaxTokens:   cfg.LLM.Kimi.MaxTokens,
			Temperature: cfg.LLM.Kimi.Temperature,
			BaseURL:     cfg.LLM.Kimi.BaseURL,
		})
		if err != nil {
			logger.Warn("failed to initialize Kimi LLM", zap.Error(err))
		} else {
			llmRegistry.Register("kimi", kimiLLM)
			logger.Info("initialized Kimi LLM", zap.String("model", cfg.LLM.Kimi.Model))
		}
	}

	if err := llmRegistry.SetDefault(cfg.LLM.DefaultProvider); err != nil {
		logger.Warn("failed to set default LLM", zap.Error(err))
	}

	return llmRegistry, nil
}

// initRetriever initializes the retriever.
func initRetriever(cfg *config.Config, logger *zap.Logger, vectorStore index.VectorStore, defaultEmbedder embedding.Embedder, llmRegistry *generation.LLMRegistry) (retrieval.Retriever, *ElasticsearchResource, error) {
	if defaultEmbedder == nil {
		return nil, nil, fmt.Errorf("no embedder available")
	}

	var rerankerInstance retrieval.Reranker
	if cfg.Retrieval.EnableRerank && cfg.Retrieval.Cohere.APIKey != "" {
		rerankerInstance, _ = rerankerCohere.NewCohereReranker(rerankerCohere.Config{
			APIKey: cfg.Retrieval.Cohere.APIKey,
			Model:  cfg.Retrieval.Cohere.Model,
			TopN:   cfg.Retrieval.Cohere.TopN,
		})
		logger.Info("initialized Cohere reranker", zap.String("model", cfg.Retrieval.Cohere.Model))
	}

	var queryRewriter retrieval.QueryRewriter
	if cfg.Retrieval.EnableQueryRewrite {
		if cfg.Retrieval.UseLLMRewriter {
			defaultLLM, _ := llmRegistry.GetDefault()
			if defaultLLM != nil {
				llmAdapter := newLLMQueryAdapter(defaultLLM)
				llmRewriter, err := retrieval.NewLLMQueryRewriter(retrieval.LLMQueryRewriterConfig{
					LLM:             llmAdapter,
					EnableExpansion: cfg.Retrieval.EnableQueryExpand,
				})
				if err != nil {
					logger.Warn("failed to initialize LLM query rewriter, falling back to simple", zap.Error(err))
					queryRewriter = retrieval.NewSimpleQueryRewriter(cfg.Retrieval.EnableQueryExpand)
				} else {
					queryRewriter = llmRewriter
					logger.Info("initialized LLM query rewriter")
				}
			} else {
				queryRewriter = retrieval.NewSimpleQueryRewriter(cfg.Retrieval.EnableQueryExpand)
			}
		} else {
			queryRewriter = retrieval.NewSimpleQueryRewriter(cfg.Retrieval.EnableQueryExpand)
			logger.Info("initialized simple query rewriter")
		}
	}

	vectorRetriever, err := retrieval.NewVectorRetriever(retrieval.VectorRetrieverConfig{
		Store:         vectorStore,
		Embedder:      defaultEmbedder,
		Reranker:      rerankerInstance,
		QueryRewriter: queryRewriter,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize vector retriever: %w", err)
	}

	var retrieverInstance retrieval.Retriever
	var esResource *ElasticsearchResource

	if cfg.Retrieval.EnableHybrid {
		var bm25Retriever retrieval.BM25RetrieverInterface

		if cfg.Retrieval.Elasticsearch.Enabled && len(cfg.Retrieval.Elasticsearch.URLs) > 0 {
			esClient, err := search.NewElasticsearchClient(search.Config{
				URLs:   cfg.Retrieval.Elasticsearch.URLs,
				Index:  cfg.Retrieval.Elasticsearch.Index,
				Sniff:  cfg.Retrieval.Elasticsearch.Sniff,
				Logger: logger,
			})
			if err != nil {
				logger.Warn("failed to initialize Elasticsearch, falling back to memory BM25", zap.Error(err))
				bm25Retriever = retrieval.NewBM25Retriever(vectorStore)
			} else {
				bm25Retriever = retrieval.NewElasticsearchBM25Retriever(esClient)
				esResource = NewElasticsearchResource(esClient)
				logger.Info("initialized Elasticsearch BM25 retriever",
					zap.Strings("urls", cfg.Retrieval.Elasticsearch.URLs),
					zap.String("index", cfg.Retrieval.Elasticsearch.Index))
			}
		} else {
			bm25Retriever = retrieval.NewBM25Retriever(vectorStore)
			logger.Info("using memory BM25 retriever (Elasticsearch not enabled)")
		}

		fusionMethod := retrieval.FusionRRF
		switch cfg.Retrieval.FusionMethod {
		case "avg":
			fusionMethod = retrieval.FusionAvg
		case "max":
			fusionMethod = retrieval.FusionMax
		}

		hybridRetriever, err := retrieval.NewHybridRetriever(retrieval.HybridRetrieverConfig{
			VectorRetriever: vectorRetriever,
			BM25Retriever:   bm25Retriever,
			FusionMethod:    fusionMethod,
			K:               cfg.Retrieval.RRFK,
		})
		if err != nil {
			logger.Warn("failed to initialize hybrid retriever, falling back to vector", zap.Error(err))
			retrieverInstance = vectorRetriever
		} else {
			retrieverInstance = hybridRetriever
			logger.Info("initialized hybrid retriever", zap.String("fusion_method", cfg.Retrieval.FusionMethod))
		}
	} else {
		retrieverInstance = vectorRetriever
	}

	return retrieverInstance, esResource, nil
}

// initJobStore initializes the job store.
func initJobStore(cfg *config.Config, vectorStore index.VectorStore) (job.Store, error) {
	switch cfg.Database.Provider {
	case "mysql+qdrant", "hybrid":
		if hybridStore, ok := vectorStore.(*hybrid.Store); ok {
			gormDB := hybridStore.DB()
			if gormDB != nil {
				return mysql.NewJobStore(gormDB), nil
			}
			return nil, fmt.Errorf("failed to get MySQL DB from hybrid store")
		}
		return nil, fmt.Errorf("vector store is not a hybrid store")
	case "qdrant":
		jobDBPath := cfg.Database.Path + ".jobs"
		if jobDBPath == ".jobs" {
			jobDBPath = "./data/jobs.db"
		}
		jobDB, err := sql.Open("sqlite3", jobDBPath+"?_foreign_keys=on")
		if err != nil {
			return nil, fmt.Errorf("failed to open job database: %w", err)
		}
		return sqlite.NewJobStore(jobDB), nil
	default: // sqlite
		if sqliteStore, ok := vectorStore.(*sqlite.Store); ok {
			return sqlite.NewJobStore(sqliteStore.DB()), nil
		}
		return nil, fmt.Errorf("unsupported vector store type for job storage")
	}
}

// initObjectStore initializes object storage (MinIO).
func initObjectStore(cfg *config.Config, logger *zap.Logger, objStore *common.ObjectStore) *MinIOResource {
	if !cfg.Storage.Enabled {
		return nil
	}

	if cfg.Storage.MinIO.Endpoint == "" || cfg.Storage.MinIO.AccessKeyID == "" || cfg.Storage.MinIO.SecretAccessKey == "" || cfg.Storage.MinIO.Bucket == "" {
		logger.Warn("storage.enabled=true but MinIO config incomplete; disabling object storage")
		return nil
	}

	minioClient, err := storage.NewMinIOClient(storage.Config{
		Endpoint:        cfg.Storage.MinIO.Endpoint,
		AccessKeyID:     cfg.Storage.MinIO.AccessKeyID,
		SecretAccessKey: cfg.Storage.MinIO.SecretAccessKey,
		UseSSL:          cfg.Storage.MinIO.UseSSL,
		BucketName:      cfg.Storage.MinIO.Bucket,
		Region:          cfg.Storage.MinIO.Region,
	})
	if err != nil {
		logger.Warn("failed to initialize MinIO client; disabling object storage", zap.Error(err))
		return nil
	}

	logger.Info("initialized MinIO object storage",
		zap.String("endpoint", cfg.Storage.MinIO.Endpoint),
		zap.String("bucket", cfg.Storage.MinIO.Bucket),
	)

	resource := NewMinIOResource(minioClient)
	adapter := NewObjectStoreAdapter(minioClient)
	*objStore = adapter
	return resource
}

// initEventBus initializes event bus (Kafka).
func initEventBus(cfg *config.Config, logger *zap.Logger, events *common.EventPublisher) *KafkaProducerResource {
	if !cfg.Messaging.Enabled || !cfg.Messaging.Kafka.Enabled {
		return nil
	}

	if len(cfg.Messaging.Kafka.Brokers) == 0 {
		logger.Warn("messaging enabled but kafka.brokers empty; disabling kafka")
		return nil
	}

	prod, err := messaging.NewKafkaProducer(messaging.ProducerConfig{
		Brokers: cfg.Messaging.Kafka.Brokers,
		Logger:  logger,
	})
	if err != nil {
		logger.Warn("failed to initialize Kafka producer; disabling kafka", zap.Error(err))
		return nil
	}

	logger.Info("initialized Kafka producer", zap.Strings("brokers", cfg.Messaging.Kafka.Brokers))
	resource := NewKafkaProducerResource(prod)
	adapter := NewEventPublisherAdapter(prod)
	*events = adapter
	return resource
}

// initJobQueue initializes the job queue.
func initJobQueue(
	cfg *config.Config,
	logger *zap.Logger,
	jobStore job.Store,
	parserRegistry *ingestion.ParserRegistry,
	chunker chunking.Chunker,
	embedder embedding.Embedder,
	vectorStore index.VectorStore,
	objStore common.ObjectStore,
	events common.EventPublisher,
	esClient *search.ElasticsearchClient,
) (job.Worker, error) {
	jobQueue := job.NewQueue(
		job.QueueConfig{
			Workers:   cfg.Job.Workers,
			QueueSize: cfg.Job.QueueSize,
		},
		jobStore,
		createJobHandler(logger, parserRegistry, chunker, embedder, vectorStore, objStore, events, cfg, esClient),
		logger,
	)

	if err := jobQueue.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start job queue: %w", err)
	}

	return jobQueue, nil
}

// initRateLimiter initializes rate limiter if enabled.
func initRateLimiter(cfg *config.Config, logger *zap.Logger) *ratelimit.Limiter {
	if !cfg.Server.RateLimit.Enabled {
		return nil
	}

	window := time.Second
	if cfg.Server.RateLimit.Window != "" {
		if parsed, err := time.ParseDuration(cfg.Server.RateLimit.Window); err == nil {
			window = parsed
		} else {
			logger.Warn("invalid rate limit window, using default 1s", zap.Error(err))
		}
	}

	rate := cfg.Server.RateLimit.Rate
	if rate <= 0 {
		rate = 100
	}

	burst := cfg.Server.RateLimit.Burst
	if burst <= 0 {
		burst = rate
	}

	limiter := ratelimit.NewLimiter(ratelimit.Config{
		Rate:   rate,
		Burst:  burst,
		Window: window,
	})
	logger.Info("rate limiter initialized",
		zap.Int("rate", rate),
		zap.Int("burst", burst),
		zap.Duration("window", window),
	)
	return limiter
}

// llmQueryAdapter adapts generation.LLM to retrieval.LLMGenerator interface.
type llmQueryAdapter struct {
	llm generation.LLM
}

// newLLMQueryAdapter creates a new LLM query adapter.
func newLLMQueryAdapter(llm generation.LLM) *llmQueryAdapter {
	return &llmQueryAdapter{llm: llm}
}

// GenerateText generates text from a prompt string.
func (a *llmQueryAdapter) GenerateText(ctx context.Context, promptText string) (string, error) {
	p := &prompt.Prompt{
		SystemMessage: "",
		UserMessage:   promptText,
		TokenCount:    0,
	}

	response, err := a.llm.Generate(ctx, p)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	return response.Answer, nil
}


// createJobHandler creates the job processing handler.
func createJobHandler(
	logger *zap.Logger,
	parserRegistry *ingestion.ParserRegistry,
	chunker chunking.Chunker,
	embedder embedding.Embedder,
	vectorStore index.VectorStore,
	objStore common.ObjectStore,
	events common.EventPublisher,
	cfg *config.Config,
	esClient *search.ElasticsearchClient,
) job.Handler {
	// Create TaskProcessor
	processor := processor.NewDefaultTaskProcessor(
		parserRegistry,
		chunker,
		embedder,
		vectorStore,
		objStore,
		events,
		esClient,
		cfg,
	)

	return func(ctx context.Context, j *job.Job) error {
		switch j.Type {
		case job.TypeDocumentIngest:
			// Parse payload
			var payload jobpayloads.DocumentIngestPayload
			if err := json.Unmarshal(j.Payload, &payload); err != nil {
				return fmt.Errorf("failed to parse job payload: %w", err)
			}

			// Create progress callback
			progressCallback := func(progress int, stage string, message string) {
				j.SetProgressWithStage(progress, stage, message)
			}

			// Process using TaskProcessor
			return processor.Process(ctx, payload, progressCallback)
		default:
			return fmt.Errorf("unknown job type: %s", j.Type)
		}
	}
}

