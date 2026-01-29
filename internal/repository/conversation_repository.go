// Package repository provides data access abstractions for business logic.
package repository

import (
	"context"

	"github.com/krc/rag/internal/conversation"
)

// ConversationRepository provides conversation data access operations.
type ConversationRepository interface {
	// Create creates a new conversation.
	Create(ctx context.Context, conv *conversation.Conversation) error

	// Get retrieves a conversation by ID.
	Get(ctx context.Context, id string) (*conversation.Conversation, error)

	// List lists conversations with pagination.
	List(ctx context.Context, limit, offset int) ([]conversation.Summary, error)

	// Update updates an existing conversation.
	Update(ctx context.Context, conv *conversation.Conversation) error

	// Delete deletes a conversation by ID.
	Delete(ctx context.Context, id string) error
}

// StoreConversationRepository implements ConversationRepository using conversation.Store.
type StoreConversationRepository struct {
	store conversation.Store
}

// NewStoreConversationRepository creates a new conversation repository.
func NewStoreConversationRepository(store conversation.Store) *StoreConversationRepository {
	return &StoreConversationRepository{
		store: store,
	}
}

// Create creates a new conversation.
func (r *StoreConversationRepository) Create(ctx context.Context, conv *conversation.Conversation) error {
	return r.store.Create(ctx, conv)
}

// Get retrieves a conversation by ID.
func (r *StoreConversationRepository) Get(ctx context.Context, id string) (*conversation.Conversation, error) {
	return r.store.Get(ctx, id)
}

// List lists conversations with pagination.
func (r *StoreConversationRepository) List(ctx context.Context, limit, offset int) ([]conversation.Summary, error) {
	return r.store.List(ctx, limit, offset)
}

// Update updates an existing conversation.
func (r *StoreConversationRepository) Update(ctx context.Context, conv *conversation.Conversation) error {
	return r.store.Update(ctx, conv)
}

// Delete deletes a conversation by ID.
func (r *StoreConversationRepository) Delete(ctx context.Context, id string) error {
	return r.store.Delete(ctx, id)
}

