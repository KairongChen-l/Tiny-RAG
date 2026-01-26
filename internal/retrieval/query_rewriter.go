package retrieval

import (
	"context"
	"strings"
)

// QueryRewriter defines the interface for query rewriting and expansion.
type QueryRewriter interface {
	// Rewrite rewrites a query to improve retrieval.
	Rewrite(ctx context.Context, query string) (string, error)

	// Expand expands a query with synonyms and related terms.
	Expand(ctx context.Context, query string) ([]string, error)

	// GenerateMultiQuery generates multiple query variants.
	GenerateMultiQuery(ctx context.Context, query string, count int) ([]string, error)
}

// SimpleQueryRewriter implements basic query rewriting without LLM.
type SimpleQueryRewriter struct {
	enableExpansion bool
}

// NewSimpleQueryRewriter creates a new simple query rewriter.
func NewSimpleQueryRewriter(enableExpansion bool) *SimpleQueryRewriter {
	return &SimpleQueryRewriter{
		enableExpansion: enableExpansion,
	}
}

// Rewrite performs basic query normalization.
func (r *SimpleQueryRewriter) Rewrite(ctx context.Context, query string) (string, error) {
	// Basic normalization
	query = strings.TrimSpace(query)

	// Remove extra whitespace
	words := strings.Fields(query)
	query = strings.Join(words, " ")

	return query, nil
}

// Expand expands query with basic synonyms (placeholder implementation).
func (r *SimpleQueryRewriter) Expand(ctx context.Context, query string) ([]string, error) {
	if !r.enableExpansion {
		return []string{query}, nil
	}

	// Basic expansion: add query variations
	variations := []string{query}

	// Add lowercase version
	if strings.ToLower(query) != query {
		variations = append(variations, strings.ToLower(query))
	}

	// Add question form if not already a question
	if !strings.HasSuffix(query, "?") && !strings.HasSuffix(query, "？") {
		variations = append(variations, query+"?")
	}

	return variations, nil
}

// GenerateMultiQuery generates multiple query variants.
func (r *SimpleQueryRewriter) GenerateMultiQuery(ctx context.Context, query string, count int) ([]string, error) {
	if count <= 0 {
		count = 3
	}

	queries := []string{query}

	// Generate variations
	words := strings.Fields(query)
	if len(words) > 1 {
		// Variation 1: Remove stop words (simple)
		stopWords := map[string]bool{
			"the": true, "a": true, "an": true, "is": true, "are": true,
			"was": true, "were": true, "be": true, "been": true,
			"what": true, "how": true, "why": true, "when": true, "where": true,
		}
		var filteredWords []string
		for _, word := range words {
			if !stopWords[strings.ToLower(word)] {
				filteredWords = append(filteredWords, word)
			}
		}
		if len(filteredWords) > 0 && len(filteredWords) < len(words) {
			queries = append(queries, strings.Join(filteredWords, " "))
		}

		// Variation 2: Question form
		if !strings.HasSuffix(query, "?") {
			queries = append(queries, query+"?")
		}

		// Variation 3: Statement form (remove question mark)
		if strings.HasSuffix(query, "?") {
			queries = append(queries, strings.TrimSuffix(query, "?"))
		}
	}

	// Trim to requested count
	if len(queries) > count {
		queries = queries[:count]
	}

	return queries, nil
}
