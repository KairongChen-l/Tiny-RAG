package conversation

import (
	"context"
	"testing"
)

func TestMemoryStore_CreateAndGet(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	conv := New("Test Chat")
	conv.AddMessage(RoleUser, "Hello")
	conv.AddMessage(RoleAssistant, "Hi!")

	// Create
	err := store.Create(ctx, conv)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get
	retrieved, err := store.Get(ctx, conv.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != conv.ID {
		t.Error("ID mismatch")
	}
	if retrieved.Title != "Test Chat" {
		t.Error("title mismatch")
	}
	if len(retrieved.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(retrieved.Messages))
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	conv := New("Original Title")
	store.Create(ctx, conv)

	// Update
	conv.Title = "Updated Title"
	conv.AddMessage(RoleUser, "New message")

	err := store.Update(ctx, conv)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify
	retrieved, _ := store.Get(ctx, conv.ID)
	if retrieved.Title != "Updated Title" {
		t.Error("title not updated")
	}
	if len(retrieved.Messages) != 1 {
		t.Error("messages not updated")
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	conv := New("To Delete")
	store.Create(ctx, conv)

	// Delete
	err := store.Delete(ctx, conv.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Get(ctx, conv.ID)
	if err == nil {
		t.Error("expected error getting deleted conversation")
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create multiple conversations
	conv1 := New("Chat 1")
	conv2 := New("Chat 2")
	conv3 := New("Chat 3")

	store.Create(ctx, conv1)
	store.Create(ctx, conv2)
	store.Create(ctx, conv3)

	// List all
	summaries, err := store.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(summaries) != 3 {
		t.Errorf("expected 3 conversations, got %d", len(summaries))
	}

	// List with pagination
	summaries, _ = store.List(ctx, 2, 0)
	if len(summaries) != 2 {
		t.Errorf("expected 2 conversations with limit, got %d", len(summaries))
	}

	summaries, _ = store.List(ctx, 10, 2)
	if len(summaries) != 1 {
		t.Errorf("expected 1 conversation with offset, got %d", len(summaries))
	}
}

func TestMemoryStore_NotFound(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "non-existent-id")
	if err == nil {
		t.Error("expected error for non-existent conversation")
	}
}
