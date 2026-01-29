// Package processor provides task processing abstractions for document ingestion pipeline.
package processor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/common"
	"github.com/krc/rag/internal/embedding"
	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/ingestion"
	jobpayloads "github.com/krc/rag/internal/job/payloads"
	"github.com/krc/rag/pkg/config"
	"github.com/krc/rag/pkg/search"
)

// ProgressCallback is called to report processing progress.
type ProgressCallback func(progress int, stage string, message string)

// TaskProcessor processes document ingestion tasks.
// This interface encapsulates the complete pipeline: parse → chunk → embed → index.
type TaskProcessor interface {
	// Process processes a document ingestion task.
	// progressCallback is optional - if nil, progress updates are skipped.
	Process(ctx context.Context, payload jobpayloads.DocumentIngestPayload, progressCallback ProgressCallback) error
}

// DefaultTaskProcessor is the default implementation of TaskProcessor.
type DefaultTaskProcessor struct {
	parserRegistry *ingestion.ParserRegistry
	chunker        chunking.Chunker
	embedder       embedding.Embedder
	vectorStore    index.VectorStore
	objStore       common.ObjectStore
	events         common.EventPublisher
	esClient       *search.ElasticsearchClient
	cfg            *config.Config
}

// NewDefaultTaskProcessor creates a new default task processor.
func NewDefaultTaskProcessor(
	parserRegistry *ingestion.ParserRegistry,
	chunker chunking.Chunker,
	embedder embedding.Embedder,
	vectorStore index.VectorStore,
	objStore common.ObjectStore,
	events common.EventPublisher,
	esClient *search.ElasticsearchClient,
	cfg *config.Config,
) *DefaultTaskProcessor {
	return &DefaultTaskProcessor{
		parserRegistry: parserRegistry,
		chunker:        chunker,
		embedder:       embedder,
		vectorStore:    vectorStore,
		objStore:       objStore,
		events:         events,
		esClient:       esClient,
		cfg:            cfg,
	}
}

