package chunking

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/krc/rag/internal/embedding"
	"github.com/krc/rag/internal/ingestion"
)

// EmbeddingSemanticChunker implements semantic chunking using embedding similarity.
type EmbeddingSemanticChunker struct {
	config              ChunkerConfig
	embedder            embedding.Embedder
	similarityThreshold float32
}

// EmbeddingSemanticChunkerConfig holds configuration for embedding-based semantic chunking.
type EmbeddingSemanticChunkerConfig struct {
	ChunkerConfig
	Embedder      embedding.Embedder
	SimilarityThreshold float32 // Minimum similarity to merge chunks (default: 0.7)
}

// NewEmbeddingSemanticChunker creates a new embedding-based semantic chunker.
func NewEmbeddingSemanticChunker(cfg EmbeddingSemanticChunkerConfig) (*EmbeddingSemanticChunker, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("embedder is required for semantic chunking")
	}
	if cfg.SimilarityThreshold == 0 {
		cfg.SimilarityThreshold = 0.7
	}

	return &EmbeddingSemanticChunker{
		config:              cfg.ChunkerConfig,
		embedder:            cfg.Embedder,
		similarityThreshold: cfg.SimilarityThreshold,
	}, nil
}

// Split splits a document into chunks using embedding similarity to detect boundaries.
func (c *EmbeddingSemanticChunker) Split(doc *ingestion.Document) ([]Chunk, error) {
	ctx := context.Background()

	// Flatten document sections
	flatSections := doc.Flatten()

	var allChunks []Chunk
	for _, section := range flatSections {
		sectionChunks, err := c.splitSectionSemantic(ctx, doc.ID, section)
		if err != nil {
			return nil, fmt.Errorf("failed to split section %s: %w", section.Path, err)
		}
		allChunks = append(allChunks, sectionChunks...)
	}

	// Assign positions
	for i := range allChunks {
		allChunks[i].Position = i
	}

	return allChunks, nil
}

// splitSectionSemantic splits a section using semantic boundaries.
func (c *EmbeddingSemanticChunker) splitSectionSemantic(ctx context.Context, docID string, section ingestion.FlatSection) ([]Chunk, error) {
	content := section.Content
	if content == "" {
		return nil, nil
	}

	// If content fits in one chunk, return as is
	if len(content) <= c.config.MaxChunkSize {
		return []Chunk{
			c.createChunk(docID, section.Path, content),
		}, nil
	}

	// Split into sentences first
	sentences := splitSentences(content)
	if len(sentences) == 0 {
		return nil, nil
	}

	// Group sentences into semantic chunks
	return c.groupBySemanticSimilarity(ctx, docID, section.Path, sentences)
}

