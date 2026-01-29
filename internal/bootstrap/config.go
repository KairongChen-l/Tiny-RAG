// Package bootstrap provides configuration validation and loading utilities.
package bootstrap

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/krc/rag/pkg/config"
)

// ValidateConfig validates the application configuration.
// This function performs comprehensive validation including:
// - Required fields
// - Port ranges
// - Timeout values
// - Service dependencies
func ValidateConfig(cfg *config.Config, logger *zap.Logger) error {
	// Validate server configuration
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", cfg.Server.Port)
	}

	if cfg.Server.ReadTimeout < 0 {
		return fmt.Errorf("server.read_timeout must be non-negative")
	}

	if cfg.Server.WriteTimeout < 0 {
		return fmt.Errorf("server.write_timeout must be non-negative")
	}

	// Validate database configuration
	if cfg.Database.Provider == "" {
		return fmt.Errorf("database.provider is required")
	}

	if cfg.Database.Provider == "sqlite" || cfg.Database.Provider == "qdrant" {
		if cfg.Database.Path == "" {
			return fmt.Errorf("database.path is required for provider %s", cfg.Database.Provider)
		}
	}

	if cfg.Database.Provider == "mysql+qdrant" || cfg.Database.Provider == "hybrid" {
		if cfg.Database.MySQL.DSN == "" {
			return fmt.Errorf("database.mysql.dsn is required for provider %s", cfg.Database.Provider)
		}
	}

	// Validate embedding configuration
	if cfg.Embedding.Provider == "" {
		return fmt.Errorf("embedding.provider is required")
	}

	if cfg.Embedding.Provider == "openai" && cfg.Embedding.OpenAI.APIKey == "" {
		return fmt.Errorf("embedding.openai.api_key is required when provider is openai")
	}

	if cfg.Embedding.OpenAI.Dimensions <= 0 {
		return fmt.Errorf("embedding.openai.dimensions must be positive")
	}

	if cfg.Embedding.Ollama.Dimensions <= 0 {
		return fmt.Errorf("embedding.ollama.dimensions must be positive")
	}

	// Validate LLM configuration
	if cfg.LLM.DefaultProvider == "" {
		logger.Warn("llm.default_provider is not set, some features may not work")
	}

	// Validate chunking configuration
	if cfg.Chunking.MaxSize <= 0 {
		return fmt.Errorf("chunking.max_size must be positive")
	}

	if cfg.Chunking.MinSize <= 0 {
		return fmt.Errorf("chunking.min_size must be positive")
	}

	if cfg.Chunking.MinSize > cfg.Chunking.MaxSize {
		return fmt.Errorf("chunking.min_size (%d) must be <= max_size (%d)", cfg.Chunking.MinSize, cfg.Chunking.MaxSize)
	}

	if cfg.Chunking.Overlap < 0 {
		return fmt.Errorf("chunking.overlap must be non-negative")
	}

	// Validate job configuration
	if cfg.Job.Workers <= 0 {
		return fmt.Errorf("job.workers must be positive")
	}

	if cfg.Job.QueueSize <= 0 {
		return fmt.Errorf("job.queue_size must be positive")
	}

	// Validate storage configuration
	if cfg.Storage.Enabled {
		if cfg.Storage.MinIO.Endpoint == "" {
			return fmt.Errorf("storage.minio.endpoint is required when storage is enabled")
		}
		if cfg.Storage.MinIO.Bucket == "" {
			return fmt.Errorf("storage.minio.bucket is required when storage is enabled")
		}
		if cfg.Storage.MinIO.AccessKeyID == "" {
			return fmt.Errorf("storage.minio.access_key_id is required when storage is enabled")
		}
		if cfg.Storage.MinIO.SecretAccessKey == "" {
			return fmt.Errorf("storage.minio.secret_access_key is required when storage is enabled")
		}
	}

	// Validate messaging configuration
	if cfg.Messaging.Enabled && cfg.Messaging.Kafka.Enabled {
		if len(cfg.Messaging.Kafka.Brokers) == 0 {
			return fmt.Errorf("messaging.kafka.brokers is required when kafka is enabled")
		}
		if cfg.Messaging.Kafka.TopicDocumentsUploaded == "" {
			logger.Warn("messaging.kafka.topic_documents_uploaded is not set")
		}
		if cfg.Messaging.Kafka.TopicDocumentsIngested == "" {
			logger.Warn("messaging.kafka.topic_documents_ingested is not set")
		}
	}

	// Validate retrieval configuration
	if cfg.Retrieval.EnableHybrid && cfg.Retrieval.Elasticsearch.Enabled {
		if len(cfg.Retrieval.Elasticsearch.URLs) == 0 {
			return fmt.Errorf("retrieval.elasticsearch.urls is required when elasticsearch is enabled")
		}
		if cfg.Retrieval.Elasticsearch.Index == "" {
			return fmt.Errorf("retrieval.elasticsearch.index is required when elasticsearch is enabled")
		}
	}

	// Validate prompt configuration
	if cfg.Prompt.MaxContextTokens <= 0 {
		return fmt.Errorf("prompt.max_context_tokens must be positive")
	}

	// Note: Timeout validation removed as it's not in the config struct

	return nil
}

