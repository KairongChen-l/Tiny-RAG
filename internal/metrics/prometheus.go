package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the RAG system.
type Metrics struct {
	// Retrieval metrics
	RetrievalRequests prometheus.Counter
	RetrievalDuration prometheus.Histogram
	RetrievalResults  prometheus.Histogram
	RetrievalErrors   prometheus.Counter

	// Embedding metrics
	EmbeddingRequests    prometheus.Counter
	EmbeddingDuration    prometheus.Histogram
	EmbeddingCacheHits   prometheus.Counter
	EmbeddingCacheMisses prometheus.Counter
	EmbeddingErrors      prometheus.Counter

	// LLM metrics
	LLMRequests   prometheus.Counter
	LLMDuration   prometheus.Histogram
	LLMTokensUsed prometheus.Counter
	LLMErrors     prometheus.Counter

	// Job metrics
	JobSubmitted prometheus.Counter
	JobCompleted prometheus.Counter
	JobFailed    prometheus.Counter
	JobDuration  prometheus.Histogram

	// Vector store metrics
	VectorStoreOperations prometheus.Counter
	VectorStoreDuration   prometheus.Histogram
	VectorStoreErrors     prometheus.Counter
}

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		// Retrieval metrics
		RetrievalRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_retrieval_requests_total",
			Help: "Total number of retrieval requests",
		}),
		RetrievalDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_retrieval_duration_seconds",
			Help:    "Retrieval request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		RetrievalResults: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_retrieval_results_count",
			Help:    "Number of results returned per retrieval",
			Buckets: []float64{0, 1, 2, 3, 4, 5, 10, 20, 50},
		}),
		RetrievalErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_retrieval_errors_total",
			Help: "Total number of retrieval errors",
		}),

		// Embedding metrics
		EmbeddingRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_embedding_requests_total",
			Help: "Total number of embedding requests",
		}),
		EmbeddingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_embedding_duration_seconds",
			Help:    "Embedding request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		}),
		EmbeddingCacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_embedding_cache_hits_total",
			Help: "Total number of embedding cache hits",
		}),
		EmbeddingCacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_embedding_cache_misses_total",
			Help: "Total number of embedding cache misses",
		}),
		EmbeddingErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_embedding_errors_total",
			Help: "Total number of embedding errors",
		}),

		// LLM metrics
		LLMRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_llm_requests_total",
			Help: "Total number of LLM requests",
		}),
		LLMDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_llm_duration_seconds",
			Help:    "LLM request duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		}),
		LLMTokensUsed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_llm_tokens_used_total",
			Help: "Total number of tokens used by LLM",
		}),
		LLMErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_llm_errors_total",
			Help: "Total number of LLM errors",
		}),

		// Job metrics
		JobSubmitted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_job_submitted_total",
			Help: "Total number of jobs submitted",
		}),
		JobCompleted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_job_completed_total",
			Help: "Total number of jobs completed",
		}),
		JobFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_job_failed_total",
			Help: "Total number of jobs failed",
		}),
		JobDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_job_duration_seconds",
			Help:    "Job processing duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 300, 600},
		}),

		// Vector store metrics
		VectorStoreOperations: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_vector_store_operations_total",
			Help: "Total number of vector store operations",
		}),
		VectorStoreDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_vector_store_duration_seconds",
			Help:    "Vector store operation duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		}),
		VectorStoreErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_vector_store_errors_total",
			Help: "Total number of vector store errors",
		}),
	}
}
