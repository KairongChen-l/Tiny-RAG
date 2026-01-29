package ingestion

import (
	"context"
	"fmt"
	"sync"
)

// Parser defines the interface for document parsers.
type Parser interface {
	// Parse reads and parses a document from the given path.
	Parse(ctx context.Context, path string) (*Document, error)

	// SupportedFormats returns the formats this parser can handle.
	SupportedFormats() []DocumentFormat
}

// ParserRegistry manages document parsers by format.
type ParserRegistry struct {
	mu      sync.RWMutex
	parsers map[DocumentFormat]Parser
}

// NewParserRegistry creates a new parser registry.
func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		parsers: make(map[DocumentFormat]Parser),
	}
}

// Register adds a parser to the registry.
func (r *ParserRegistry) Register(parser Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, format := range parser.SupportedFormats() {
		r.parsers[format] = parser
	}
}

// GetParser returns a parser for the given format.
func (r *ParserRegistry) GetParser(format DocumentFormat) (Parser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	parser, ok := r.parsers[format]
	if !ok {
		return nil, fmt.Errorf("no parser registered for format: %s", format)
	}
	return parser, nil
}

// SupportedFormats returns all registered formats.
func (r *ParserRegistry) SupportedFormats() []DocumentFormat {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formats := make([]DocumentFormat, 0, len(r.parsers))
	for format := range r.parsers {
		formats = append(formats, format)
	}
	return formats
}

// DetectFormat detects document format from file extension.
func DetectFormat(filename string) (DocumentFormat, error) {
	ext := getExtension(filename)
	switch ext {
	case ".md", ".markdown":
		return FormatMarkdown, nil
	case ".pdf":
		return FormatPDF, nil
	case ".txt", ".text":
		return FormatText, nil
	case ".docx", ".doc":
		return FormatWord, nil
	case ".xlsx", ".xls":
		return FormatExcel, nil
	case ".pptx", ".ppt":
		return FormatPowerPoint, nil
	case ".html", ".htm":
		return FormatHTML, nil
	case ".xml":
		return FormatXML, nil
	case ".rtf":
		return FormatRTF, nil
	case ".odt":
		return FormatODT, nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// getExtension returns the lowercase file extension including the dot.
func getExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			ext := filename[i:]
			// Convert to lowercase
			result := make([]byte, len(ext))
			for j := 0; j < len(ext); j++ {
				c := ext[j]
				if c >= 'A' && c <= 'Z' {
					c += 'a' - 'A'
				}
				result[j] = c
			}
			return string(result)
		}
		if filename[i] == '/' || filename[i] == '\\' {
			break
		}
	}
	return ""
}