// groupBySemanticSimilarity groups sentences into chunks based on semantic similarity.
func (c *EmbeddingSemanticChunker) groupBySemanticSimilarity(ctx context.Context, docID, path string, sentences []string) ([]Chunk, error) {
	if len(sentences) == 0 {
		return nil, nil
	}

	var chunks []Chunk
	var currentGroup []string
	var currentSize int

	// Embed first sentence to start comparison
	firstEmbedding, err := c.embedder.Embed(ctx, sentences[0])
	if err != nil {
		// Fallback to size-based chunking if embedding fails
		return c.fallbackSizeChunking(docID, path, sentences), nil
	}

	currentGroup = []string{sentences[0]}
	currentSize = len(sentences[0])
	prevEmbedding := firstEmbedding

	// Process remaining sentences
	for i := 1; i < len(sentences); i++ {
		sentence := sentences[i]
		sentenceSize := len(sentence)

		// Check if adding this sentence would exceed max size
		if currentSize+sentenceSize > c.config.MaxChunkSize && len(currentGroup) > 0 {
			// Save current chunk
			chunkContent := strings.Join(currentGroup, " ")
			chunks = append(chunks, c.createChunk(docID, path, chunkContent))

			// Start new chunk with overlap
			overlapSentences := c.getOverlapSentences(currentGroup)
			currentGroup = overlapSentences
			currentSize = len(strings.Join(currentGroup, " "))

			// Re-embed for new chunk
			if len(currentGroup) > 0 {
				prevEmbedding, err = c.embedder.Embed(ctx, strings.Join(currentGroup, " "))
				if err != nil {
					// Continue without semantic comparison
					prevEmbedding = nil
				}
			}
		}

		// Check semantic similarity if we have previous embedding
		if prevEmbedding != nil {
			currEmbedding, err := c.embedder.Embed(ctx, sentence)
			if err == nil {
				similarity := cosineSimilarity(prevEmbedding, currEmbedding)
				if similarity < c.similarityThreshold && len(currentGroup) > 0 {
					// Low similarity indicates semantic boundary
					// Save current chunk and start new one
					chunkContent := strings.Join(currentGroup, " ")
					if len(chunkContent) >= c.config.MinChunkSize {
						chunks = append(chunks, c.createChunk(docID, path, chunkContent))
					}

					// Start new chunk with overlap
					overlapSentences := c.getOverlapSentences(currentGroup)
					currentGroup = overlapSentences
					currentSize = len(strings.Join(currentGroup, " "))
					prevEmbedding = currEmbedding
					continue
				}
				prevEmbedding = currEmbedding
			}
		}

		// Add sentence to current group
		currentGroup = append(currentGroup, sentence)
		currentSize += sentenceSize + 1 // +1 for space
	}

	// Add final chunk
	if len(currentGroup) > 0 {
		chunkContent := strings.Join(currentGroup, " ")
		if len(chunkContent) >= c.config.MinChunkSize || len(chunks) == 0 {
			chunks = append(chunks, c.createChunk(docID, path, chunkContent))
		} else if len(chunks) > 0 {
			// Merge small final chunk with previous
			lastIdx := len(chunks) - 1
			chunks[lastIdx].Content += " " + chunkContent
			chunks[lastIdx].Hash = calculateChunkHash(chunks[lastIdx].Content)
		}
	}

	return chunks, nil
}

// getOverlapSentences returns sentences for overlap from the end of a group.
func (c *EmbeddingSemanticChunker) getOverlapSentences(group []string) []string {
	if c.config.Overlap == 0 || len(group) == 0 {
		return nil
	}

	// Calculate how many sentences to include based on overlap size
	totalSize := 0
	var overlapSentences []string
	for i := len(group) - 1; i >= 0; i-- {
		sentence := group[i]
		if totalSize+len(sentence) > c.config.Overlap {
			break
		}
		overlapSentences = append([]string{sentence}, overlapSentences...)
		totalSize += len(sentence) + 1
	}

	return overlapSentences
}

// fallbackSizeChunking falls back to size-based chunking when embedding fails.
func (c *EmbeddingSemanticChunker) fallbackSizeChunking(docID, path string, sentences []string) []Chunk {
	var chunks []Chunk
	var currentChunk strings.Builder

	for _, sentence := range sentences {
		if currentChunk.Len()+len(sentence) > c.config.MaxChunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, c.createChunk(docID, path, currentChunk.String()))
			currentChunk.Reset()
		}
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(sentence)
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, c.createChunk(docID, path, currentChunk.String()))
	}

	return chunks
}

// createChunk creates a new chunk with ID and hash.
func (c *EmbeddingSemanticChunker) createChunk(docID, path, content string) Chunk {
	return Chunk{
		ID:          uuid.New().String(),
		DocumentID:  docID,
		Content:     strings.TrimSpace(content),
		SectionPath: path,
		Metadata:    make(map[string]string),
		Hash:        calculateChunkHash(content),
	}
}

// cosineSimilarity calculates cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt calculates square root (simple approximation).
func sqrt(x float32) float32 {
	// Simple approximation using Newton's method
	if x == 0 {
		return 0
	}
	if x < 0 {
		return 0
	}

	guess := x / 2
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

