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
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	LLM       LLMConfig       `mapstructure:"llm"`
	Chunking  ChunkingConfig  `mapstructure:"chunking"`
	Retrieval RetrievalConfig `mapstructure:"retrieval"`
	Prompt    PromptConfig    `mapstructure:"prompt"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// JobConfig holds async job processing configuration.
type JobConfig struct {
	Workers   int `mapstructure:"workers"`
	QueueSize int `mapstructure:"queue_size"`
}

// EmbeddingConfig holds embedding service configuration.
type EmbeddingConfig struct {
	Provider  string              `mapstructure:"provider"`
	BatchSize int                 `mapstructure:"batch_size"`
	OpenAI    OpenAIEmbedConfig   `mapstructure:"openai"`
	Ollama    OllamaEmbedConfig   `mapstructure:"ollama"`
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
	DefaultProvider string           `mapstructure:"default_provider"`
	OpenAI          OpenAILLMConfig  `mapstructure:"openai"`
	Anthropic       AnthropicConfig  `mapstructure:"anthropic"`
	Ollama          OllamaLLMConfig  `mapstructure:"ollama"`
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

// ChunkingConfig holds text chunking configuration.
type ChunkingConfig struct {
	MaxSize       int  `mapstructure:"max_size"`
	MinSize       int  `mapstructure:"min_size"`
	Overlap       int  `mapstructure:"overlap"`
	RespectBounds bool `mapstructure:"respect_bounds"`
}

// RetrievalConfig holds retrieval configuration.
type RetrievalConfig struct {
	DefaultTopK  int     `mapstructure:"default_top_k"`
	CandidateK   int     `mapstructure:"candidate_k"`
	MinScore     float32 `mapstructure:"min_score"`
	EnableRerank bool    `mapstructure:"enable_rerank"`
}

// PromptConfig holds prompt construction configuration.
type PromptConfig struct {
	MaxContextTokens int  `mapstructure:"max_context_tokens"`
	IncludeCitations bool `mapstructure:"include_citations"`
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

	// Job defaults
	v.SetDefault("job.workers", 3)
	v.SetDefault("job.queue_size", 100)

	// Embedding defaults
	v.SetDefault("embedding.provider", "openai")
	v.SetDefault("embedding.batch_size", 100)
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

	// Chunking defaults
	v.SetDefault("chunking.max_size", 1000)
	v.SetDefault("chunking.min_size", 100)
	v.SetDefault("chunking.overlap", 100)
	v.SetDefault("chunking.respect_bounds", true)

	// Retrieval defaults
	v.SetDefault("retrieval.default_top_k", 5)
	v.SetDefault("retrieval.candidate_k", 20)
	v.SetDefault("retrieval.min_score", 0.7)
	v.SetDefault("retrieval.enable_rerank", false)

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

	return nil
}