// Process processes a document ingestion task.
// This implements the complete pipeline: parse → chunk → embed → index.
// The logic is migrated from job.HandleDocumentIngest.
func (p *DefaultTaskProcessor) Process(ctx context.Context, payload jobpayloads.DocumentIngestPayload, progressCallback ProgressCallback) error {
	// Helper function to report progress
	reportProgress := func(progress int, stage string, message string) {
		if progressCallback != nil {
			progressCallback(progress, stage, message)
		}
	}

	// Resolve input to a local file path for current parsers (they are file-path based).
	// Exactly one of LocalPath or ObjectKey should be set.
	inputPath := payload.LocalPath
	cleanupTemp := func() {}
	if inputPath == "" && payload.ObjectKey != "" {
		if p.objStore == nil {
			return fmt.Errorf("object_key provided but object store not configured")
		}
		rc, err := p.objStore.Get(ctx, payload.ObjectKey)
		if err != nil {
			return fmt.Errorf("failed to download object %q: %w", payload.ObjectKey, err)
		}
		defer rc.Close()

		tempDir := filepath.Join(os.TempDir(), "rag-uploads")
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}

		ext := filepath.Ext(payload.Filename)
		if ext == "" {
			ext = ".bin"
		}
		// Use a unique ID for temp file
		docID := payload.Metadata["document_id"]
		if docID == "" {
			docID = fmt.Sprintf("doc-%d", time.Now().UnixNano())
		}
		tmp := filepath.Join(tempDir, "obj-"+docID+ext)
		f, err := os.Create(tmp)
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		if _, err := io.Copy(f, rc); err != nil {
			f.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("failed to write temp file: %w", err)
		}
		_ = f.Close()

		inputPath = tmp
		cleanupTemp = func() { _ = os.Remove(tmp) }
	}
	defer cleanupTemp()

	if inputPath == "" {
		return fmt.Errorf("invalid payload: both local_path and object_key are empty")
	}

	reportProgress(10, "parsing", "Parsing document")

	// Detect format
	format, err := ingestion.DetectFormat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to detect format: %w", err)
	}

	// Get parser
	parser, err := p.parserRegistry.GetParser(format)
	if err != nil {
		return fmt.Errorf("no parser for format: %w", err)
	}

	// Parse document
	doc, err := parser.Parse(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("failed to parse document: %w", err)
	}

	// Add metadata
	for k, v := range payload.Metadata {
		doc.AddMetadata(k, v)
	}

	// Set document title from metadata if available
	if title, ok := payload.Metadata["title"]; ok && title != "" {
		doc.Title = title
	}

	reportProgress(30, "checking", "Checking for existing document")

	// Check if document already exists with same hash
	existingHash, err := p.vectorStore.GetDocumentHash(ctx, doc.ID)
	if err == nil && existingHash == doc.Hash {
		reportProgress(100, "complete", "Document unchanged, skipping")
		return nil
	}

	reportProgress(40, "chunking", "Chunking document")

	// Chunk document
	chunks, err := p.chunker.Split(doc)
	if err != nil {
		return fmt.Errorf("failed to chunk document: %w", err)
	}

	// Store document first to get timestamps
	now := time.Now()
	storedDoc := &index.StoredDocument{
		ID:        doc.ID,
		Source:    doc.Source,
		Title:     doc.Title,
		Format:    string(doc.Format),
		Hash:      doc.Hash,
		Metadata:  doc.Metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Add document metadata to all chunks
	for i := range chunks {
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = make(map[string]string)
		}
		for k, v := range doc.Metadata {
			chunks[i].Metadata[k] = v
		}
		if storedDoc.Source != "" {
			chunks[i].Metadata["source"] = storedDoc.Source
		}
		if storedDoc.Title != "" {
			chunks[i].Metadata["title"] = storedDoc.Title
		}
		if storedDoc.Format != "" {
			chunks[i].Metadata["format"] = storedDoc.Format
		}
		chunks[i].Metadata["created_at"] = now.Format(time.RFC3339)
		chunks[i].Metadata["updated_at"] = now.Format(time.RFC3339)
		chunks[i].Metadata["document_source"] = storedDoc.Source
		chunks[i].Metadata["document_title"] = storedDoc.Title
		chunks[i].Metadata["document_format"] = storedDoc.Format
		chunks[i].Metadata["document_created_at"] = now.Format(time.RFC3339)
		chunks[i].Metadata["document_updated_at"] = now.Format(time.RFC3339)
	}

	reportProgress(50, "embedding", fmt.Sprintf("Generating embeddings for %d chunks", len(chunks)))

	// Embed chunks
	if p.embedder == nil {
		return fmt.Errorf("no embedder configured")
	}

	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	vectors, err := p.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to embed %d chunks: %w", len(texts), err)
	}

	reportProgress(80, "indexing", "Storing document and chunks")

	// Create chunks with vectors
	chunksWithVectors := make([]chunking.ChunkWithVector, len(chunks))
	for i, chunk := range chunks {
		chunksWithVectors[i] = chunking.ChunkWithVector{
			Chunk:  chunk,
			Vector: vectors[i],
		}
	}

	// Add timestamps to storedDoc metadata
	if storedDoc.Metadata == nil {
		storedDoc.Metadata = make(map[string]string)
	}
	storedDoc.Metadata["created_at"] = now.Format(time.RFC3339)
	storedDoc.Metadata["updated_at"] = now.Format(time.RFC3339)
	if storedDoc.Source != "" {
		storedDoc.Metadata["source"] = storedDoc.Source
	}
	if storedDoc.Title != "" {
		storedDoc.Metadata["title"] = storedDoc.Title
	}
	if storedDoc.Format != "" {
		storedDoc.Metadata["format"] = storedDoc.Format
	}

	// Store document metadata
	if err := p.vectorStore.StoreDocument(ctx, storedDoc); err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}

	// Store or replace chunks
	if existingHash != "" {
		if err := p.vectorStore.ReplaceDocument(ctx, doc.ID, chunksWithVectors); err != nil {
			return fmt.Errorf("failed to replace document chunks: %w", err)
		}
	} else {
		if err := p.vectorStore.Store(ctx, chunksWithVectors); err != nil {
			return fmt.Errorf("failed to store chunks: %w", err)
		}
	}

	// Index chunks to Elasticsearch if enabled
	if p.esClient != nil {
		esDocs := make([]search.Document, len(chunks))
		for i, chunk := range chunks {
			esDocs[i] = search.Document{
				ID:          chunk.ID,
				DocumentID:  chunk.DocumentID,
				Content:     chunk.Content,
				Title:       chunk.Metadata["title"],
				SectionPath: chunk.SectionPath,
				Metadata:    chunk.Metadata,
			}
		}
		if err := p.esClient.BulkIndex(ctx, esDocs); err != nil {
			// Log but don't fail - best effort
			_ = err
		}
	}

	// Publish ingest event
	if p.events != nil && p.cfg != nil && p.cfg.Messaging.Kafka.TopicDocumentsIngested != "" {
		_ = p.events.Publish(ctx, p.cfg.Messaging.Kafka.TopicDocumentsIngested, doc.ID, map[string]any{
			"document_id": doc.ID,
			"source":      doc.Source,
			"title":       doc.Title,
			"format":      storedDoc.Format,
			"chunks":      len(chunks),
		})
	}

	// Cleanup original object if configured
	if p.cfg != nil && p.cfg.Storage.Enabled && p.cfg.Storage.MinIO.DeleteAfterIngest && p.objStore != nil && payload.ObjectKey != "" {
		_ = p.objStore.Delete(ctx, payload.ObjectKey)
	}

	reportProgress(100, "complete", "Document processing completed")

	// Clean up temp file if local path was used
	if payload.LocalPath != "" {
		_ = os.Remove(payload.LocalPath)
	}

	return nil
}


