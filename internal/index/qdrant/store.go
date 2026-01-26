package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

		// Add custom metadata
		if len(chunk.Metadata) > 0 {
			payload["metadata"] = chunk.Metadata
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
			must = append(must, map[string]interface{}{
				"key":   fmt.Sprintf("metadata.%s", key),
				"match": map[string]interface{}{"value": value},
			})
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

	// Extract metadata
	if metadata, ok := payload["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				chunk.Metadata[k] = str
			}
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

// ListDocuments returns all stored documents.
func (s *Store) ListDocuments(ctx context.Context) ([]index.StoredDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use scroll to get all points
	docMap := make(map[string]*index.StoredDocument)
	offset := ""
	limit := 100

	for {
		scrollReq := map[string]interface{}{
			"limit":        limit,
			"with_payload": true,
		}
		if offset != "" {
			scrollReq["offset"] = offset
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

			// Get or create document
			doc, exists := docMap[docID]
			if !exists {
				doc = &index.StoredDocument{
					ID:       docID,
					Metadata: make(map[string]string),
				}
				docMap[docID] = doc
			}

			// Update document info from chunk
			if doc.Source == "" {
				if source, ok := chunk.Metadata["source"]; ok {
					doc.Source = source
				}
			}
			if doc.Title == "" {
				if title, ok := chunk.Metadata["title"]; ok {
					doc.Title = title
				}
			}
			if doc.Format == "" {
				if format, ok := chunk.Metadata["format"]; ok {
					doc.Format = format
				}
			}
			if doc.Hash == "" {
				doc.Hash = chunk.Hash
			}
		}

		// Check if we have more results
		if scrollResult.Result.NextPageOffset == nil {
			break
		}

		// Update offset for next iteration
		if nextOffset, ok := scrollResult.Result.NextPageOffset.(string); ok && nextOffset != "" {
			offset = nextOffset
		} else {
			break
		}
	}

	// Convert map to slice
	docs := make([]index.StoredDocument, 0, len(docMap))
	for _, doc := range docMap {
		docs = append(docs, *doc)
	}

	return docs, nil
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

	if len(doc.Metadata) > 0 {
		payload["metadata"] = doc.Metadata
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
func (s *Store) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}
