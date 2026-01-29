package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

// ElasticsearchClient wraps Elasticsearch client for full-text search.
type ElasticsearchClient struct {
	client *elastic.Client
	logger *zap.Logger
	index  string
}

// Config holds Elasticsearch configuration.
type Config struct {
	URLs   []string
	Index  string
	Logger *zap.Logger
	Sniff  bool
}

// Document represents a searchable document.
type Document struct {
	ID          string            `json:"id"`
	DocumentID  string            `json:"document_id"`
	Content     string            `json:"content"`
	Title       string            `json:"title,omitempty"`
	SectionPath string            `json:"section_path,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
	// Multi-tenant fields
	UserID      string `json:"user_id,omitempty"`      // User ID for multi-tenant support
	OrgTag      string `json:"org_tag,omitempty"`      // Organization tag for multi-tenant support
	IsPublic    bool   `json:"is_public,omitempty"`     // Whether document is public
}

// SearchResult represents a search result.
type SearchResult struct {
	Document Document
	Score    float64
}

// NewElasticsearchClient creates a new Elasticsearch client.
func NewElasticsearchClient(cfg Config) (*ElasticsearchClient, error) {
	client, err := elastic.NewClient(
		elastic.SetURL(cfg.URLs...),
		elastic.SetSniff(cfg.Sniff),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	es := &ElasticsearchClient{
		client: client,
		logger: cfg.Logger,
		index:  cfg.Index,
	}

	// Create index if it doesn't exist
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := es.createIndex(ctx); err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	return es, nil
}

// createIndex creates the Elasticsearch index with mapping.
func (e *ElasticsearchClient) createIndex(ctx context.Context) error {
	exists, err := e.client.IndexExists(e.index).Do(ctx)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// Define mapping for full-text search with multi-tenant support
	mapping := `{
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"document_id": {"type": "keyword"},
				"content": {
					"type": "text",
					"analyzer": "standard"
				},
				"title": {
					"type": "text",
					"analyzer": "standard"
				},
				"section_path": {"type": "keyword"},
				"metadata": {"type": "object"},
				"created_at": {"type": "date"},
				"user_id": {"type": "keyword"},
				"org_tag": {"type": "keyword"},
				"is_public": {"type": "boolean"}
			}
		}
	}`

	_, err = e.client.CreateIndex(e.index).BodyString(mapping).Do(ctx)
	return err
}

// IndexDocument indexes a document in Elasticsearch.
func (e *ElasticsearchClient) IndexDocument(ctx context.Context, doc Document) error {
	_, err := e.client.Index().
		Index(e.index).
		Id(doc.ID).
		BodyJson(doc).
		Do(ctx)
	return err
}

// SearchOptions holds options for search queries.
type SearchOptions struct {
	UserID   string // Filter by user ID (multi-tenant)
	OrgTag   string // Filter by organization tag (multi-tenant)
	IsPublic *bool  // Filter by public status (nil = no filter)
}

// Search performs a full-text search with optional multi-tenant filtering.
func (e *ElasticsearchClient) Search(ctx context.Context, query string, size int, opts ...SearchOptions) ([]SearchResult, error) {
	searchQuery := elastic.NewMultiMatchQuery(query, "content", "title").
		Type("best_fields").
		Fuzziness("AUTO")

	// Apply multi-tenant filters if provided
	var searchOpts SearchOptions
	if len(opts) > 0 {
		searchOpts = opts[0]
	}

	// Build bool query with filters
	boolQuery := elastic.NewBoolQuery().Must(searchQuery)

	// Add user ID filter
	if searchOpts.UserID != "" {
		boolQuery = boolQuery.Filter(elastic.NewTermQuery("user_id", searchOpts.UserID))
	}

	// Add org tag filter
	if searchOpts.OrgTag != "" {
		boolQuery = boolQuery.Filter(elastic.NewTermQuery("org_tag", searchOpts.OrgTag))
	}

	// Add public filter
	if searchOpts.IsPublic != nil {
		boolQuery = boolQuery.Filter(elastic.NewTermQuery("is_public", *searchOpts.IsPublic))
	} else {
		// Default: show public documents or user's own documents
		if searchOpts.UserID != "" {
			publicQuery := elastic.NewBoolQuery().
				Should(elastic.NewTermQuery("is_public", true)).
				Should(elastic.NewTermQuery("user_id", searchOpts.UserID)).
				MinimumShouldMatch("1")
			boolQuery = boolQuery.Filter(publicQuery)
		} else {
			// If no user ID, only show public documents
			boolQuery = boolQuery.Filter(elastic.NewTermQuery("is_public", true))
		}
	}

	searchResult, err := e.client.Search().
		Index(e.index).
		Query(boolQuery).
		Size(size).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, hit := range searchResult.Hits.Hits {
		var doc Document
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			e.logger.Warn("failed to unmarshal document", zap.Error(err))
			continue
		}

		score := 0.0
		if hit.Score != nil {
			score = *hit.Score
		}

		results = append(results, SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	return results, nil
}

// DeleteDocument deletes a document from Elasticsearch.
func (e *ElasticsearchClient) DeleteDocument(ctx context.Context, id string) error {
	_, err := e.client.Delete().
		Index(e.index).
		Id(id).
		Do(ctx)
	return err
}

// DeleteByDocumentID deletes all documents with a given document_id.
func (e *ElasticsearchClient) DeleteByDocumentID(ctx context.Context, documentID string) error {
	query := elastic.NewTermQuery("document_id", documentID)
	_, err := e.client.DeleteByQuery(e.index).
		Query(query).
		Do(ctx)
	return err
}

// Close closes the Elasticsearch client.
func (e *ElasticsearchClient) Close() error {
	e.client.Stop()
	return nil
}

// BulkIndex indexes multiple documents in bulk.
func (e *ElasticsearchClient) BulkIndex(ctx context.Context, docs []Document) error {
	bulk := e.client.Bulk()

	for _, doc := range docs {
		req := elastic.NewBulkIndexRequest().
			Index(e.index).
			Id(doc.ID).
			Doc(doc)
		bulk = bulk.Add(req)
	}

	response, err := bulk.Do(ctx)
	if err != nil {
		return err
	}

	if response.Errors {
		var errorMsgs []string
		for _, item := range response.Failed() {
			errorMsgs = append(errorMsgs, fmt.Sprintf("failed to index %s: %s", item.Id, item.Error.Reason))
		}
		return fmt.Errorf("bulk index errors: %s", strings.Join(errorMsgs, "; "))
	}

	return nil
}
