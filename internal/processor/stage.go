// Package processor provides pipeline stage interfaces for document processing.
package processor

import (
	"context"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/ingestion"
)

// ProcessorStage represents a single stage in the document processing pipeline.
type ProcessorStage interface {
	// Process processes the input and returns output for the next stage.
	Process(ctx context.Context, input StageInput) (StageOutput, error)
}

// StageInput represents input to a processing stage.
type StageInput struct {
	DocumentID string
	Content    []byte
	FilePath   string // For file-based parsers
	Metadata   map[string]string
}

// StageOutput represents output from a processing stage.
type StageOutput struct {
	Document   *ingestion.Document
	Chunks     []chunking.Chunk
	Embeddings [][]float32
	Metadata   map[string]string
}

// ParserStage processes the parsing stage.
type ParserStage struct {
	parserRegistry *ingestion.ParserRegistry
}

// NewParserStage creates a new parser stage.
func NewParserStage(parserRegistry *ingestion.ParserRegistry) *ParserStage {
	return &ParserStage{
		parserRegistry: parserRegistry,
	}
}

// Process processes the parsing stage.
func (s *ParserStage) Process(ctx context.Context, input StageInput) (StageOutput, error) {
	// Detect format
	format, err := ingestion.DetectFormat(input.FilePath)
	if err != nil {
		return StageOutput{}, err
	}

	// Get parser
	parser, err := s.parserRegistry.GetParser(format)
	if err != nil {
		return StageOutput{}, err
	}

	// Parse document
	doc, err := parser.Parse(ctx, input.FilePath)
	if err != nil {
		return StageOutput{}, err
	}

	// Add metadata
	for k, v := range input.Metadata {
		doc.AddMetadata(k, v)
	}

	return StageOutput{
		Document: doc,
		Metadata: input.Metadata,
	}, nil
}

