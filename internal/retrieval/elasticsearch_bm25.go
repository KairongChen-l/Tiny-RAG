package retrieval

import (
	"context"
	"fmt"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/pkg/search"
)

// ElasticsearchBM25Retriever implements BM25 retrieval using Elasticsearch.
type ElasticsearchBM25Retriever struct {
	esClient *search.ElasticsearchClient
}

// NewElasticsearchBM25Retriever creates a new Elasticsearch-based BM25 retriever.
func NewElasticsearchBM25Retriever(esClient *search.ElasticsearchClient) *ElasticsearchBM25Retriever {
	return &ElasticsearchBM25Retriever{
		esClient: esClient,
	}
}

// Search performs BM25 keyword search using Elasticsearch.
func (e *ElasticsearchBM25Retriever) Search(ctx context.Context, query string, topK int) ([]RetrievedChunk, error) {
	if e.esClient == nil {
		return nil, fmt.Errorf("Elasticsearch client not initialized")
	}

	// Perform search
	results, err := e.esClient.Search(ctx, query, topK) // Using default SearchOptions (no multi-tenant filters)
	if err != nil {
		return nil, fmt.Errorf("Elasticsearch search failed: %w", err)
	}

	// Convert to RetrievedChunk
	chunks := make([]RetrievedChunk, len(results))
	for i, result := range results {
		chunks[i] = RetrievedChunk{
			Chunk: chunking.Chunk{
				ID:          result.Document.ID,
				DocumentID:  result.Document.DocumentID,
				Content:     result.Document.Content,
				SectionPath: result.Document.SectionPath,
				Metadata:    result.Document.Metadata,
			},
			Score:      float32(result.Score),
			CitationID: i + 1,
		}
	}

	return chunks, nil
}

// IndexChunks indexes chunks in Elasticsearch for BM25 retrieval.
func (e *ElasticsearchBM25Retriever) IndexChunks(ctx context.Context, chunks []chunking.Chunk) error {
	if e.esClient == nil {
		return fmt.Errorf("Elasticsearch client not initialized")
	}

	// Convert chunks to Elasticsearch documents
	docs := make([]search.Document, len(chunks))
	for i, chunk := range chunks {
		docs[i] = search.Document{
			ID:          chunk.ID,
			DocumentID:  chunk.DocumentID,
			Content:     chunk.Content,
			Title:       chunk.Metadata["title"],
			SectionPath: chunk.SectionPath,
			Metadata:    chunk.Metadata,
		}
	}

	// Bulk index
	return e.esClient.BulkIndex(ctx, docs)
}

// DeleteByDocumentID deletes all chunks for a document from Elasticsearch.
func (e *ElasticsearchBM25Retriever) DeleteByDocumentID(ctx context.Context, documentID string) error {
	if e.esClient == nil {
		return fmt.Errorf("Elasticsearch client not initialized")
	}
	return e.esClient.DeleteByDocumentID(ctx, documentID)
}

