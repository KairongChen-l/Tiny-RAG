package chunking

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/krc/rag/internal/ingestion"
)

// SemanticChunker implements structure-aware document chunking.
type SemanticChunker struct {
	config ChunkerConfig
}

// NewSemanticChunker creates a new semantic chunker with the given config.
func NewSemanticChunker(config ChunkerConfig) *SemanticChunker {
	return &SemanticChunker{config: config}
}

// Split splits a document into chunks while respecting semantic boundaries.
func (c *SemanticChunker) Split(doc *ingestion.Document) ([]Chunk, error) {
	var chunks []Chunk

	// Flatten document sections
	flatSections := doc.Flatten()

	for _, section := range flatSections {
		sectionChunks := c.splitSection(doc.ID, section)
		chunks = append(chunks, sectionChunks...)
	}

	// Assign positions
	for i := range chunks {
		chunks[i].Position = i
	}

	return chunks, nil
}

// splitSection splits a single section into chunks.
func (c *SemanticChunker) splitSection(docID string, section ingestion.FlatSection) []Chunk {
	content := section.Content
	if content == "" {
		return nil
	}

	// If content fits in one chunk, return as is
	if len(content) <= c.config.MaxChunkSize {
		return []Chunk{
			c.createChunk(docID, section.Path, content),
		}
	}

	// Split by paragraphs first if respecting bounds
	if c.config.RespectBounds {
		return c.splitByParagraphs(docID, section.Path, content)
	}

	// Otherwise, split by size
	return c.splitBySize(docID, section.Path, content)
}

// splitByParagraphs splits content by paragraph boundaries.
func (c *SemanticChunker) splitByParagraphs(docID, path, content string) []Chunk {
	paragraphs := strings.Split(content, "\n\n")
	var chunks []Chunk
	var currentChunk strings.Builder

	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Check if adding this paragraph would exceed max size
		potentialSize := currentChunk.Len() + len(para)
		if currentChunk.Len() > 0 {
			potentialSize += 2 // for "\n\n" separator
		}

		if potentialSize > c.config.MaxChunkSize && currentChunk.Len() > 0 {
			// Save current chunk
			chunks = append(chunks, c.createChunk(docID, path, currentChunk.String()))

			// Start new chunk with overlap
			overlapText := c.getOverlapText(currentChunk.String())
			currentChunk.Reset()
			if overlapText != "" {
				currentChunk.WriteString(overlapText)
				currentChunk.WriteString("\n\n")
			}
		}

		// Add paragraph to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)

		// Handle very long paragraphs
		if currentChunk.Len() > c.config.MaxChunkSize {
			// Force split the paragraph
			longChunks := c.splitLongParagraph(docID, path, currentChunk.String())
			if len(longChunks) > 0 {
				chunks = append(chunks, longChunks[:len(longChunks)-1]...)
				// Keep last part for potential continuation
				if i < len(paragraphs)-1 {
					currentChunk.Reset()
					currentChunk.WriteString(longChunks[len(longChunks)-1].Content)
				} else {
					chunks = append(chunks, longChunks[len(longChunks)-1])
					currentChunk.Reset()
				}
			}
		}
	}

	// Don't forget the last chunk
	if currentChunk.Len() >= c.config.MinChunkSize {
		chunks = append(chunks, c.createChunk(docID, path, currentChunk.String()))
	} else if currentChunk.Len() > 0 && len(chunks) > 0 {
		// Merge small final chunk with previous
		lastIdx := len(chunks) - 1
		chunks[lastIdx].Content += "\n\n" + currentChunk.String()
		chunks[lastIdx].Hash = calculateChunkHash(chunks[lastIdx].Content)
	} else if currentChunk.Len() > 0 {
		// Add even if small (it's the only chunk)
		chunks = append(chunks, c.createChunk(docID, path, currentChunk.String()))
	}

	return chunks
}

// splitBySize splits content by size only.
func (c *SemanticChunker) splitBySize(docID, path, content string) []Chunk {
	var chunks []Chunk
	runes := []rune(content)

	for i := 0; i < len(runes); {
		end := i + c.config.MaxChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		// Try to find a good break point
		if end < len(runes) {
			// Look for sentence or word boundary
			for j := end - 1; j > i+c.config.MinChunkSize; j-- {
				r := runes[j]
				if r == '.' || r == '!' || r == '?' || r == '\n' || r == ' ' {
					end = j + 1
					break
				}
			}
		}

		chunkContent := string(runes[i:end])
		chunks = append(chunks, c.createChunk(docID, path, chunkContent))

		// Calculate next start with overlap
		nextStart := end - c.config.Overlap
		if nextStart <= i {
			nextStart = end
		}
		i = nextStart
	}

	return chunks
}

// splitLongParagraph handles paragraphs that exceed max size.
func (c *SemanticChunker) splitLongParagraph(docID, path, content string) []Chunk {
	// Try to split by sentences first
	sentences := splitSentences(content)
	if len(sentences) > 1 {
		var chunks []Chunk
		var currentChunk strings.Builder

		for _, sent := range sentences {
			if currentChunk.Len()+len(sent) > c.config.MaxChunkSize && currentChunk.Len() > 0 {
				chunks = append(chunks, c.createChunk(docID, path, currentChunk.String()))
				overlapText := c.getOverlapText(currentChunk.String())
				currentChunk.Reset()
				if overlapText != "" {
					currentChunk.WriteString(overlapText)
					currentChunk.WriteString(" ")
				}
			}
			currentChunk.WriteString(sent)
		}

		if currentChunk.Len() > 0 {
			chunks = append(chunks, c.createChunk(docID, path, currentChunk.String()))
		}
		return chunks
	}

	// Fall back to size-based splitting
	return c.splitBySize(docID, path, content)
}

// getOverlapText returns the overlap text from the end of content.
func (c *SemanticChunker) getOverlapText(content string) string {
	if c.config.Overlap == 0 {
		return ""
	}

	runes := []rune(content)
	if len(runes) <= c.config.Overlap {
		return content
	}

	// Try to start overlap at word boundary
	start := len(runes) - c.config.Overlap
	for i := start; i < len(runes); i++ {
		if runes[i] == ' ' {
			return string(runes[i+1:])
		}
	}

	return string(runes[start:])
}

// createChunk creates a new chunk with ID and hash.
func (c *SemanticChunker) createChunk(docID, path, content string) Chunk {
	return Chunk{
		ID:          uuid.New().String(),
		DocumentID:  docID,
		Content:     strings.TrimSpace(content),
		SectionPath: path,
		Metadata:    make(map[string]string),
		Hash:        calculateChunkHash(content),
	}
}

// splitSentences splits text into sentences.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		// Check for sentence end
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Check if followed by space and uppercase (new sentence)
			if i+2 < len(runes) && runes[i+1] == ' ' {
				nextChar := runes[i+2]
				if nextChar >= 'A' && nextChar <= 'Z' {
					sentences = append(sentences, current.String())
					current.Reset()
					i++ // Skip the space
					continue
				}
			}
			// End of text
			if i == len(runes)-1 || (i+1 < len(runes) && runes[i+1] == ' ') {
				sentences = append(sentences, current.String())
				current.Reset()
			}
		}
	}

	// Add remaining text
	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}

	return sentences
}

// calculateChunkHash calculates SHA256 hash of chunk content.
func calculateChunkHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

