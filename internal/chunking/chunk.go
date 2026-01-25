// Package chunking provides document chunking functionality.
package chunking

// Chunk represents a text chunk with metadata for retrieval.
type Chunk struct {
	ID          string            // Unique identifier
	DocumentID  string            // Parent document ID
	Content     string            // Chunk text content
	SectionPath string            // Path in document structure (e.g., "Chapter1/Section2")
	Position    int               // Position in document (0-indexed)
	Metadata    map[string]string // Additional metadata
	Hash        string            // Content hash
}

// NewChunk creates a new Chunk with initialized fields.
func NewChunk(id, documentID, content string) *Chunk {
	return &Chunk{
		ID:         id,
		DocumentID: documentID,
		Content:    content,
		Metadata:   make(map[string]string),
	}
}

// AddMetadata adds a metadata key-value pair to the chunk.
func (c *Chunk) AddMetadata(key, value string) {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
	c.Metadata[key] = value
}

// ChunkWithVector represents a chunk with its embedding vector.
type ChunkWithVector struct {
	Chunk
	Vector []float32
}

// NewChunkWithVector creates a ChunkWithVector from a Chunk and vector.
func NewChunkWithVector(chunk Chunk, vector []float32) *ChunkWithVector {
	return &ChunkWithVector{
		Chunk:  chunk,
		Vector: vector,
	}
}

