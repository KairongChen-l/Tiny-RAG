package ingestion

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// MarkdownParser parses Markdown documents.
type MarkdownParser struct{}

// NewMarkdownParser creates a new Markdown parser.
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{}
}

// Parse reads and parses a Markdown file.
func (p *MarkdownParser) Parse(ctx context.Context, path string) (*Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create document
	doc := NewDocument(uuid.New().String(), path, FormatMarkdown)
	doc.Title = extractTitleFromPath(path)

	// Read and parse content
	var contentBuilder strings.Builder
	var currentSection *Section
	var sectionStack []*Section

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

		// Check for heading
		if strings.HasPrefix(line, "#") {
			level, title := parseHeading(line)
			if level > 0 {
				newSection := &Section{
					Title:    title,
					Level:    level,
					Children: []Section{},
				}

				// Find parent section
				for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1].Level >= level {
					sectionStack = sectionStack[:len(sectionStack)-1]
				}

				if len(sectionStack) == 0 {
					doc.Sections = append(doc.Sections, *newSection)
					currentSection = &doc.Sections[len(doc.Sections)-1]
				} else {
					parent := sectionStack[len(sectionStack)-1]
					parent.Children = append(parent.Children, *newSection)
					currentSection = &parent.Children[len(parent.Children)-1]
				}

				sectionStack = append(sectionStack, currentSection)
				continue
			}
		}

		// Add content to current section
		if currentSection != nil && line != "" {
			if currentSection.Content != "" {
				currentSection.Content += "\n"
			}
			currentSection.Content += line
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Set title from first H1 if available
	if len(doc.Sections) > 0 && doc.Sections[0].Level == 1 {
		doc.Title = doc.Sections[0].Title
	}

	// Calculate content hash
	doc.Hash = calculateHash(contentBuilder.String())

	return doc, nil
}

// SupportedFormats returns supported formats.
func (p *MarkdownParser) SupportedFormats() []DocumentFormat {
	return []DocumentFormat{FormatMarkdown}
}

// parseHeading extracts heading level and title from a Markdown heading line.
func parseHeading(line string) (int, string) {
	level := 0
	for _, c := range line {
		if c == '#' {
			level++
		} else {
			break
		}
	}
	
	if level == 0 || level > 6 {
		return 0, ""
	}

	title := strings.TrimSpace(strings.TrimLeft(line, "#"))
	return level, title
}

// extractTitleFromPath extracts a title from the file path.
func extractTitleFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// calculateHash calculates SHA256 hash of content.
func calculateHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

