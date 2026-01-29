// Package config provides configuration management for the RAG system.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the RAG system.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Job       JobConfig       `mapstructure:"job"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Messaging MessagingConfig `mapstructure:"messaging"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	LLM       LLMConfig       `mapstructure:"llm"`
	Chunking  ChunkingConfig  `mapstructure:"chunking"`
	Retrieval RetrievalConfig `mapstructure:"retrieval"`
	Prompt    PromptConfig    `mapstructure:"prompt"`
	Ingestion IngestionConfig `mapstructure:"ingestion"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port         int               `mapstructure:"port"`
	ReadTimeout  time.Duration     `mapstructure:"read_timeout"`
	WriteTimeout time.Duration     `mapstructure:"write_timeout"`
	RateLimit    RateLimitConfig   `mapstructure:"rate_limit"`
	Performance  PerformanceConfig `mapstructure:"performance"`
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Rate    int    `mapstructure:"rate"`   // Requests per window
	Burst   int    `mapstructure:"burst"`  // Maximum burst
	Window  string `mapstructure:"window"` // Time window (e.g., "1s", "1m")
}

// PerformanceConfig holds performance monitoring configuration.
type PerformanceConfig struct {
	SlowQueryThreshold string `mapstructure:"slow_query_threshold"` // e.g., "1s", "500ms"
	EnableTracing      bool   `mapstructure:"enable_tracing"`       // Enable performance tracing
	LogSlowQueries     bool   `mapstructure:"log_slow_queries"`     // Log slow queries
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Path     string       `mapstructure:"path"`
	Provider string       `mapstructure:"provider"` // "sqlite", "qdrant", or "mysql+qdrant"
	MySQL    MySQLConfig  `mapstructure:"mysql"`
	Qdrant   QdrantConfig `mapstructure:"qdrant"`
}

// MySQLConfig holds MySQL configuration.
type MySQLConfig struct {
	DSN string `mapstructure:"dsn"` // MySQL DSN: "user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
}

// QdrantConfig holds Qdrant configuration.
type QdrantConfig struct {
	URL        string `mapstructure:"url"`
	Collection string `mapstructure:"collection"`
	APIKey     string `mapstructure:"api_key"`
}

// JobConfig holds async job processing configuration.
type JobConfig struct {
	Workers   int `mapstructure:"workers"`
	QueueSize int `mapstructure:"queue_size"`
}

// StorageConfig holds object storage configuration.
type StorageConfig struct {
	Enabled bool        `mapstructure:"enabled"`
	MinIO   MinIOConfig `mapstructure:"minio"`
}

// MinIOConfig holds MinIO configuration.
type MinIOConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	UseSSL          bool   `mapstructure:"use_ssl"`
	Bucket          string `mapstructure:"bucket"`
	Region          string `mapstructure:"region"`
	// If true, delete the uploaded object after successful ingestion.
	// Default: false (keep originals for audit / reprocessing)
	DeleteAfterIngest bool `mapstructure:"delete_after_ingest"`
}

// MessagingConfig holds messaging/event bus configuration.
type MessagingConfig struct {
	Enabled bool       `mapstructure:"enabled"`
	Kafka   KafkaConfig `mapstructure:"kafka"`
}

// KafkaConfig holds Kafka configuration.
type KafkaConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Brokers []string `mapstructure:"brokers"`
	// Topics used by this service (producer-side)
	TopicDocumentsUploaded  string `mapstructure:"topic_documents_uploaded"`
	TopicDocumentsIngested  string `mapstructure:"topic_documents_ingested"`
	TopicDocumentsFailed    string `mapstructure:"topic_documents_failed"`
}

// EmbeddingConfig holds embedding service configuration.
type EmbeddingConfig struct {
	Provider  string               `mapstructure:"provider"`
	BatchSize int                  `mapstructure:"batch_size"`
	Cache     EmbeddingCacheConfig `mapstructure:"cache"`
	OpenAI    OpenAIEmbedConfig    `mapstructure:"openai"`
	Ollama    OllamaEmbedConfig    `mapstructure:"ollama"`
}

// EmbeddingCacheConfig holds embedding cache configuration.
type EmbeddingCacheConfig struct {
	Enabled bool             `mapstructure:"enabled"`
	Type    string           `mapstructure:"type"` // "memory" or "redis"
	TTL     string           `mapstructure:"ttl"`  // e.g., "24h"
	MaxSize int              `mapstructure:"max_size"`
	Redis   RedisCacheConfig `mapstructure:"redis"`
}

// RedisCacheConfig holds Redis cache configuration.
type RedisCacheConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// OpenAIEmbedConfig holds OpenAI embedding configuration.
type OpenAIEmbedConfig struct {
	APIKey     string `mapstructure:"api_key"`
	Model      string `mapstructure:"model"`
	Dimensions int    `mapstructure:"dimensions"`
}

// OllamaEmbedConfig holds Ollama embedding configuration.
type OllamaEmbedConfig struct {
	BaseURL    string `mapstructure:"base_url"`
	Model      string `mapstructure:"model"`
	Dimensions int    `mapstructure:"dimensions"`
}

// LLMConfig holds LLM service configuration.
type LLMConfig struct {
	DefaultProvider string          `mapstructure:"default_provider"`
	OpenAI          OpenAILLMConfig `mapstructure:"openai"`
	Anthropic       AnthropicConfig `mapstructure:"anthropic"`
	Ollama          OllamaLLMConfig `mapstructure:"ollama"`
	Kimi            KimiLLMConfig   `mapstructure:"kimi"`
}

// OpenAILLMConfig holds OpenAI LLM configuration.
type OpenAILLMConfig struct {
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float32 `mapstructure:"temperature"`
}

// AnthropicConfig holds Anthropic LLM configuration.
type AnthropicConfig struct {
	APIKey    string `mapstructure:"api_key"`
	Model     string `mapstructure:"model"`
	MaxTokens int    `mapstructure:"max_tokens"`
}

// OllamaLLMConfig holds Ollama LLM configuration.
type OllamaLLMConfig struct {
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

// KimiLLMConfig holds Kimi (Moonshot AI) LLM configuration.
type KimiLLMConfig struct {
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float32 `mapstructure:"temperature"`
	BaseURL     string  `mapstructure:"base_url"`
}

// ChunkingConfig holds text chunking configuration.
type ChunkingConfig struct {
	MaxSize             int     `mapstructure:"max_size"`
	MinSize             int     `mapstructure:"min_size"`
	Overlap             int     `mapstructure:"overlap"`
	RespectBounds       bool    `mapstructure:"respect_bounds"`
	UseSemanticChunking bool    `mapstructure:"use_semantic_chunking"` // Use embedding-based semantic chunking
	SimilarityThreshold float32 `mapstructure:"similarity_threshold"`  // For semantic chunking
}

// RetrievalConfig holds retrieval configuration.
type RetrievalConfig struct {
	DefaultTopK        int                `mapstructure:"default_top_k"`
	CandidateK         int                `mapstructure:"candidate_k"`
	MinScore           float32            `mapstructure:"min_score"`
	EnableRerank       bool               `mapstructure:"enable_rerank"`
	EnableHybrid       bool               `mapstructure:"enable_hybrid"`
	EnableQueryRewrite bool               `mapstructure:"enable_query_rewrite"`
	EnableQueryExpand  bool               `mapstructure:"enable_query_expand"`
	UseLLMRewriter     bool               `mapstructure:"use_llm_rewriter"`
	MultiQueryCount    int                `mapstructure:"multi_query_count"`
	FusionMethod       string             `mapstructure:"fusion_method"` // "rrf", "avg", "max"
	RRFK               int                `mapstructure:"rrf_k"`         // RRF parameter
	Cohere             CohereRerankConfig `mapstructure:"cohere"`
	Elasticsearch      ElasticsearchConfig `mapstructure:"elasticsearch"`
}

// ElasticsearchConfig holds Elasticsearch configuration for full-text search.
type ElasticsearchConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	URLs    []string `mapstructure:"urls"`    // Elasticsearch URLs (e.g., ["http://localhost:9200"])
	Index   string   `mapstructure:"index"`   // Index name (default: "rag_chunks")
	Sniff   bool     `mapstructure:"sniff"`   // Enable node sniffing (default: false)
}

// CohereRerankConfig holds Cohere reranker configuration.
type CohereRerankConfig struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
	TopN   int    `mapstructure:"top_n"`
}

// PromptConfig holds prompt construction configuration.
type PromptConfig struct {
	MaxContextTokens int  `mapstructure:"max_context_tokens"`
	IncludeCitations bool `mapstructure:"include_citations"`
}

// IngestionConfig holds document ingestion configuration.
type IngestionConfig struct {
	Tika TikaConfig `mapstructure:"tika"`
}

// TikaConfig holds Apache Tika configuration.
type TikaConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// Load reads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// Set defaults
	setDefaults(v)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, use defaults and env vars
	}

	// Enable environment variable substitution
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand environment variables in sensitive fields
	cfg.Embedding.OpenAI.APIKey = expandEnv(cfg.Embedding.OpenAI.APIKey)
	cfg.LLM.OpenAI.APIKey = expandEnv(cfg.LLM.OpenAI.APIKey)
	cfg.LLM.Anthropic.APIKey = expandEnv(cfg.LLM.Anthropic.APIKey)
	cfg.LLM.Kimi.APIKey = expandEnv(cfg.LLM.Kimi.APIKey)
	cfg.Retrieval.Cohere.APIKey = expandEnv(cfg.Retrieval.Cohere.APIKey)
	cfg.Storage.MinIO.AccessKeyID = expandEnv(cfg.Storage.MinIO.AccessKeyID)
	cfg.Storage.MinIO.SecretAccessKey = expandEnv(cfg.Storage.MinIO.SecretAccessKey)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for configuration.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "60s")

	// Database defaults
	v.SetDefault("database.path", "./data/rag.db")
	v.SetDefault("database.provider", "sqlite")
	v.SetDefault("database.qdrant.url", "http://localhost:6333")
	v.SetDefault("database.qdrant.collection", "rag_chunks")

	// Job defaults
	v.SetDefault("job.workers", 3)
	v.SetDefault("job.queue_size", 100)
	v.SetDefault("job.use_redis", false)
	v.SetDefault("job.redis.addr", "localhost:6379")
	v.SetDefault("job.redis.db", 0)
	v.SetDefault("job.redis.concurrency", 10)
	v.SetDefault("job.redis.max_retries", 3)

	// Storage defaults
	v.SetDefault("storage.enabled", false)
	v.SetDefault("storage.minio.endpoint", "localhost:9000")
	v.SetDefault("storage.minio.access_key_id", "minioadmin")
	v.SetDefault("storage.minio.secret_access_key", "minioadmin")
	v.SetDefault("storage.minio.use_ssl", false)
	v.SetDefault("storage.minio.bucket", "rag-documents")
	v.SetDefault("storage.minio.region", "")
	v.SetDefault("storage.minio.delete_after_ingest", false)

	// Messaging defaults
	v.SetDefault("messaging.enabled", false)
	v.SetDefault("messaging.kafka.enabled", false)
	v.SetDefault("messaging.kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("messaging.kafka.topic_documents_uploaded", "rag.documents.uploaded")
	v.SetDefault("messaging.kafka.topic_documents_ingested", "rag.documents.ingested")
	v.SetDefault("messaging.kafka.topic_documents_failed", "rag.documents.failed")

	// Retrieval defaults
	v.SetDefault("retrieval.elasticsearch.enabled", false)
	v.SetDefault("retrieval.elasticsearch.urls", []string{"http://localhost:9200"})
	v.SetDefault("retrieval.elasticsearch.index", "rag_chunks")
	v.SetDefault("retrieval.elasticsearch.sniff", false)

	// Embedding defaults
	v.SetDefault("embedding.provider", "openai")
	v.SetDefault("embedding.batch_size", 100)
	v.SetDefault("embedding.cache.enabled", false)
	v.SetDefault("embedding.cache.type", "memory")
	v.SetDefault("embedding.cache.ttl", "24h")
	v.SetDefault("embedding.cache.max_size", 10000)
	v.SetDefault("embedding.cache.redis.addr", "localhost:6379")
	v.SetDefault("embedding.cache.redis.db", 0)
	v.SetDefault("embedding.openai.model", "text-embedding-3-small")
	v.SetDefault("embedding.openai.dimensions", 1536)
	v.SetDefault("embedding.ollama.base_url", "http://localhost:11434")
	v.SetDefault("embedding.ollama.model", "nomic-embed-text")
	v.SetDefault("embedding.ollama.dimensions", 768)

	// LLM defaults
	v.SetDefault("llm.default_provider", "openai")
	v.SetDefault("llm.openai.model", "gpt-4")
	v.SetDefault("llm.openai.max_tokens", 2048)
	v.SetDefault("llm.openai.temperature", 0.7)
	v.SetDefault("llm.anthropic.model", "claude-3-sonnet-20240229")
	v.SetDefault("llm.anthropic.max_tokens", 2048)
	v.SetDefault("llm.ollama.base_url", "http://localhost:11434")
	v.SetDefault("llm.ollama.model", "llama2")
	v.SetDefault("llm.kimi.model", "moonshot-v1-8k")
	v.SetDefault("llm.kimi.max_tokens", 4096)
	v.SetDefault("llm.kimi.temperature", 0.7)
	v.SetDefault("llm.kimi.base_url", "https://api.moonshot.cn/v1")

	// Chunking defaults
	v.SetDefault("chunking.max_size", 1000)
	v.SetDefault("chunking.min_size", 100)
	v.SetDefault("chunking.overlap", 100)
	v.SetDefault("chunking.respect_bounds", true)
	v.SetDefault("chunking.use_semantic_chunking", false)
	v.SetDefault("chunking.similarity_threshold", 0.7)

	// Retrieval defaults
	v.SetDefault("retrieval.default_top_k", 5)
	v.SetDefault("retrieval.candidate_k", 20)
	v.SetDefault("retrieval.min_score", 0.7)
	v.SetDefault("retrieval.enable_rerank", false)
	v.SetDefault("retrieval.enable_hybrid", false)
	v.SetDefault("retrieval.enable_query_rewrite", false)
	v.SetDefault("retrieval.enable_query_expand", false)
	v.SetDefault("retrieval.use_llm_rewriter", false)
	v.SetDefault("retrieval.multi_query_count", 0)
	v.SetDefault("retrieval.fusion_method", "rrf")
	v.SetDefault("retrieval.rrf_k", 60)
	v.SetDefault("retrieval.cohere.model", "rerank-multilingual-v3.0")
	v.SetDefault("retrieval.cohere.top_n", 10)

	// Prompt defaults
	v.SetDefault("prompt.max_context_tokens", 3000)
	v.SetDefault("prompt.include_citations", true)
}

// expandEnv expands environment variables in a string.
func expandEnv(s string) string {
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(s, "${"), "}")
		return os.Getenv(envVar)
	}
	return s
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}

	if c.Job.Workers <= 0 {
		return fmt.Errorf("job workers must be positive")
	}

	if c.Chunking.MaxSize <= 0 {
		return fmt.Errorf("chunking max_size must be positive")
	}

	if c.Chunking.MinSize < 0 {
		return fmt.Errorf("chunking min_size cannot be negative")
	}

	if c.Chunking.Overlap < 0 {
		return fmt.Errorf("chunking overlap cannot be negative")
	}

	if c.Retrieval.DefaultTopK <= 0 {
		return fmt.Errorf("retrieval default_top_k must be positive")
	}

	if c.Prompt.MaxContextTokens <= 0 {
		return fmt.Errorf("prompt max_context_tokens must be positive")
	}

	if c.Storage.Enabled {
		if c.Storage.MinIO.Endpoint == "" {
			return fmt.Errorf("storage.minio.endpoint is required when storage is enabled")
		}
		if c.Storage.MinIO.Bucket == "" {
			return fmt.Errorf("storage.minio.bucket is required when storage is enabled")
		}
	}

	if c.Messaging.Enabled && c.Messaging.Kafka.Enabled {
		if len(c.Messaging.Kafka.Brokers) == 0 {
			return fmt.Errorf("messaging.kafka.brokers is required when kafka is enabled")
		}
	}

	return nil
}
