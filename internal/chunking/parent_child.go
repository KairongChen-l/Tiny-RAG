package chunking

import (
	"strings"

	"github.com/google/uuid"
	"github.com/krc/rag/internal/ingestion"
)

// ParentChildConfig holds configuration for parent-child chunking.
type ParentChildConfig struct {
	ParentChunkSize int // Maximum parent chunk size in characters (default: 2000)
	ChildChunkSize  int // Maximum child chunk size in characters (default: 400)
	ChildOverlap    int // Overlap between child chunks in characters (default: 50)
}

// DefaultParentChildConfig returns the default parent-child chunking configuration.
func DefaultParentChildConfig() ParentChildConfig {
	return ParentChildConfig{
		ParentChunkSize: 2000,
		ChildChunkSize:  400,
		ChildOverlap:    50,
	}
}

// ParentChildChunker implements a two-level chunking strategy where large parent
// chunks preserve full context while smaller child chunks enable fine-grained
// retrieval. Child chunks reference their parent so that context can be restored
// after retrieval.
type ParentChildChunker struct {
	config ParentChildConfig
}

// NewParentChildChunker creates a new ParentChildChunker with the given config.
func NewParentChildChunker(config ParentChildConfig) *ParentChildChunker {
	return &ParentChildChunker{config: config}
}

// Split splits a document into parent and child chunks. Parent chunks (ChunkLevel 0)
// cover large segments of the document. Each parent is further split into child
// chunks (ChunkLevel 1) that reference the parent via ParentChunkID. Both parent
// and child chunks are returned together; callers can distinguish them by ChunkLevel.
func (pc *ParentChildChunker) Split(doc *ingestion.Document) ([]Chunk, error) {
	var allChunks []Chunk

	flatSections := doc.Flatten()

	for _, section := range flatSections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}

		parents := pc.createParentChunks(doc.ID, section.Path, content)
		for i := range parents {
			children := pc.splitIntoChildren(doc.ID, section.Path, &parents[i])
			allChunks = append(allChunks, parents[i])
			allChunks = append(allChunks, children...)
		}
	}

	// Assign global positions
	for i := range allChunks {
		allChunks[i].Position = i
	}

	return allChunks, nil
}

// createParentChunks splits section content into large parent-level chunks,
// preferring paragraph boundaries.
func (pc *ParentChildChunker) createParentChunks(docID, path, content string) []Chunk {
	if len(content) <= pc.config.ParentChunkSize {
		return []Chunk{pc.newParentChunk(docID, path, content)}
	}

	paragraphs := strings.Split(content, "\n\n")
	var chunks []Chunk
	var buf strings.Builder

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		needed := buf.Len() + len(para)
		if buf.Len() > 0 {
			needed += 2 // "\n\n"
		}

		if needed > pc.config.ParentChunkSize && buf.Len() > 0 {
			chunks = append(chunks, pc.newParentChunk(docID, path, buf.String()))
			buf.Reset()
		}

		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(para)

		// Handle a single paragraph that exceeds the parent size
		if buf.Len() > pc.config.ParentChunkSize {
			chunks = append(chunks, pc.forceSplitParent(docID, path, buf.String())...)
			buf.Reset()
		}
	}

	if buf.Len() > 0 {
		chunks = append(chunks, pc.newParentChunk(docID, path, buf.String()))
	}

	return chunks
}

// forceSplitParent splits text that exceeds ParentChunkSize by character boundary.
func (pc *ParentChildChunker) forceSplitParent(docID, path, text string) []Chunk {
	var chunks []Chunk
	runes := []rune(text)

	for i := 0; i < len(runes); {
		end := i + pc.config.ParentChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		// Try to break at a space
		if end < len(runes) {
			for j := end - 1; j > i+pc.config.ParentChunkSize/2; j-- {
				if runes[j] == ' ' || runes[j] == '\n' {
					end = j + 1
					break
				}
			}
		}
		chunks = append(chunks, pc.newParentChunk(docID, path, string(runes[i:end])))
		i = end
	}

	return chunks
}

// splitIntoChildren splits a parent chunk into smaller child chunks with overlap.
// It updates the parent's ChildChunkIDs and sets each child's ParentChunkID.
func (pc *ParentChildChunker) splitIntoChildren(docID, path string, parent *Chunk) []Chunk {
	content := parent.Content
	if len(content) <= pc.config.ChildChunkSize {
		// Content is small enough to be a single child
		child := pc.newChildChunk(docID, path, content, parent.ID)
		parent.ChildChunkIDs = append(parent.ChildChunkIDs, child.ID)
		return []Chunk{child}
	}

	var children []Chunk
	runes := []rune(content)

	for i := 0; i < len(runes); {
		end := i + pc.config.ChildChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		// Try to break at a sentence or word boundary
		if end < len(runes) {
			for j := end - 1; j > i+pc.config.ChildChunkSize/2; j-- {
				r := runes[j]
				if r == '.' || r == '!' || r == '?' || r == '\n' || r == ' ' {
					end = j + 1
					break
				}
			}
		}

		childContent := strings.TrimSpace(string(runes[i:end]))
		if childContent != "" {
			child := pc.newChildChunk(docID, path, childContent, parent.ID)
			children = append(children, child)
			parent.ChildChunkIDs = append(parent.ChildChunkIDs, child.ID)
		}

		next := end - pc.config.ChildOverlap
		if next <= i {
			next = end
		}
		i = next
	}

	return children
}

// newParentChunk creates a parent-level chunk (ChunkLevel 0).
func (pc *ParentChildChunker) newParentChunk(docID, path, content string) Chunk {
	content = strings.TrimSpace(content)
	return Chunk{
		ID:          uuid.New().String(),
		DocumentID:  docID,
		Content:     content,
		SectionPath: path,
		Metadata:    make(map[string]string),
		Hash:        calculateChunkHash(content),
		ChunkLevel:  0,
	}
}

// newChildChunk creates a child-level chunk (ChunkLevel 1) linked to a parent.
func (pc *ParentChildChunker) newChildChunk(docID, path, content, parentID string) Chunk {
	content = strings.TrimSpace(content)
	return Chunk{
		ID:            uuid.New().String(),
		DocumentID:    docID,
		Content:       content,
		SectionPath:   path,
		Metadata:      make(map[string]string),
		Hash:          calculateChunkHash(content),
		ParentChunkID: parentID,
		ChunkLevel:    1,
	}
}
