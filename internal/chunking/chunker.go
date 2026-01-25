package chunking

import (
	"github.com/krc/rag/internal/ingestion"
)

// ChunkerConfig holds configuration for text chunking.
type ChunkerConfig struct {
	MaxChunkSize   int  // Maximum chunk size in characters (default: 1000)
	MinChunkSize   int  // Minimum chunk size in characters (default: 100)
	Overlap        int  // Overlap between chunks in characters (default: 100)
	RespectBounds  bool // Respect paragraph/heading boundaries (default: true)
	SplitByHeading bool // Split by heading levels (default: true)
}

// DefaultChunkerConfig returns the default chunking configuration.
func DefaultChunkerConfig() ChunkerConfig {
	return ChunkerConfig{
		MaxChunkSize:   1000,
		MinChunkSize:   100,
		Overlap:        100,
		RespectBounds:  true,
		SplitByHeading: true,
	}
}

// Chunker defines the interface for document chunking.
type Chunker interface {
	// Split splits a document into chunks.
	Split(doc *ingestion.Document) ([]Chunk, error)
}

