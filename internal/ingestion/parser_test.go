package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkdownParser(t *testing.T) {
	// Create temp markdown file
	content := `# Main Title

This is the introduction paragraph.

## Section 1

Content of section 1.

### Subsection 1.1

Content of subsection 1.1.

## Section 2

Content of section 2.
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewMarkdownParser()
	doc, err := parser.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify document
	if doc.Title != "Main Title" {
		t.Errorf("expected title 'Main Title', got '%s'", doc.Title)
	}
	if doc.Format != FormatMarkdown {
		t.Errorf("expected format 'markdown', got '%s'", doc.Format)
	}
	if doc.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if len(doc.Sections) == 0 {
		t.Error("expected at least one section")
	}
}

func TestTextParser(t *testing.T) {
	content := `First paragraph of the document.
This continues the first paragraph.

Second paragraph starts here.
And continues here.

Third paragraph.
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewTextParser()
	doc, err := parser.Parse(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify document
	if doc.Format != FormatText {
		t.Errorf("expected format 'text', got '%s'", doc.Format)
	}
	if len(doc.Sections) != 3 {
		t.Errorf("expected 3 sections (paragraphs), got %d", len(doc.Sections))
	}
}

func TestParserRegistry(t *testing.T) {
	registry := NewParserRegistry()

	// Register parsers
	registry.Register(NewMarkdownParser())
	registry.Register(NewTextParser())

	// Get markdown parser
	parser, err := registry.GetParser(FormatMarkdown)
	if err != nil {
		t.Fatalf("GetParser(markdown) failed: %v", err)
	}
	formats := parser.SupportedFormats()
	if len(formats) == 0 || formats[0] != FormatMarkdown {
		t.Error("expected markdown parser")
	}

	// Get text parser
	parser, err = registry.GetParser(FormatText)
	if err != nil {
		t.Fatalf("GetParser(text) failed: %v", err)
	}
	formats = parser.SupportedFormats()
	if len(formats) == 0 || formats[0] != FormatText {
		t.Error("expected text parser")
	}

	// Get unsupported format
	_, err = registry.GetParser("unsupported")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		filename string
		want     DocumentFormat
		wantErr  bool
	}{
		{"test.md", FormatMarkdown, false},
		{"test.markdown", FormatMarkdown, false},
		{"test.txt", FormatText, false},
		{"test.text", FormatText, false},
		{"test.pdf", FormatPDF, false},
		{"test.unknown", "", true},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got, err := DetectFormat(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectFormat(%s) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DetectFormat(%s) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestDocumentFlatten(t *testing.T) {
	doc := &Document{
		ID:    "doc1",
		Title: "Test",
		Sections: []Section{
			{
				Title:   "Chapter 1",
				Level:   1,
				Content: "Intro",
				Children: []Section{
					{
						Title:   "Section 1.1",
						Level:   2,
						Content: "Content 1.1",
					},
				},
			},
			{
				Title:   "Chapter 2",
				Level:   1,
				Content: "Content 2",
			},
		},
	}

	flat := doc.Flatten()
	if len(flat) != 3 {
		t.Errorf("expected 3 flat sections, got %d", len(flat))
	}

	// Check paths
	expectedPaths := []string{"Chapter 1", "Chapter 1/Section 1.1", "Chapter 2"}
	for i, f := range flat {
		if f.Path != expectedPaths[i] {
			t.Errorf("expected path '%s', got '%s'", expectedPaths[i], f.Path)
		}
	}
}

