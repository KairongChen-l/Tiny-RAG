package chunking

import (
	"testing"

	"github.com/krc/rag/internal/ingestion"
)

func TestSemanticChunker_Basic(t *testing.T) {
	config := ChunkerConfig{
		MaxChunkSize:   200,
		MinChunkSize:   50,
		Overlap:        20,
		RespectBounds:  true,
		SplitByHeading: true,
	}

	chunker := NewSemanticChunker(config)

	doc := &ingestion.Document{
		ID:    "doc1",
		Title: "Test Document",
		Sections: []ingestion.Section{
			{
				Title:   "Introduction",
				Level:   1,
				Content: "This is the introduction. It contains some basic information about the topic.",
			},
			{
				Title:   "Main Content",
				Level:   1,
				Content: "This is the main content section. It has more detailed information.",
			},
		},
	}

	chunks, err := chunker.Split(doc)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify all chunks have required fields
	for i, chunk := range chunks {
		if chunk.ID == "" {
			t.Errorf("chunk %d has empty ID", i)
		}
		if chunk.DocumentID != "doc1" {
			t.Errorf("chunk %d has wrong document ID", i)
		}
		if chunk.Content == "" {
			t.Errorf("chunk %d has empty content", i)
		}
		if chunk.Hash == "" {
			t.Errorf("chunk %d has empty hash", i)
		}
	}
}

func TestSemanticChunker_LongContent(t *testing.T) {
	config := ChunkerConfig{
		MaxChunkSize:   100,
		MinChunkSize:   20,
		Overlap:        10,
		RespectBounds:  true,
		SplitByHeading: true,
	}

	chunker := NewSemanticChunker(config)

	// Create a document with long content
	longContent := "This is sentence one. This is sentence two. This is sentence three. " +
		"This is sentence four. This is sentence five. This is sentence six. " +
		"This is sentence seven. This is sentence eight. This is sentence nine. " +
		"This is sentence ten which is the final sentence."

	doc := &ingestion.Document{
		ID:    "doc1",
		Title: "Long Document",
		Sections: []ingestion.Section{
			{
				Title:   "Content",
				Level:   1,
				Content: longContent,
			},
		},
	}

	chunks, err := chunker.Split(doc)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Should have multiple chunks due to length
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for long content, got %d", len(chunks))
	}

	// Verify no chunk exceeds max size (with some tolerance for word boundaries)
	for i, chunk := range chunks {
		if len(chunk.Content) > config.MaxChunkSize+50 {
			t.Errorf("chunk %d exceeds max size: %d > %d", i, len(chunk.Content), config.MaxChunkSize)
		}
	}
}

func TestSemanticChunker_Paragraphs(t *testing.T) {
	config := ChunkerConfig{
		MaxChunkSize:   150,
		MinChunkSize:   30,
		Overlap:        20,
		RespectBounds:  true,
		SplitByHeading: true,
	}

	chunker := NewSemanticChunker(config)

	content := `First paragraph with some content.

Second paragraph with different content.

Third paragraph with more content here.`

	doc := &ingestion.Document{
		ID:    "doc1",
		Title: "Paragraph Document",
		Sections: []ingestion.Section{
			{
				Title:   "Content",
				Level:   1,
				Content: content,
			},
		},
	}

	chunks, err := chunker.Split(doc)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Verify chunks were created
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestSemanticChunker_SmallContent(t *testing.T) {
	config := ChunkerConfig{
		MaxChunkSize:   1000,
		MinChunkSize:   10,
		Overlap:        20,
		RespectBounds:  true,
		SplitByHeading: true,
	}

	chunker := NewSemanticChunker(config)

	doc := &ingestion.Document{
		ID:    "doc1",
		Title: "Small Document",
		Sections: []ingestion.Section{
			{
				Title:   "Content",
				Level:   1,
				Content: "Short content.",
			},
		},
	}

	chunks, err := chunker.Split(doc)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Should have exactly one chunk for small content
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for small content, got %d", len(chunks))
	}
}

func TestSemanticChunker_EmptySection(t *testing.T) {
	config := DefaultChunkerConfig()
	chunker := NewSemanticChunker(config)

	doc := &ingestion.Document{
		ID:    "doc1",
		Title: "Empty Section Document",
		Sections: []ingestion.Section{
			{
				Title:   "Empty",
				Level:   1,
				Content: "",
			},
			{
				Title:   "Has Content",
				Level:   1,
				Content: "This section has content.",
			},
		},
	}

	chunks, err := chunker.Split(doc)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Should only have chunks from non-empty sections
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk (from non-empty section), got %d", len(chunks))
	}
}

func TestDefaultChunkerConfig(t *testing.T) {
	config := DefaultChunkerConfig()

	if config.MaxChunkSize != 1000 {
		t.Errorf("expected MaxChunkSize 1000, got %d", config.MaxChunkSize)
	}
	if config.MinChunkSize != 100 {
		t.Errorf("expected MinChunkSize 100, got %d", config.MinChunkSize)
	}
	if config.Overlap != 100 {
		t.Errorf("expected Overlap 100, got %d", config.Overlap)
	}
	if !config.RespectBounds {
		t.Error("expected RespectBounds to be true")
	}
}

func TestSplitSentences(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Hello. World.", 2},
		{"One sentence.", 1},
		{"Hello! How are you? I am fine.", 3},
		{"No period at end", 1},
		{"", 0},
	}

	for _, tt := range tests {
		sentences := splitSentences(tt.input)
		if len(sentences) != tt.expected {
			t.Errorf("splitSentences(%q) = %d sentences, expected %d", tt.input, len(sentences), tt.expected)
		}
	}
}

