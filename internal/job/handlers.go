package job

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/embedding"
	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/ingestion"
	jobpayloads "github.com/krc/rag/internal/job/payloads"
	"github.com/krc/rag/internal/common"
	"github.com/krc/rag/pkg/config"
	"github.com/krc/rag/pkg/search"
)

// HandleDocumentIngest handles document ingestion jobs.
// This function will be moved to processor package in Phase 2.
func HandleDocumentIngest(
	ctx context.Context,
	j *Job,
	logger *zap.Logger,
	parserRegistry *ingestion.ParserRegistry,
	chunker chunking.Chunker,
	embedder embedding.Embedder,
	vectorStore index.VectorStore,
	objStore common.ObjectStore,
	events common.EventPublisher,
	cfg *config.Config,
	esClient *search.ElasticsearchClient,
) error {
	logger = logger.With(zap.String("job_id", j.ID))
	logger.Info("processing document ingest job")

	// Parse payload
	var payload jobpayloads.DocumentIngestPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse job payload: %w", err)
	}

	// Resolve input to a local file path for current parsers (they are file-path based).
	// Exactly one of LocalPath or ObjectKey should be set.
	inputPath := payload.LocalPath
	cleanupTemp := func() {}
	if inputPath == "" && payload.ObjectKey != "" {
		if objStore == nil {
			return fmt.Errorf("object_key provided but object store not configured")
		}
		rc, err := objStore.Get(ctx, payload.ObjectKey)
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
		tmp := filepath.Join(tempDir, "obj-"+j.ID+ext)
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

	// Detect format
	format, err := ingestion.DetectFormat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to detect format: %w", err)
	}

	// Get parser
	parser, err := parserRegistry.GetParser(format)
	if err != nil {
		return fmt.Errorf("no parser for format: %w", err)
	}

	j.SetProgressWithStage(10, "parsing", "Parsing document")

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

	j.SetProgressWithStage(30, "checking", "Checking for existing document")

	// Check if document already exists with same hash
	existingHash, err := vectorStore.GetDocumentHash(ctx, doc.ID)
	if err == nil && existingHash == doc.Hash {
		logger.Info("document unchanged, skipping")
		j.SetProgressWithStage(100, "complete", "Document unchanged, skipping")
		return nil
	}

	j.SetProgressWithStage(40, "chunking", "Chunking document")

	// Chunk document
	chunks, err := chunker.Split(doc)
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
	// This ensures document-level info (title, source, format, timestamps) is available during retrieval
	for i := range chunks {
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = make(map[string]string)
		}
		// Copy document metadata to chunks
		for k, v := range doc.Metadata {
			chunks[i].Metadata[k] = v
		}
		// Ensure document-level fields are in chunk metadata
		if storedDoc.Source != "" {
			chunks[i].Metadata["source"] = storedDoc.Source
		}
		if storedDoc.Title != "" {
			chunks[i].Metadata["title"] = storedDoc.Title
		}
		if storedDoc.Format != "" {
			chunks[i].Metadata["format"] = storedDoc.Format
		}
		// Add timestamps to chunk metadata so they're stored in Qdrant
		chunks[i].Metadata["created_at"] = now.Format(time.RFC3339)
		chunks[i].Metadata["updated_at"] = now.Format(time.RFC3339)
		// Also add document-level prefix for easier access
		chunks[i].Metadata["document_source"] = storedDoc.Source
		chunks[i].Metadata["document_title"] = storedDoc.Title
		chunks[i].Metadata["document_format"] = storedDoc.Format
		chunks[i].Metadata["document_created_at"] = now.Format(time.RFC3339)
		chunks[i].Metadata["document_updated_at"] = now.Format(time.RFC3339)
	}

	j.SetProgressWithStage(50, "embedding", fmt.Sprintf("Generating embeddings for %d chunks", len(chunks)))

	// Embed chunks
	if embedder == nil {
		return fmt.Errorf("no embedder configured")
	}

	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	vectors, err := embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to embed %d chunks (progress: 50%%): %w", len(texts), err)
	}

	j.SetProgressWithStage(80, "indexing", "Storing document and chunks")

	// Create chunks with vectors
	chunksWithVectors := make([]chunking.ChunkWithVector, len(chunks))
	for i, chunk := range chunks {
		chunksWithVectors[i] = chunking.ChunkWithVector{
			Chunk:  chunk,
			Vector: vectors[i],
		}
	}

	// Add timestamps to storedDoc metadata so they're stored in Qdrant
	if storedDoc.Metadata == nil {
		storedDoc.Metadata = make(map[string]string)
	}
	storedDoc.Metadata["created_at"] = now.Format(time.RFC3339)
	storedDoc.Metadata["updated_at"] = now.Format(time.RFC3339)
	// Ensure document-level fields are in metadata
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
	if err := vectorStore.StoreDocument(ctx, storedDoc); err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}

	// Store or replace chunks
	if existingHash != "" {
		// Document exists, replace chunks
		if err := vectorStore.ReplaceDocument(ctx, doc.ID, chunksWithVectors); err != nil {
			return fmt.Errorf("failed to replace document chunks: %w", err)
		}
	} else {
		// New document, store chunks
		if err := vectorStore.Store(ctx, chunksWithVectors); err != nil {
			return fmt.Errorf("failed to store chunks: %w", err)
		}
	}

	// Index chunks to Elasticsearch if enabled (best-effort)
	if esClient != nil {
		// Convert chunks to Elasticsearch documents
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
		if err := esClient.BulkIndex(ctx, esDocs); err != nil {
			logger.Warn("failed to index chunks to Elasticsearch", zap.Error(err))
		} else {
			logger.Debug("indexed chunks to Elasticsearch", zap.Int("count", len(chunks)))
		}
	}

	// Best-effort: publish ingest event
	if events != nil && cfg != nil && cfg.Messaging.Kafka.TopicDocumentsIngested != "" {
		_ = events.Publish(ctx, cfg.Messaging.Kafka.TopicDocumentsIngested, doc.ID, map[string]any{
			"document_id": doc.ID,
			"source":      doc.Source,
			"title":       doc.Title,
			"format":      storedDoc.Format,
			"chunks":      len(chunks),
		})
	}

	// Cleanup original object if configured
	if cfg != nil && cfg.Storage.Enabled && cfg.Storage.MinIO.DeleteAfterIngest && objStore != nil && payload.ObjectKey != "" {
		_ = objStore.Delete(ctx, payload.ObjectKey)
	}

	j.SetProgressWithStage(100, "complete", "Document processing completed")
	logger.Info("document ingestion completed",
		zap.String("doc_id", doc.ID),
		zap.Int("chunks", len(chunks)),
	)

	// Clean up temp file if local path was used
	if payload.LocalPath != "" {
		_ = os.Remove(payload.LocalPath)
	}

	return nil
}

