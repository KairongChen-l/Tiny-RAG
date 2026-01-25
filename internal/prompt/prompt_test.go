package prompt

import (
	"context"
	"testing"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/retrieval"
)

func TestTemplateBuilder_Build(t *testing.T) {
	tokenizer := NewSimpleTokenizer()
	builder := NewTemplateBuilder(tokenizer)

	chunks := []retrieval.RetrievedChunk{
		{
			Chunk: chunking.Chunk{
				ID:          "chunk1",
				DocumentID:  "doc1",
				Content:     "This is the first chunk content about topic A.",
				SectionPath: "Section1",
				Metadata:    map[string]string{"source": "test.md"},
			},
			Score:      0.9,
			CitationID: 1,
		},
		{
			Chunk: chunking.Chunk{
				ID:          "chunk2",
				DocumentID:  "doc1",
				Content:     "This is the second chunk content about topic B.",
				SectionPath: "Section2",
				Metadata:    map[string]string{"source": "test.md"},
			},
			Score:      0.8,
			CitationID: 2,
		},
	}

	opts := PromptOptions{
		MaxContextTokens: 1000,
		IncludeCitations: true,
	}

	prompt, err := builder.Build(context.Background(), "What is topic A?", chunks, opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify system message contains citation instructions
	if prompt.SystemMessage == "" {
		t.Error("expected non-empty system message")
	}

	// Verify user message contains context and query
	if prompt.UserMessage == "" {
		t.Error("expected non-empty user message")
	}
	if len(prompt.UserMessage) < 10 {
		t.Error("user message too short")
	}

	// Verify citations
	if len(prompt.Citations) != 2 {
		t.Errorf("expected 2 citations, got %d", len(prompt.Citations))
	}

	// Verify token count
	if prompt.TokenCount <= 0 {
		t.Error("expected positive token count")
	}
}

func TestTemplateBuilder_Truncation(t *testing.T) {
	tokenizer := NewSimpleTokenizer()
	builder := NewTemplateBuilder(tokenizer)

	// Create many chunks
	var chunks []retrieval.RetrievedChunk
	for i := 0; i < 10; i++ {
		chunks = append(chunks, retrieval.RetrievedChunk{
			Chunk: chunking.Chunk{
				ID:          "chunk" + string(rune('0'+i)),
				DocumentID:  "doc1",
				Content:     "This is a fairly long chunk content that takes up some tokens. " + string(rune('A'+i)),
				SectionPath: "Section",
			},
			Score:      0.9,
			CitationID: i + 1,
		})
	}

	opts := PromptOptions{
		MaxContextTokens: 100, // Very small limit
		IncludeCitations: true,
	}

	prompt, err := builder.Build(context.Background(), "Test query", chunks, opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have fewer citations due to truncation
	if len(prompt.Citations) >= 10 {
		t.Error("expected truncation to reduce citations")
	}
}

func TestParseCitations(t *testing.T) {
	tests := []struct {
		text     string
		expected []int
	}{
		{"No citations here.", nil},
		{"See [citation:1] for details.", []int{1}},
		{"Both [citation:1] and [citation:2] apply.", []int{1, 2}},
		{"[citation:1][citation:2][citation:1]", []int{1, 2}}, // Deduped
		{"[citation:10] is high.", []int{10}},
	}

	for _, tt := range tests {
		citations := ParseCitations(tt.text)
		if len(citations) != len(tt.expected) {
			t.Errorf("ParseCitations(%q) = %v, expected %v", tt.text, citations, tt.expected)
			continue
		}
		for i, c := range citations {
			if c != tt.expected[i] {
				t.Errorf("ParseCitations(%q)[%d] = %d, expected %d", tt.text, i, c, tt.expected[i])
			}
		}
	}
}

func TestSimpleTokenizer(t *testing.T) {
	tokenizer := NewSimpleTokenizer()

	// 4 chars per token by default
	text := "This is a test sentence."
	tokens := tokenizer.Count(text)

	// 25 chars / 4 = ~6 tokens
	if tokens < 5 || tokens > 8 {
		t.Errorf("expected ~6 tokens, got %d", tokens)
	}
}

func TestTruncator(t *testing.T) {
	tokenizer := NewSimpleTokenizer()
	truncator := NewTruncator(tokenizer, 20, 5) // 15 available tokens

	// Long text should be truncated
	longText := "This is a very long text that should be truncated because it exceeds the token limit."
	result := truncator.TruncateText(longText)

	if len(result) >= len(longText) {
		t.Error("expected truncation")
	}
	if !contains(result, "...") {
		t.Error("expected ellipsis at end")
	}

	// Short text should not be truncated
	shortText := "Short text."
	result = truncator.TruncateText(shortText)
	if result != shortText {
		t.Errorf("short text should not be truncated: got %q", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}

