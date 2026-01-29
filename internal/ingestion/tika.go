// Package ingestion provides Apache Tika parser for various document formats.
package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TikaParser parses documents using Apache Tika server.
// Tika supports a wide range of formats: DOCX, XLSX, PPTX, ODT, RTF, HTML, XML, etc.
type TikaParser struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// Config holds Tika parser configuration.
type TikaConfig struct {
	BaseURL string        // Tika server URL (default: http://localhost:9998)
	Timeout time.Duration // Request timeout (default: 60s)
}

// NewTikaParser creates a new Tika parser.
func NewTikaParser(cfg TikaConfig) *TikaParser {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:9998"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &TikaParser{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		timeout: cfg.Timeout,
	}
}

// Parse reads and parses a document using Apache Tika.
func (p *TikaParser) Parse(ctx context.Context, path string) (*Document, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	// Read file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create multipart form
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add file field
	part, err := writer.CreateFormFile("file", path)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Create request with timeout
	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "PUT", p.baseURL+"/tika", &requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "text/plain")

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Tika request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Tika returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read extracted text
	text, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Tika response: %w", err)
	}

	textStr := string(text)

	// Get metadata
	metadata, err := p.getMetadata(ctx, path)
	if err != nil {
		// Metadata extraction is optional, continue without it
		metadata = make(map[string]string)
	}

	// Detect format from content type
	format := p.detectFormatFromMetadata(metadata, path)

	// Create document
	doc := NewDocument(uuid.New().String(), path, format)
	doc.Title = extractTitleFromPath(path)

	// Extract title from metadata if available
	if title, ok := metadata["title"]; ok && title != "" {
		doc.Title = title
	}

	// Parse text into sections
	sections := p.parseTextToSections(textStr)
	doc.Sections = sections

	// Add metadata
	for k, v := range metadata {
		doc.AddMetadata(k, v)
	}

	// Calculate content hash
	doc.Hash = calculateHash(textStr)

	return doc, nil
}

// getMetadata retrieves document metadata from Tika.
func (p *TikaParser) getMetadata(ctx context.Context, path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("file", path)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "PUT", p.baseURL+"/meta", &requestBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tika metadata returned status %d", resp.StatusCode)
	}

	var metadata map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}

	// Convert to string map
	result := make(map[string]string)
	for k, v := range metadata {
		if str, ok := v.(string); ok {
			result[k] = str
		} else if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
			if str, ok := arr[0].(string); ok {
				result[k] = str
			}
		}
	}

	return result, nil
}

// detectFormatFromMetadata detects document format from metadata or file extension.
func (p *TikaParser) detectFormatFromMetadata(metadata map[string]string, path string) DocumentFormat {
	// Try to get content type from metadata
	if contentType, ok := metadata["Content-Type"]; ok {
		contentType = strings.ToLower(contentType)
		if strings.Contains(contentType, "wordprocessingml") || strings.Contains(contentType, "msword") {
			return FormatWord // DOCX, DOC
		}
		if strings.Contains(contentType, "spreadsheetml") || strings.Contains(contentType, "excel") {
			return FormatExcel // XLSX, XLS
		}
		if strings.Contains(contentType, "presentationml") || strings.Contains(contentType, "powerpoint") {
			return FormatPowerPoint // PPTX, PPT
		}
		if strings.Contains(contentType, "html") {
			return FormatHTML
		}
		if strings.Contains(contentType, "xml") {
			return FormatXML
		}
		if strings.Contains(contentType, "rtf") {
			return FormatRTF
		}
	}

	// Fallback to file extension
	ext := getExtension(path)
	switch ext {
	case ".docx", ".doc":
		return FormatWord
	case ".xlsx", ".xls":
		return FormatExcel
	case ".pptx", ".ppt":
		return FormatPowerPoint
	case ".html", ".htm":
		return FormatHTML
	case ".xml":
		return FormatXML
	case ".rtf":
		return FormatRTF
	case ".odt":
		return FormatODT
	default:
		return FormatText // Default fallback
	}
}

// parseTextToSections parses extracted text into sections.
func (p *TikaParser) parseTextToSections(text string) []Section {
	var sections []Section

	// Split by double newlines (paragraphs)
	paragraphs := strings.Split(text, "\n\n")

	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Try to detect headings (lines that are short and don't end with punctuation)
		lines := strings.Split(para, "\n")
		if len(lines) == 1 && len(para) < 100 && !strings.HasSuffix(para, ".") && !strings.HasSuffix(para, ",") {
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
// Tika supports many formats, but we'll list the most common ones.
func (p *TikaParser) SupportedFormats() []DocumentFormat {
	return []DocumentFormat{
		FormatWord,      // DOCX, DOC
		FormatExcel,     // XLSX, XLS
		FormatPowerPoint, // PPTX, PPT
		FormatHTML,      // HTML, HTM
		FormatXML,       // XML
		FormatRTF,       // RTF
		FormatODT,       // ODT
	}
}

