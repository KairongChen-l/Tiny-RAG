//go:build fitz
// +build fitz

package ingestion

import (
	"context"
	"fmt"
	"strings"

	"github.com/gen2brain/go-fitz"
)

// extractWithFitz uses go-fitz library for PDF text extraction.
// This requires CGO and MuPDF library.
// Build with: go build -tags fitz
func (p *PDFParser) extractWithFitz(ctx context.Context, path string) (string, error) {
	doc, err := fitz.New(path)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF with fitz: %w", err)
	}
	defer doc.Close()

	var text strings.Builder
	numPages := doc.NumPage()

	for i := 0; i < numPages; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		pageText, err := doc.Text(i)
		if err != nil {
			continue // Skip page on error
		}

		if text.Len() > 0 {
			text.WriteString("\n\n")
		}
		text.WriteString(pageText)
	}

	return text.String(), nil
}

