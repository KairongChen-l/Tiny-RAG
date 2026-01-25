package ingestion

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

// TextParser parses plain text documents.
type TextParser struct{}

// NewTextParser creates a new text parser.
func NewTextParser() *TextParser {
	return &TextParser{}
}

// Parse reads and parses a plain text file.
func (p *TextParser) Parse(ctx context.Context, path string) (*Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create document
	doc := NewDocument(uuid.New().String(), path, FormatText)
	doc.Title = extractTitleFromPath(path)

	// Read content
	var contentBuilder strings.Builder
	var paragraphs []string
	var currentParagraph strings.Builder

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := scanner.Text()
		contentBuilder.WriteString(line)
		contentBuilder.WriteString("\n")

		// Detect paragraph breaks (empty lines)
		if strings.TrimSpace(line) == "" {
			if currentParagraph.Len() > 0 {
				paragraphs = append(paragraphs, currentParagraph.String())
				currentParagraph.Reset()
			}
		} else {
			if currentParagraph.Len() > 0 {
				currentParagraph.WriteString("\n")
			}
			currentParagraph.WriteString(line)
		}
	}

	// Add final paragraph
	if currentParagraph.Len() > 0 {
		paragraphs = append(paragraphs, currentParagraph.String())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Create sections from paragraphs
	for i, para := range paragraphs {
		section := Section{
			Title:   fmt.Sprintf("Paragraph %d", i+1),
			Level:   1,
			Content: para,
		}
		doc.Sections = append(doc.Sections, section)
	}

	// Calculate content hash
	doc.Hash = calculateHash(contentBuilder.String())

	return doc, nil
}

// SupportedFormats returns supported formats.
func (p *TextParser) SupportedFormats() []DocumentFormat {
	return []DocumentFormat{FormatText}
}

