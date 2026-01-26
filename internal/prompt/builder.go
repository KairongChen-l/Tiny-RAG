// Package prompt provides prompt construction functionality.
package prompt

import (
	"context"

	"github.com/krc/rag/internal/retrieval"
)

// PromptOptions holds options for prompt construction.
type PromptOptions struct {
	MaxContextTokens int    // Maximum tokens for context (default: 3000)
	IncludeCitations bool   // Include citation instructions (default: true)
	SystemPrompt     string // Custom system prompt (optional)
}

// DefaultPromptOptions returns default prompt options.
func DefaultPromptOptions() PromptOptions {
	return PromptOptions{
		MaxContextTokens: 3000,
		IncludeCitations: true,
	}
}

// Prompt represents a constructed prompt ready for LLM.
type Prompt struct {
	SystemMessage string         // System message content
	UserMessage   string         // User message content
	TokenCount    int            // Estimated token count
	Citations     []CitationInfo // Citation metadata
}

// CitationInfo holds metadata for a citation.
type CitationInfo struct {
	ID          int    // Citation number (1-based)
	DocumentID  string // Source document ID
	Source      string // Source file path
	SectionPath string // Section path in document
	Preview     string // Content preview
}

// PromptBuilder defines the interface for prompt construction.
type PromptBuilder interface {
	// Build constructs a prompt from query and retrieved chunks.
	Build(ctx context.Context, query string, chunks []retrieval.RetrievedChunk, opts PromptOptions) (*Prompt, error)

	// BuildWithHistory constructs a prompt with conversation history.
	BuildWithHistory(ctx context.Context, query string, chunks []retrieval.RetrievedChunk, history string, opts PromptOptions) (*Prompt, error)
}

// DefaultSystemPrompt is the default system prompt with citation instructions.
const DefaultSystemPrompt = `You are a helpful assistant. Answer the question based on the provided context.
Use the format [citation:x] to cite sources, where x is the context number.
If multiple contexts support a statement, cite all: [citation:1][citation:2].
If the context doesn't contain relevant information, say so.
Be concise and accurate.`
