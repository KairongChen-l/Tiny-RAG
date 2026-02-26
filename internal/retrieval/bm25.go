package retrieval

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/kljensen/snowball"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
)

// BM25RetrieverInterface defines the interface for BM25 retrieval implementations.
type BM25RetrieverInterface interface {
	Search(ctx context.Context, query string, topK int) ([]RetrievedChunk, error)
	IndexChunks(ctx context.Context, chunks []chunking.Chunk) error
	DeleteByDocumentID(ctx context.Context, documentID string) error
}

// BM25Retriever implements keyword-based retrieval using BM25 algorithm (in-memory).
type BM25Retriever struct {
	store     index.VectorStore
	indexed   map[string]*indexedChunk // chunk ID -> indexed chunk
	idf       map[string]float64       // term -> IDF score
	avgDocLen float64
}

// indexedChunk represents a chunk with its term frequencies.
type indexedChunk struct {
	chunk     chunking.Chunk
	termFreq  map[string]int // term -> frequency
	docLength int            // total number of terms
}

// NewBM25Retriever creates a new BM25 retriever.
// Note: This requires chunks to be indexed first via IndexChunks.
func NewBM25Retriever(store index.VectorStore) *BM25Retriever {
	return &BM25Retriever{
		store:   store,
		indexed: make(map[string]*indexedChunk),
		idf:     make(map[string]float64),
	}
}

// IndexChunks indexes chunks for BM25 retrieval.
// This should be called after chunks are stored in the vector store.
func (b *BM25Retriever) IndexChunks(ctx context.Context, chunks []chunking.Chunk) error {
	// Build term frequency index
	totalTerms := 0
	termDocCount := make(map[string]int) // term -> number of documents containing it

	for _, chunk := range chunks {
		terms := b.tokenize(chunk.Content)
		termFreq := make(map[string]int)
		for _, term := range terms {
			termFreq[term]++
			totalTerms++
		}

		b.indexed[chunk.ID] = &indexedChunk{
			chunk:     chunk,
			termFreq:  termFreq,
			docLength: len(terms),
		}

		// Count documents containing each term
		for term := range termFreq {
			termDocCount[term]++
		}
	}

	// Calculate average document length
	if len(chunks) > 0 {
		b.avgDocLen = float64(totalTerms) / float64(len(chunks))
	}

	// Calculate IDF (Inverse Document Frequency)
	totalDocs := float64(len(chunks))
	for term, docCount := range termDocCount {
		// IDF = log((N - df + 0.5) / (df + 0.5))
		// where N is total documents, df is document frequency
		b.idf[term] = b.idfLog((totalDocs - float64(docCount) + 0.5) / (float64(docCount) + 0.5))
	}

	return nil
}

// tokenize tokenizes text into terms (words).
func (b *BM25Retriever) tokenize(text string) []string {
	// Simple tokenization: split by whitespace and punctuation
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fff))
	})

	// Apply stemming (optional, for English)
	terms := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) < 2 {
			continue // Skip very short words
		}
		// Try to stem the word (English only)
		stemmed, err := snowball.Stem(word, "english", true)
		if err == nil && stemmed != "" {
			terms = append(terms, stemmed)
		} else {
			terms = append(terms, word)
		}
	}

	return terms
}

// idf calculates the IDF component: log2((N - df + 0.5) / (df + 0.5)).
// Returns 0 for non-positive inputs to avoid NaN.
func (b *BM25Retriever) idfLog(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return math.Log2(x)
}

// Search performs BM25 keyword search.
func (b *BM25Retriever) Search(ctx context.Context, query string, topK int) ([]RetrievedChunk, error) {
	if len(b.indexed) == 0 {
		return nil, fmt.Errorf("no chunks indexed for BM25 search")
	}

	// Tokenize query
	queryTerms := b.tokenize(query)
	if len(queryTerms) == 0 {
		return nil, fmt.Errorf("query has no valid terms")
	}

	// Calculate BM25 scores for each chunk
	type scorePair struct {
		chunk chunking.Chunk
		score float64
	}
	scores := make([]scorePair, 0, len(b.indexed))

	// BM25 parameters (tunable)
	k1 := 1.5      // term frequency saturation parameter
	bParam := 0.75 // length normalization parameter

	for _, indexed := range b.indexed {
		score := 0.0
		docLength := float64(indexed.docLength)

		for _, term := range queryTerms {
			// Get term frequency in document
			tf := float64(indexed.termFreq[term])
			if tf == 0 {
				continue // Term not in document
			}

			// Get IDF
			idf := b.idf[term]

			// BM25 formula: IDF * (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * (docLen / avgDocLen)))
			numerator := tf * (k1 + 1)
			denominator := tf + k1*(1-bParam+bParam*(docLength/b.avgDocLen))
			score += idf * (numerator / denominator)
		}

		if score > 0 {
			scores = append(scores, scorePair{
				chunk: indexed.chunk,
				score: score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Return top K results
	if topK > 0 && topK < len(scores) {
		scores = scores[:topK]
	}

	// Convert to RetrievedChunk
	results := make([]RetrievedChunk, len(scores))
	for i, pair := range scores {
		results[i] = RetrievedChunk{
			Chunk:      pair.chunk,
			Score:      float32(pair.score),
			CitationID: i + 1,
		}
	}

	return results, nil
}

// DeleteByDocumentID deletes all chunks for a document from the in-memory index.
func (b *BM25Retriever) DeleteByDocumentID(ctx context.Context, documentID string) error {
	// Remove all chunks with matching document ID
	for id, indexed := range b.indexed {
		if indexed.chunk.DocumentID == documentID {
			delete(b.indexed, id)
		}
	}
	// Recalculate IDF (simplified - in production, you'd want to recalculate properly)
	return nil
}
