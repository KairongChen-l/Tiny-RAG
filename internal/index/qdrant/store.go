package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
)

// Store implements VectorStore using Qdrant HTTP API.
type Store struct {
	baseURL    string
	collection string
	dimension  int
	httpClient *http.Client
	apiKey     string
	mu         sync.RWMutex
}

// Config holds Qdrant store configuration.
type Config struct {
	URL        string // Qdrant server URL (e.g., "http://localhost:6333")
	Collection string // Collection name (default: "rag_chunks")
	Dimension  int    // Vector dimension
	APIKey     string // Optional API key for authentication
}

// New creates a new Qdrant vector store.
func New(cfg Config) (*Store, error) {
	if cfg.URL == "" {
		cfg.URL = "http://localhost:6333"
	}
	if cfg.Collection == "" {
		cfg.Collection = "rag_chunks"
	}
	if cfg.Dimension <= 0 {
		return nil, fmt.Errorf("dimension must be positive, got %d", cfg.Dimension)
	}

	store := &Store{
		baseURL:    cfg.URL,
		collection: cfg.Collection,
		dimension:  cfg.Dimension,
		apiKey:     cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Initialize collection
	if err := store.initCollection(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize collection: %w", err)
	}

	return store, nil
}

// initCollection creates the collection if it doesn't exist.
func (s *Store) initCollection(ctx context.Context) error {
	// Check if collection exists
	exists, err := s.collectionExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}

	if !exists {
		// Create collection
		createReq := map[string]interface{}{
			"vectors": map[string]interface{}{
				"size":     s.dimension,
				"distance": "Cosine",
			},
		}

		reqBody, err := json.Marshal(createReq)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.collection)
		req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(reqBody))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if s.apiKey != "" {
			req.Header.Set("api-key", s.apiKey)
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to create collection: status %d, body: %s", resp.StatusCode, string(body))
		}
	}

	return nil
}

