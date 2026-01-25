package conversation

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Store defines the interface for conversation storage.
type Store interface {
	// Create creates a new conversation.
	Create(ctx context.Context, conv *Conversation) error

	// Get retrieves a conversation by ID.
	Get(ctx context.Context, id string) (*Conversation, error)

	// Update updates an existing conversation.
	Update(ctx context.Context, conv *Conversation) error

	// Delete removes a conversation.
	Delete(ctx context.Context, id string) error

	// List returns conversation summaries with pagination.
	List(ctx context.Context, limit, offset int) ([]Summary, error)
}

// MemoryStore is an in-memory implementation of Store.
type MemoryStore struct {
	mu            sync.RWMutex
	conversations map[string]*Conversation
}

// NewMemoryStore creates a new in-memory conversation store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[string]*Conversation),
	}
}

// Create creates a new conversation.
func (s *MemoryStore) Create(ctx context.Context, conv *Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Deep copy to avoid mutation
	stored := &Conversation{
		ID:        conv.ID,
		Title:     conv.Title,
		Messages:  make([]Message, len(conv.Messages)),
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}
	copy(stored.Messages, conv.Messages)

	s.conversations[conv.ID] = stored
	return nil
}

// Get retrieves a conversation by ID.
func (s *MemoryStore) Get(ctx context.Context, id string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.conversations[id]
	if !ok {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}

	// Return a copy to avoid mutation
	result := &Conversation{
		ID:        conv.ID,
		Title:     conv.Title,
		Messages:  make([]Message, len(conv.Messages)),
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}
	copy(result.Messages, conv.Messages)

	return result, nil
}

// Update updates an existing conversation.
func (s *MemoryStore) Update(ctx context.Context, conv *Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conversations[conv.ID]; !ok {
		return fmt.Errorf("conversation not found: %s", conv.ID)
	}

	// Deep copy to avoid mutation
	stored := &Conversation{
		ID:        conv.ID,
		Title:     conv.Title,
		Messages:  make([]Message, len(conv.Messages)),
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}
	copy(stored.Messages, conv.Messages)

	s.conversations[conv.ID] = stored
	return nil
}

// Delete removes a conversation.
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conversations[id]; !ok {
		return fmt.Errorf("conversation not found: %s", id)
	}

	delete(s.conversations, id)
	return nil
}

// List returns conversation summaries with pagination.
func (s *MemoryStore) List(ctx context.Context, limit, offset int) ([]Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all conversations
	convs := make([]*Conversation, 0, len(s.conversations))
	for _, conv := range s.conversations {
		convs = append(convs, conv)
	}

	// Sort by UpdatedAt descending (most recent first)
	sort.Slice(convs, func(i, j int) bool {
		return convs[i].UpdatedAt.After(convs[j].UpdatedAt)
	})

	// Apply pagination
	if offset >= len(convs) {
		return []Summary{}, nil
	}

	end := offset + limit
	if end > len(convs) {
		end = len(convs)
	}

	convs = convs[offset:end]

	// Convert to summaries
	summaries := make([]Summary, len(convs))
	for i, conv := range convs {
		summaries[i] = conv.Summary()
	}

	return summaries, nil
}

