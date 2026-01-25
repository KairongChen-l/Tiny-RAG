package ingestion

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/uuid"
)

// PDFParser parses PDF documents.
// It uses external tools (pdftotext) for extraction as pure Go PDF libraries
// have limitations with complex PDFs.
type PDFParser struct {
	// UsePdftotext indicates whether to use the pdftotext command.
	// If false, will attempt to use a Go library (limited support).
	UsePdftotext bool
}

// NewPDFParser creates a new PDF parser.
func NewPDFParser() *PDFParser {
	return &PDFParser{
		UsePdftotext: true, // Default to pdftotext for better results
	}
}

// Parse reads and parses a PDF file.
func (p *PDFParser) Parse(ctx context.Context, path string) (*Document, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	var text string
	var err error

	if p.UsePdftotext {
		text, err = p.extractWithPdftotext(ctx, path)
	} else {
		text, err = p.extractWithGoLib(ctx, path)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to extract PDF text: %w", err)
	}

	// Create document
	doc := NewDocument(uuid.New().String(), path, FormatPDF)
	doc.Title = extractTitleFromPath(path)

	// Parse text into sections (by page or paragraph)
	sections := p.parseTextToSections(text)
	doc.Sections = sections

	// Calculate content hash
	doc.Hash = calculateHash(text)

	return doc, nil
}

// extractWithPdftotext uses the pdftotext command-line tool.
func (p *PDFParser) extractWithPdftotext(ctx context.Context, path string) (string, error) {
	// Check if pdftotext is available
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("pdftotext not found, please install poppler-utils")
	}

	// Run pdftotext
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", path, "-")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed: %w", err)
	}

	return string(output), nil
}

// extractWithGoLib attempts to extract text using a Go library.
// This is a fallback with limited support.
func (p *PDFParser) extractWithGoLib(ctx context.Context, path string) (string, error) {
	// Read file content for basic text extraction
	// Note: This is a simplified implementation. For production use,
	// consider using github.com/ledongthuc/pdf or similar.
	
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Very basic text extraction from PDF
	// This won't work well for most PDFs but serves as a fallback
	text := extractBasicPDFText(content)
	if text == "" {
		return "", fmt.Errorf("could not extract text from PDF, pdftotext is recommended")
	}

	return text, nil
}

// extractBasicPDFText attempts basic text extraction from PDF bytes.
// This is very limited and mainly for demonstration.
func extractBasicPDFText(content []byte) string {
	// Look for text streams in PDF
	// This is a very naive implementation
	str := string(content)
	var text strings.Builder

	// Find text between BT and ET markers (very simplified)
	inText := false
	for i := 0; i < len(str)-1; i++ {
		if str[i] == 'B' && str[i+1] == 'T' {
			inText = true
			continue
		}
		if str[i] == 'E' && str[i+1] == 'T' {
			inText = false
			text.WriteString(" ")
			continue
		}
		if inText {
			c := str[i]
			if c >= 32 && c < 127 {
				text.WriteByte(c)
			}
		}
	}

	return strings.TrimSpace(text.String())
}

// parseTextToSections parses extracted text into sections.
func (p *PDFParser) parseTextToSections(text string) []Section {
	var sections []Section

	// Split by double newlines (paragraphs)
	paragraphs := strings.Split(text, "\n\n")
	
	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Try to detect if this is a heading
		lines := strings.Split(para, "\n")
		if len(lines) == 1 && len(para) < 100 && !strings.HasSuffix(para, ".") {
			// Might be a heading
			sections = append(sections, Section{
				Title:   para,
				Level:   1,
				Content: "",
			})
		} else {
			// Regular paragraph
			sections = append(sections, Section{
				Title:   fmt.Sprintf("Section %d", i+1),
				Level:   1,
				Content: para,
			})
		}
	}

	// If no sections were created, create one with all content
	if len(sections) == 0 {
		sections = append(sections, Section{
			Title:   "Content",
			Level:   1,
			Content: text,
		})
	}

	return sections
}

// SupportedFormats returns supported formats.
func (p *PDFParser) SupportedFormats() []DocumentFormat {
	return []DocumentFormat{FormatPDF}
}