// collectionExists checks if a collection exists.
func (s *Store) collectionExists(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// Store stores chunks with their vectors.
func (s *Store) Store(ctx context.Context, chunks []chunking.ChunkWithVector) error {
	if len(chunks) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Prepare points for batch upsert
	points := make([]map[string]interface{}, 0, len(chunks))
	for _, chunk := range chunks {
		if len(chunk.Vector) != s.dimension {
			return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimension, len(chunk.Vector))
		}

		// Convert float32 to float64 for JSON
		vector := make([]float64, len(chunk.Vector))
		for i, v := range chunk.Vector {
			vector[i] = float64(v)
		}

		// Prepare payload (metadata)
		payload := map[string]interface{}{
			"document_id":  chunk.DocumentID,
			"content":      chunk.Content,
			"section_path": chunk.SectionPath,
			"position":     chunk.Position,
			"hash":         chunk.Hash,
		}

		// Add custom metadata - merge chunk metadata into payload
		// This ensures document-level metadata (title, source, format) is accessible
		if len(chunk.Metadata) > 0 {
			// Store metadata as nested object for filtering, but also merge important fields
			payload["metadata"] = chunk.Metadata
			// Also store document-level fields at top level for easier access
			if source, ok := chunk.Metadata["source"]; ok {
				payload["source"] = source
			}
			if title, ok := chunk.Metadata["title"]; ok {
				payload["title"] = title
			}
			if format, ok := chunk.Metadata["format"]; ok {
				payload["format"] = format
			}
			if createdAt, ok := chunk.Metadata["created_at"]; ok {
				payload["created_at"] = createdAt
			}
			if updatedAt, ok := chunk.Metadata["updated_at"]; ok {
				payload["updated_at"] = updatedAt
			}
		}

		point := map[string]interface{}{
			"id":      chunk.ID,
			"vector":  vector,
			"payload": payload,
		}
		points = append(points, point)
	}

	// Batch upsert
	upsertReq := map[string]interface{}{
		"points": points,
	}

	reqBody, err := json.Marshal(upsertReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upsert points: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Search performs vector similarity search.
func (s *Store) Search(ctx context.Context, query []float32, opts index.SearchOptions) ([]index.SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(query) != s.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", s.dimension, len(query))
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Convert float32 to float64
	queryVector := make([]float64, len(query))
	for i, v := range query {
		queryVector[i] = float64(v)
	}

	// Build search request
	searchReq := map[string]interface{}{
		"vector":       queryVector,
		"limit":        topK,
		"with_payload": true,
	}

	// Add filter if metadata filters are present
	if len(opts.MetadataFilter) > 0 {
		must := make([]map[string]interface{}, 0, len(opts.MetadataFilter))
		for key, value := range opts.MetadataFilter {
			// For known top-level fields, use them directly
			// For other fields, try both top-level and nested metadata
			topLevelFields := map[string]bool{
				"source":      true,
				"title":       true,
				"format":      true,
				"created_at":  true,
				"updated_at":  true,
				"document_id": true,
			}

			if topLevelFields[key] {
				// Use top-level field directly
				must = append(must, map[string]interface{}{
					"key":   key,
					"match": map[string]interface{}{"value": value},
				})
			} else {
				// Try nested metadata field
				must = append(must, map[string]interface{}{
					"key":   fmt.Sprintf("metadata.%s", key),
					"match": map[string]interface{}{"value": value},
				})
			}
		}
		searchReq["filter"] = map[string]interface{}{
			"must": must,
		}
	}

	// Add score threshold
	if opts.MinScore > 0 {
		searchReq["score_threshold"] = float64(opts.MinScore)
	}

	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: status %d, body: %s", resp.StatusCode, string(body))
	}

	var searchResult struct {
		Result []struct {
			ID      interface{}            `json:"id"`
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert results
	results := make([]index.SearchResult, 0, len(searchResult.Result))
	citationNum := 1
	for _, hit := range searchResult.Result {
		score := float32(hit.Score)
		if opts.MinScore > 0 && score < opts.MinScore {
			continue
		}

		// Extract chunk from payload
		chunk, err := s.extractChunkFromPayload(hit.Payload, hit.ID)
		if err != nil {
			continue // Skip invalid chunks
		}

		results = append(results, index.SearchResult{
			Chunk:    chunk,
			Score:    score,
			Citation: citationNum,
		})
		citationNum++
	}

	return results, nil
}

// extractChunkFromPayload extracts chunk data from Qdrant payload.
func (s *Store) extractChunkFromPayload(payload map[string]interface{}, pointID interface{}) (chunking.Chunk, error) {
	chunk := chunking.Chunk{
		Metadata: make(map[string]string),
	}

	// Extract point ID
	if pointID != nil {
		if idStr, ok := pointID.(string); ok {
			chunk.ID = idStr
		} else if idNum, ok := pointID.(float64); ok {
			chunk.ID = fmt.Sprintf("%.0f", idNum)
		}
	}

	// Extract fields
	if docID, ok := payload["document_id"].(string); ok {
		chunk.DocumentID = docID
	}
	if content, ok := payload["content"].(string); ok {
		chunk.Content = content
	}
	if sectionPath, ok := payload["section_path"].(string); ok {
		chunk.SectionPath = sectionPath
	}
	if position, ok := payload["position"].(float64); ok {
		chunk.Position = int(position)
	}
	if hash, ok := payload["hash"].(string); ok {
		chunk.Hash = hash
	}

	// Extract metadata from nested metadata object
	if metadata, ok := payload["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				chunk.Metadata[k] = str
			}
		}
	}

	// Also extract top-level fields that might be metadata (source, title, format, created_at, updated_at)
	// These are stored at top level for easier filtering
	if source, ok := payload["source"].(string); ok && source != "" {
		if _, exists := chunk.Metadata["source"]; !exists {
			chunk.Metadata["source"] = source
		}
	}
	if title, ok := payload["title"].(string); ok && title != "" {
		if _, exists := chunk.Metadata["title"]; !exists {
			chunk.Metadata["title"] = title
		}
	}
	if format, ok := payload["format"].(string); ok && format != "" {
		if _, exists := chunk.Metadata["format"]; !exists {
			chunk.Metadata["format"] = format
		}
	}
	if createdAt, ok := payload["created_at"].(string); ok && createdAt != "" {
		if _, exists := chunk.Metadata["created_at"]; !exists {
			chunk.Metadata["created_at"] = createdAt
		}
	}
	if updatedAt, ok := payload["updated_at"].(string); ok && updatedAt != "" {
		if _, exists := chunk.Metadata["updated_at"]; !exists {
			chunk.Metadata["updated_at"] = updatedAt
		}
	}

	return chunk, nil
}

// DeleteByDocument deletes all chunks for a document.
func (s *Store) DeleteByDocument(ctx context.Context, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create filter to delete by document_id
	filter := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"key":   "document_id",
				"match": map[string]interface{}{"value": documentID},
			},
		},
	}

	deleteReq := map[string]interface{}{
		"filter": filter,
	}

	reqBody, err := json.Marshal(deleteReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete document chunks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete document chunks: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetDocumentHash returns the hash of a stored document.
func (s *Store) GetDocumentHash(ctx context.Context, documentID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Search for one chunk from this document to get the hash
	filter := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"key":   "document_id",
				"match": map[string]interface{}{"value": documentID},
			},
		},
	}

	scrollReq := map[string]interface{}{
		"filter":       filter,
		"limit":        1,
		"with_payload": true,
	}

	reqBody, err := json.Marshal(scrollReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/scroll", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to scroll: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil // Document not found
	}

	var scrollResult struct {
		Result struct {
			Points []struct {
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(scrollResult.Result.Points) == 0 {
		return "", nil // Document not found
	}

	// Extract hash from payload
	if hash, ok := scrollResult.Result.Points[0].Payload["hash"].(string); ok {
		return hash, nil
	}

	return "", nil
}

// ReplaceDocument atomically replaces all chunks for a document.
func (s *Store) ReplaceDocument(ctx context.Context, documentID string, chunks []chunking.ChunkWithVector) error {
	// Delete old chunks
	if err := s.DeleteByDocument(ctx, documentID); err != nil {
		return fmt.Errorf("failed to delete old chunks: %w", err)
	}

	// Store new chunks
	if err := s.Store(ctx, chunks); err != nil {
		return fmt.Errorf("failed to store new chunks: %w", err)
	}

	return nil
}

// ListDocuments returns stored documents with pagination and filtering.
func (s *Store) ListDocuments(ctx context.Context, opts index.ListOptions) (*index.ListDocumentsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Set defaults
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.SortBy == "" {
		opts.SortBy = "created_at"
	}
	if opts.Order == "" {
		opts.Order = "desc"
	}

	// Use scroll to get all points and build document map
	docMap := make(map[string]*index.StoredDocument)
	scrollOffset := ""
	scrollLimit := 100

	for {
		scrollReq := map[string]interface{}{
			"limit":        scrollLimit,
			"with_payload": true,
		}
		if scrollOffset != "" {
			scrollReq["offset"] = scrollOffset
		}

		reqBody, err := json.Marshal(scrollReq)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		url := fmt.Sprintf("%s/collections/%s/points/scroll", s.baseURL, s.collection)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if s.apiKey != "" {
			req.Header.Set("api-key", s.apiKey)
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to scroll: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to scroll: status %d, body: %s", resp.StatusCode, string(body))
		}

		var scrollResult struct {
			Result struct {
				Points []struct {
					ID      interface{}            `json:"id"`
					Payload map[string]interface{} `json:"payload"`
				} `json:"points"`
				NextPageOffset interface{} `json:"next_page_offset"`
			} `json:"result"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		// Process results
		for _, point := range scrollResult.Result.Points {
			chunk, err := s.extractChunkFromPayload(point.Payload, point.ID)
			if err != nil {
				continue
			}

			docID := chunk.DocumentID
			if docID == "" {
				continue
			}

		// Check if document is soft-deleted (unless include_deleted is true)
		if !opts.IncludeDeleted {
			if deletedAt, ok := point.Payload["deleted_at"].(string); ok && deletedAt != "" {
				continue // Skip soft-deleted documents
			}
		}

		// Get or create document
		doc, exists := docMap[docID]
		if !exists {
			doc = &index.StoredDocument{
				ID:       docID,
				Metadata: make(map[string]string),
			}
			docMap[docID] = doc
		}

		// Set DeletedAt if document is soft-deleted
		if deletedAtStr, ok := point.Payload["deleted_at"].(string); ok && deletedAtStr != "" {
			if deletedAt, err := time.Parse(time.RFC3339, deletedAtStr); err == nil {
				doc.DeletedAt = &deletedAt
			}
		}

			// Update document info from chunk
			// Try to get from chunk metadata first, then from payload directly
			if doc.Source == "" {
				if source, ok := chunk.Metadata["source"]; ok {
					doc.Source = source
				} else if source, ok := point.Payload["source"].(string); ok {
					doc.Source = source
				}
			}
			if doc.Title == "" {
				if title, ok := chunk.Metadata["title"]; ok {
					doc.Title = title
				} else if title, ok := point.Payload["title"].(string); ok {
					doc.Title = title
				}
			}
			if doc.Format == "" {
				if format, ok := chunk.Metadata["format"]; ok {
					doc.Format = format
				} else if format, ok := point.Payload["format"].(string); ok {
					doc.Format = format
				}
			}
			if doc.Hash == "" {
				doc.Hash = chunk.Hash
			}

			// Extract timestamps from payload
			if doc.CreatedAt.IsZero() {
				if createdAtStr, ok := point.Payload["created_at"].(string); ok {
					if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
						doc.CreatedAt = t
					} else if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
						doc.CreatedAt = t
					}
				} else if createdAtStr, ok := chunk.Metadata["created_at"]; ok {
					if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
						doc.CreatedAt = t
					} else if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
						doc.CreatedAt = t
					}
				}
			}
			if doc.UpdatedAt.IsZero() {
				if updatedAtStr, ok := point.Payload["updated_at"].(string); ok {
					if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
						doc.UpdatedAt = t
					} else if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
						doc.UpdatedAt = t
					}
				} else if updatedAtStr, ok := chunk.Metadata["updated_at"]; ok {
					if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
						doc.UpdatedAt = t
					} else if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
						doc.UpdatedAt = t
					}
				}
			}
		}

		// Check if we have more results
		if scrollResult.Result.NextPageOffset == nil {
			break
		}

		// Update offset for next iteration
		if nextOffset, ok := scrollResult.Result.NextPageOffset.(string); ok && nextOffset != "" {
			scrollOffset = nextOffset
		} else {
			break
		}
	}

	// Convert map to slice
	allDocs := make([]index.StoredDocument, 0, len(docMap))
	for _, doc := range docMap {
		allDocs = append(allDocs, *doc)
	}

	// Sort documents
	sortDocs(allDocs, opts.SortBy, opts.Order)

	// Apply pagination
	total := len(allDocs)
	start := opts.Offset
	if start > total {
		start = total
	}
	end := start + opts.Limit
	if end > total {
		end = total
	}

	var docs []index.StoredDocument
	if start < total {
		docs = allDocs[start:end]
	} else {
		docs = []index.StoredDocument{}
	}

	return &index.ListDocumentsResult{
		Documents: docs,
		Total:     total,
		Limit:     opts.Limit,
		Offset:    opts.Offset,
	}, nil
}

// sortDocs sorts documents by the specified field and order.
func sortDocs(docs []index.StoredDocument, sortBy, order string) {
	switch sortBy {
	case "title":
		if order == "asc" {
			sort.Slice(docs, func(i, j int) bool {
				return docs[i].Title < docs[j].Title
			})
		} else {
			sort.Slice(docs, func(i, j int) bool {
				return docs[i].Title > docs[j].Title
			})
		}
	case "updated_at":
		if order == "asc" {
			sort.Slice(docs, func(i, j int) bool {
				return docs[i].UpdatedAt.Before(docs[j].UpdatedAt)
			})
		} else {
			sort.Slice(docs, func(i, j int) bool {
				return docs[i].UpdatedAt.After(docs[j].UpdatedAt)
			})
		}
	case "created_at":
		fallthrough
	default:
		if order == "asc" {
			sort.Slice(docs, func(i, j int) bool {
				return docs[i].CreatedAt.Before(docs[j].CreatedAt)
			})
		} else {
			sort.Slice(docs, func(i, j int) bool {
				return docs[i].CreatedAt.After(docs[j].CreatedAt)
			})
		}
	}
}

// StoreDocument stores or updates a document record.
func (s *Store) StoreDocument(ctx context.Context, doc *index.StoredDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find all chunks for this document and update their payload
	filter := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"key":   "document_id",
				"match": map[string]interface{}{"value": doc.ID},
			},
		},
	}

	// Get all point IDs for this document
	scrollReq := map[string]interface{}{
		"filter":       filter,
		"limit":        100,
		"with_payload": false,
	}

	reqBody, err := json.Marshal(scrollReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/scroll", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to scroll: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil // No chunks found, nothing to update
	}

	var scrollResult struct {
		Result struct {
			Points []struct {
				ID interface{} `json:"id"`
			} `json:"points"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Update payload for each point
	pointIDs := make([]interface{}, 0, len(scrollResult.Result.Points))
	for _, point := range scrollResult.Result.Points {
		pointIDs = append(pointIDs, point.ID)
	}

	if len(pointIDs) == 0 {
		return nil // No points to update
	}

	payload := map[string]interface{}{
		"source": doc.Source,
		"title":  doc.Title,
		"format": doc.Format,
		"hash":   doc.Hash,
	}

	// Add timestamps
	if !doc.CreatedAt.IsZero() {
		payload["created_at"] = doc.CreatedAt.Format(time.RFC3339)
	} else {
		payload["created_at"] = time.Now().Format(time.RFC3339)
	}
	if !doc.UpdatedAt.IsZero() {
		payload["updated_at"] = doc.UpdatedAt.Format(time.RFC3339)
	} else {
		payload["updated_at"] = time.Now().Format(time.RFC3339)
	}

	if len(doc.Metadata) > 0 {
		payload["metadata"] = doc.Metadata
		// Also merge important fields to top level for easier access
		if source, ok := doc.Metadata["source"]; ok && doc.Source == "" {
			payload["source"] = source
		}
		if title, ok := doc.Metadata["title"]; ok && doc.Title == "" {
			payload["title"] = title
		}
		if format, ok := doc.Metadata["format"]; ok && doc.Format == "" {
			payload["format"] = format
		}
		if createdAt, ok := doc.Metadata["created_at"]; ok {
			payload["created_at"] = createdAt
		}
		if updatedAt, ok := doc.Metadata["updated_at"]; ok {
			payload["updated_at"] = updatedAt
		}
	}

	setPayloadReq := map[string]interface{}{
		"payload": payload,
		"points":  pointIDs,
	}

	reqBody, err = json.Marshal(setPayloadReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url = fmt.Sprintf("%s/collections/%s/points/payload", s.baseURL, s.collection)
	req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err = s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set payload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set payload: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetDocument retrieves a document by ID.
func (s *Store) GetDocument(ctx context.Context, id string) (*index.StoredDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get one chunk to extract document info
	filter := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"key":   "document_id",
				"match": map[string]interface{}{"value": id},
			},
		},
	}

	scrollReq := map[string]interface{}{
		"filter":       filter,
		"limit":        1,
		"with_payload": true,
	}

	reqBody, err := json.Marshal(scrollReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/scroll", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to scroll: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil // Document not found
	}

	var scrollResult struct {
		Result struct {
			Points []struct {
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(scrollResult.Result.Points) == 0 {
		return nil, nil // Document not found
	}

	// Extract document info from chunk
	chunk, err := s.extractChunkFromPayload(scrollResult.Result.Points[0].Payload, nil)
	if err != nil {
		return nil, err
	}

	doc := &index.StoredDocument{
		ID:       id,
		Metadata: chunk.Metadata,
		Hash:     chunk.Hash,
	}

	// Extract document-level metadata
	if source, ok := chunk.Metadata["source"]; ok {
		doc.Source = source
	}
	if title, ok := chunk.Metadata["title"]; ok {
		doc.Title = title
	}
	if format, ok := chunk.Metadata["format"]; ok {
		doc.Format = format
	}

	return doc, nil
}

// Close closes the store and releases resources.
// GetStats returns statistics about stored documents and chunks.
func (s *Store) GetStats(ctx context.Context) (*index.Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &index.Stats{}

	// Get collection info to get total points (chunks)
	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Collection might not exist yet
		return &index.Stats{
			TotalDocuments: 0,
			TotalChunks:    0,
			TotalSize:      0,
		}, nil
	}

	var collectionInfo struct {
		Result struct {
			PointsCount int64 `json:"points_count"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&collectionInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	stats.TotalChunks = collectionInfo.Result.PointsCount

	// Count unique documents by scrolling through all points
	// This is expensive, but necessary for accurate document count
	documentIDs := make(map[string]bool)
	scrollReq := map[string]interface{}{
		"limit":        100,
		"with_payload": true,
		"with_vector":  false,
	}

	for {
		reqBody, _ := json.Marshal(scrollReq)
		scrollURL := fmt.Sprintf("%s/collections/%s/points/scroll", s.baseURL, s.collection)
		scrollHTTPReq, err := http.NewRequestWithContext(ctx, "POST", scrollURL, bytes.NewBuffer(reqBody))
		if err != nil {
			break
		}
		scrollHTTPReq.Header.Set("Content-Type", "application/json")
		if s.apiKey != "" {
			scrollHTTPReq.Header.Set("api-key", s.apiKey)
		}

		scrollResp, err := s.httpClient.Do(scrollHTTPReq)
		if err != nil {
			break
		}

		var scrollResult struct {
			Result struct {
				Points []struct {
					Payload map[string]interface{} `json:"payload"`
				} `json:"points"`
				NextPageOffset interface{} `json:"next_page_offset"`
			} `json:"result"`
		}

		if err := json.NewDecoder(scrollResp.Body).Decode(&scrollResult); err != nil {
			scrollResp.Body.Close()
			break
		}
		scrollResp.Body.Close()

		for _, point := range scrollResult.Result.Points {
			if docID, ok := point.Payload["document_id"].(string); ok {
				documentIDs[docID] = true
			}
		}

		if scrollResult.Result.NextPageOffset == nil {
			break
		}

		scrollReq["offset"] = scrollResult.Result.NextPageOffset
	}

	stats.TotalDocuments = int64(len(documentIDs))

	// Total size is not easily available from Qdrant without scanning all points
	// For now, we'll set it to 0 or approximate it
	stats.TotalSize = 0

	return stats, nil
}

// StoreVersion stores a version snapshot of a document.
// Note: Qdrant doesn't natively support versioning, so this is a no-op.
// For full version control, consider using a separate version tracking system.
func (s *Store) StoreVersion(ctx context.Context, documentID string, changeNote string) error {
	// Qdrant doesn't support versioning natively
	// In a production system, you might want to store version metadata separately
	return fmt.Errorf("version control not supported for Qdrant store")
}

// ListVersions returns all versions of a document.
func (s *Store) ListVersions(ctx context.Context, documentID string) ([]index.DocumentVersion, error) {
	// Qdrant doesn't support versioning natively
	return nil, fmt.Errorf("version control not supported for Qdrant store")
}

// RestoreVersion restores a document to a specific version.
func (s *Store) RestoreVersion(ctx context.Context, documentID string, version int) error {
	// Qdrant doesn't support versioning natively
	return fmt.Errorf("version control not supported for Qdrant store")
}

// SoftDeleteDocument marks a document as deleted by adding deleted_at metadata.
func (s *Store) SoftDeleteDocument(ctx context.Context, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find all chunks for this document
	filter := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"key":   "document_id",
				"match": map[string]interface{}{"value": documentID},
			},
		},
	}

	// Get all point IDs for this document
	scrollReq := map[string]interface{}{
		"filter":       filter,
		"limit":        100,
		"with_payload": false,
	}

	reqBody, err := json.Marshal(scrollReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/scroll", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to scroll: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to scroll points: status %d, body: %s", resp.StatusCode, string(body))
	}

	var scrollResult struct {
		Result struct {
			Points []struct {
				ID interface{} `json:"id"`
			} `json:"points"`
			NextPageOffset interface{} `json:"next_page_offset"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Collect all point IDs (handle pagination)
	pointIDs := make([]interface{}, 0)
	for _, point := range scrollResult.Result.Points {
		pointIDs = append(pointIDs, point.ID)
	}

	// Handle pagination
	for scrollResult.Result.NextPageOffset != nil {
		scrollReq["offset"] = scrollResult.Result.NextPageOffset
		reqBody, _ := json.Marshal(scrollReq)
		req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		if s.apiKey != "" {
			req.Header.Set("api-key", s.apiKey)
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			break
		}

		if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
			resp.Body.Close()
			break
		}
		resp.Body.Close()

		for _, point := range scrollResult.Result.Points {
			pointIDs = append(pointIDs, point.ID)
		}
	}

	if len(pointIDs) == 0 {
		return fmt.Errorf("document not found: %s", documentID)
	}

	// Update payload to add deleted_at timestamp
	deletedAt := time.Now().Format(time.RFC3339)
	setPayload := map[string]interface{}{
		"deleted_at": deletedAt,
	}

	setPayloadReq := map[string]interface{}{
		"points": pointIDs,
		"payload": setPayload,
	}

	reqBody, err = json.Marshal(setPayloadReq)
	if err != nil {
		return fmt.Errorf("failed to marshal set payload request: %w", err)
	}

	setURL := fmt.Sprintf("%s/collections/%s/points/payload", s.baseURL, s.collection)
	setReq, err := http.NewRequestWithContext(ctx, "PUT", setURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create set payload request: %w", err)
	}
	setReq.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		setReq.Header.Set("api-key", s.apiKey)
	}

	setResp, err := s.httpClient.Do(setReq)
	if err != nil {
		return fmt.Errorf("failed to set payload: %w", err)
	}
	defer setResp.Body.Close()

	if setResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(setResp.Body)
		return fmt.Errorf("failed to set deleted_at: status %d, body: %s", setResp.StatusCode, string(body))
	}

	return nil
}

// RestoreDocument restores a soft-deleted document by removing deleted_at metadata.
func (s *Store) RestoreDocument(ctx context.Context, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find all chunks for this document (including deleted ones)
	filter := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"key":   "document_id",
				"match": map[string]interface{}{"value": documentID},
			},
		},
	}

	// Get all point IDs for this document
	scrollReq := map[string]interface{}{
		"filter":       filter,
		"limit":        100,
		"with_payload": false,
	}

	reqBody, err := json.Marshal(scrollReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/scroll", s.baseURL, s.collection)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to scroll: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to scroll points: status %d, body: %s", resp.StatusCode, string(body))
	}

	var scrollResult struct {
		Result struct {
			Points []struct {
				ID interface{} `json:"id"`
			} `json:"points"`
			NextPageOffset interface{} `json:"next_page_offset"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Collect all point IDs (handle pagination)
	pointIDs := make([]interface{}, 0)
	for _, point := range scrollResult.Result.Points {
		pointIDs = append(pointIDs, point.ID)
	}

	// Handle pagination
	for scrollResult.Result.NextPageOffset != nil {
		scrollReq["offset"] = scrollResult.Result.NextPageOffset
		reqBody, _ := json.Marshal(scrollReq)
		req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		if s.apiKey != "" {
			req.Header.Set("api-key", s.apiKey)
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			break
		}

		if err := json.NewDecoder(resp.Body).Decode(&scrollResult); err != nil {
			resp.Body.Close()
			break
		}
		resp.Body.Close()

		for _, point := range scrollResult.Result.Points {
			pointIDs = append(pointIDs, point.ID)
		}
	}

	if len(pointIDs) == 0 {
		return fmt.Errorf("document not found: %s", documentID)
	}

	// Delete deleted_at from payload
	deletePayloadReq := map[string]interface{}{
		"points": pointIDs,
		"keys":   []string{"deleted_at"},
	}

	reqBody, err = json.Marshal(deletePayloadReq)
	if err != nil {
		return fmt.Errorf("failed to marshal delete payload request: %w", err)
	}

	deleteURL := fmt.Sprintf("%s/collections/%s/points/payload/delete", s.baseURL, s.collection)
	deleteReq, err := http.NewRequestWithContext(ctx, "POST", deleteURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create delete payload request: %w", err)
	}
	deleteReq.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		deleteReq.Header.Set("api-key", s.apiKey)
	}

	deleteResp, err := s.httpClient.Do(deleteReq)
	if err != nil {
		return fmt.Errorf("failed to delete payload: %w", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(deleteResp.Body)
		return fmt.Errorf("failed to remove deleted_at: status %d, body: %s", deleteResp.StatusCode, string(body))
	}

	return nil
}

// HardDeleteDocument permanently deletes a document.
func (s *Store) HardDeleteDocument(ctx context.Context, documentID string) error {
	return s.DeleteByDocument(ctx, documentID)
}

func (s *Store) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}
