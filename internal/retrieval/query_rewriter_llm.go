package retrieval

import (
	"context"
	"fmt"
	"strings"
)

// LLMGenerator defines a simple interface for text generation (to avoid circular imports).
type LLMGenerator interface {
	// GenerateText generates text from a prompt string.
	GenerateText(ctx context.Context, prompt string) (string, error)
}

// LLMQueryRewriter implements query rewriting using LLM.
type LLMQueryRewriter struct {
	llm              LLMGenerator
	enableExpansion  bool
	rewritePrompt    string
	expandPrompt     string
	multiQueryPrompt string
}

// LLMQueryRewriterConfig holds configuration for LLM query rewriter.
type LLMQueryRewriterConfig struct {
	LLM             LLMGenerator
	EnableExpansion bool
}

// NewLLMQueryRewriter creates a new LLM-based query rewriter.
func NewLLMQueryRewriter(cfg LLMQueryRewriterConfig) (*LLMQueryRewriter, error) {
	if cfg.LLM == nil {
		return nil, fmt.Errorf("LLM is required for LLM query rewriter")
	}

	return &LLMQueryRewriter{
		llm:             cfg.LLM,
		enableExpansion: cfg.EnableExpansion,
		rewritePrompt: `Rewrite the following search query to be more effective for information retrieval. 
Keep the core meaning but make it clearer and more specific. Return only the rewritten query, nothing else.

Original query: {{query}}

Rewritten query:`,
		expandPrompt: `Generate synonyms and related terms for the following query. 
Return a comma-separated list of related terms that could help find relevant information.

Query: {{query}}

Related terms:`,
		multiQueryPrompt: `Generate {{count}} different ways to express the following query. 
Each variant should capture the same intent but use different wording.
Return one query per line, nothing else.

Original query: {{query}}

Query variants:`,
	}, nil
}

// Rewrite rewrites a query using LLM.
func (r *LLMQueryRewriter) Rewrite(ctx context.Context, query string) (string, error) {
	prompt := strings.ReplaceAll(r.rewritePrompt, "{{query}}", query)

	response, err := r.llm.GenerateText(ctx, prompt)
	if err != nil {
		return query, fmt.Errorf("failed to rewrite query: %w", err)
	}

	// Clean up response
	rewritten := strings.TrimSpace(response)
	if rewritten == "" {
		return query, nil // Fallback to original
	}

	return rewritten, nil
}

// Expand expands query with synonyms using LLM.
func (r *LLMQueryRewriter) Expand(ctx context.Context, query string) ([]string, error) {
	if !r.enableExpansion {
		return []string{query}, nil
	}

	prompt := strings.ReplaceAll(r.expandPrompt, "{{query}}", query)

	response, err := r.llm.GenerateText(ctx, prompt)
	if err != nil {
		return []string{query}, fmt.Errorf("failed to expand query: %w", err)
	}

	// Parse comma-separated terms
	terms := strings.Split(response, ",")
	expanded := []string{query} // Always include original

	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term != "" && term != query {
			expanded = append(expanded, term)
		}
	}

	return expanded, nil
}

// GenerateMultiQuery generates multiple query variants using LLM.
func (r *LLMQueryRewriter) GenerateMultiQuery(ctx context.Context, query string, count int) ([]string, error) {
	if count <= 0 {
		count = 3
	}

	prompt := strings.ReplaceAll(r.multiQueryPrompt, "{{query}}", query)
	prompt = strings.ReplaceAll(prompt, "{{count}}", fmt.Sprintf("%d", count))

	response, err := r.llm.GenerateText(ctx, prompt)
	if err != nil {
		return []string{query}, fmt.Errorf("failed to generate multi-query: %w", err)
	}

	// Parse line-separated queries
	lines := strings.Split(response, "\n")
	queries := []string{query} // Always include original

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != query {
			queries = append(queries, line)
		}
	}

	// Trim to requested count
	if len(queries) > count {
		queries = queries[:count]
	}

	return queries, nil
}
