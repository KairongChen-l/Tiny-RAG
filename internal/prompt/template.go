package prompt

import (
	"context"
	"fmt"
	"strings"

	"github.com/krc/rag/internal/retrieval"
)

// TemplateBuilder implements PromptBuilder with template-based construction.
type TemplateBuilder struct {
	tokenizer Tokenizer
}

// Tokenizer interface for token counting.
type Tokenizer interface {
	Count(text string) int
}

// NewTemplateBuilder creates a new template-based prompt builder.
func NewTemplateBuilder(tokenizer Tokenizer) *TemplateBuilder {
	return &TemplateBuilder{
		tokenizer: tokenizer,
	}
}

// Build constructs a prompt from query and retrieved chunks.
func (b *TemplateBuilder) Build(ctx context.Context, query string, chunks []retrieval.RetrievedChunk, opts PromptOptions) (*Prompt, error) {
	// Apply defaults
	if opts.MaxContextTokens <= 0 {
		opts.MaxContextTokens = 3000
	}

	// Build system message
	systemPrompt := opts.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = b.buildSystemPrompt(opts.IncludeCitations)
	}

	// Build context from chunks
	context, citations, truncated := b.buildContext(chunks, opts)

	// Build user message
	userMessage := b.buildUserMessage(query, context)

	// Calculate token count
	tokenCount := 0
	if b.tokenizer != nil {
		tokenCount = b.tokenizer.Count(systemPrompt) + b.tokenizer.Count(userMessage)
	}

	prompt := &Prompt{
		SystemMessage: systemPrompt,
		UserMessage:   userMessage,
		TokenCount:    tokenCount,
		Citations:     citations,
	}

	if truncated {
		// Add note about truncation
		prompt.UserMessage += "\n\n(Note: Some context was truncated due to length limits.)"
	}

	return prompt, nil
}

// buildSystemPrompt constructs the system prompt.
func (b *TemplateBuilder) buildSystemPrompt(includeCitations bool) string {
	if includeCitations {
		return DefaultSystemPrompt
	}
	return `You are a helpful assistant. Answer the question based on the provided context.
If the context doesn't contain relevant information, say so.
Be concise and accurate.`
}

// buildContext builds the context string from chunks.
func (b *TemplateBuilder) buildContext(chunks []retrieval.RetrievedChunk, opts PromptOptions) (string, []CitationInfo, bool) {
	var contextBuilder strings.Builder
	var citations []CitationInfo
	truncated := false
	currentTokens := 0

	// If no chunks, return empty context
	if len(chunks) == 0 {
		return "", citations, false
	}

	for i, chunk := range chunks {
		citationID := i + 1

		// Format context entry
		entry := fmt.Sprintf("[%d] %s\n\n", citationID, chunk.Content)
		entryTokens := 0
		if b.tokenizer != nil {
			entryTokens = b.tokenizer.Count(entry)
		} else {
			// Rough estimate: 4 chars per token
			entryTokens = len(entry) / 4
		}

		// Check if adding this entry would exceed limit
		if opts.MaxContextTokens > 0 && currentTokens+entryTokens > opts.MaxContextTokens {
			truncated = true
			break
		}

		contextBuilder.WriteString(entry)
		currentTokens += entryTokens

		// Build citation info
		citations = append(citations, CitationInfo{
			ID:          citationID,
			DocumentID:  chunk.DocumentID,
			Source:      chunk.Metadata["source"],
			SectionPath: chunk.SectionPath,
			Preview:     truncatePreview(chunk.Content, 100),
		})
	}

	return contextBuilder.String(), citations, truncated
}

// buildUserMessage constructs the user message.
func (b *TemplateBuilder) buildUserMessage(query, context string) string {
	return fmt.Sprintf("Context:\n%s\nQuestion: %s", context, query)
}

// BuildWithHistory constructs a prompt with conversation history.
func (b *TemplateBuilder) BuildWithHistory(ctx context.Context, query string, chunks []retrieval.RetrievedChunk, history string, opts PromptOptions) (*Prompt, error) {
	// Apply defaults
	if opts.MaxContextTokens <= 0 {
		opts.MaxContextTokens = 3000
	}

	// Build system message with history awareness
	systemPrompt := opts.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = b.buildConversationalSystemPrompt(opts.IncludeCitations)
	}

	// Build context from chunks
	docContext, citations, truncated := b.buildContext(chunks, opts)

	// Build user message with history
	userMessage := b.buildUserMessageWithHistory(query, docContext, history)

	// Calculate token count
	tokenCount := 0
	if b.tokenizer != nil {
		tokenCount = b.tokenizer.Count(systemPrompt) + b.tokenizer.Count(userMessage)
	}

	prompt := &Prompt{
		SystemMessage: systemPrompt,
		UserMessage:   userMessage,
		TokenCount:    tokenCount,
		Citations:     citations,
	}

	if truncated {
		prompt.UserMessage += "\n\n(Note: Some context was truncated due to length limits.)"
	}

	return prompt, nil
}

// buildConversationalSystemPrompt builds system prompt for multi-turn conversation.
func (b *TemplateBuilder) buildConversationalSystemPrompt(includeCitations bool) string {
	base := `You are a helpful AI assistant engaged in a conversation. 
Use the provided context and conversation history to give relevant, helpful answers.
Remember the context of the conversation and refer back to previous exchanges when appropriate.`

	if includeCitations {
		base += `
Use the format [citation:x] to cite sources, where x is the context number.
If multiple contexts support a statement, cite all: [citation:1][citation:2].
If the context doesn't contain relevant information, say so clearly.`
	}

	base += `
Be conversational, concise, and accurate.`

	return base
}

// buildUserMessageWithHistory constructs user message with history.
func (b *TemplateBuilder) buildUserMessageWithHistory(query, docContext, history string) string {
	var sb strings.Builder

	if history != "" {
		sb.WriteString("Previous conversation:\n")
		sb.WriteString(history)
		sb.WriteString("\n---\n\n")
	}

	if docContext != "" {
		sb.WriteString("Retrieved context:\n")
		sb.WriteString(docContext)
		sb.WriteString("\n")
	} else {
		// If no context retrieved, add a note
		sb.WriteString("Note: No relevant documents were found in the knowledge base for this query.\n")
		sb.WriteString("Please answer based on your general knowledge and the conversation history if available.\n\n")
	}

	sb.WriteString("Current question: ")
	sb.WriteString(query)

	return sb.String()
}

// truncatePreview truncates text to maxLen with ellipsis.
func truncatePreview(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// SimpleTokenizer is a basic tokenizer that estimates tokens.
type SimpleTokenizer struct {
	charsPerToken int
}

// NewSimpleTokenizer creates a simple tokenizer.
func NewSimpleTokenizer() *SimpleTokenizer {
	return &SimpleTokenizer{
		charsPerToken: 4, // Rough estimate for English text
	}
}

// Count estimates the token count for text.
func (t *SimpleTokenizer) Count(text string) int {
	return len(text) / t.charsPerToken
}
