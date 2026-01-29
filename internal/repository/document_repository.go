// Package repository provides data access abstractions for business logic.
package repository

import (
	"context"

	"github.com/krc/rag/internal/index"
)

// DocumentRepository provides document data access operations.
type DocumentRepository interface {
	// Create creates a new document.
	Create(ctx context.Context, doc *index.StoredDocument) error

	// Get retrieves a document by ID.
	Get(ctx context.Context, id string) (*index.StoredDocument, error)

	// List lists documents with pagination and filtering.
	List(ctx context.Context, opts index.ListOptions) (*index.ListDocumentsResult, error)

	// Update updates an existing document.
	Update(ctx context.Context, doc *index.StoredDocument) error

	// Delete deletes a document by ID.
	Delete(ctx context.Context, id string) error

	// GetHash returns the hash of a document.
	GetHash(ctx context.Context, id string) (string, error)
}

// VectorStoreDocumentRepository implements DocumentRepository using VectorStore.
type VectorStoreDocumentRepository struct {
	store index.VectorStore
}

// NewVectorStoreDocumentRepository creates a new document repository.
func NewVectorStoreDocumentRepository(store index.VectorStore) *VectorStoreDocumentRepository {
	return &VectorStoreDocumentRepository{
		store: store,
	}
}

// Create creates a new document.
func (r *VectorStoreDocumentRepository) Create(ctx context.Context, doc *index.StoredDocument) error {
	return r.store.StoreDocument(ctx, doc)
}

// Get retrieves a document by ID.
func (r *VectorStoreDocumentRepository) Get(ctx context.Context, id string) (*index.StoredDocument, error) {
	// VectorStore doesn't have a direct Get method, so we use ListDocuments with a filter
	opts := index.ListOptions{
		Limit:  1,
		Offset: 0,
	}

	result, err := r.store.ListDocuments(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Find the document with matching ID
	for _, doc := range result.Documents {
		if doc.ID == id {
			return &doc, nil
		}
	}

	return nil, &DocumentNotFoundError{ID: id}
}

// List lists documents with pagination and filtering.
func (r *VectorStoreDocumentRepository) List(ctx context.Context, opts index.ListOptions) (*index.ListDocumentsResult, error) {
	return r.store.ListDocuments(ctx, opts)
}

// Update updates an existing document.
func (r *VectorStoreDocumentRepository) Update(ctx context.Context, doc *index.StoredDocument) error {
	// For VectorStore, update is the same as create (upsert behavior)
	return r.store.StoreDocument(ctx, doc)
}

// Delete deletes a document by ID.
func (r *VectorStoreDocumentRepository) Delete(ctx context.Context, id string) error {
	return r.store.DeleteByDocument(ctx, id)
}

// GetHash returns the hash of a document.
func (r *VectorStoreDocumentRepository) GetHash(ctx context.Context, id string) (string, error) {
	return r.store.GetDocumentHash(ctx, id)
}

// DocumentNotFoundError represents an error when a document is not found.
type DocumentNotFoundError struct {
	ID string
}

func (e *DocumentNotFoundError) Error() string {
	return "document not found: " + e.ID
}

