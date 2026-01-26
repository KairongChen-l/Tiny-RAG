package conversation

import (
	"testing"
	"time"
)

func TestNewConversation(t *testing.T) {
	conv := New("Test Conversation")

	if conv.ID == "" {
		t.Error("conversation ID should not be empty")
	}
	if conv.Title != "Test Conversation" {
		t.Errorf("expected title 'Test Conversation', got %q", conv.Title)
	}
	if len(conv.Messages) != 0 {
		t.Errorf("new conversation should have 0 messages, got %d", len(conv.Messages))
	}
	if conv.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestConversation_AddMessage(t *testing.T) {
	conv := New("Test")

	// Add user message
	userMsg := conv.AddMessage(RoleUser, "Hello, what is RAG?")
	if userMsg.Role != RoleUser {
		t.Errorf("expected role %q, got %q", RoleUser, userMsg.Role)
	}
	if userMsg.Content != "Hello, what is RAG?" {
		t.Errorf("unexpected content: %q", userMsg.Content)
	}
	if len(conv.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(conv.Messages))
	}

	// Add assistant message with citations
	assistantMsg := conv.AddMessage(RoleAssistant, "RAG stands for Retrieval-Augmented Generation...")
	assistantMsg.Citations = []Citation{
		{ID: 1, Source: "doc1.md", Section: "Introduction", Preview: "RAG is..."},
	}

	if len(conv.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(conv.Messages))
	}
	if len(assistantMsg.Citations) != 1 {
		t.Errorf("expected 1 citation, got %d", len(assistantMsg.Citations))
	}
}

func TestConversation_GetMessages(t *testing.T) {
	conv := New("Test")
	conv.AddMessage(RoleUser, "Q1")
	conv.AddMessage(RoleAssistant, "A1")
	conv.AddMessage(RoleUser, "Q2")
	conv.AddMessage(RoleAssistant, "A2")

	messages := conv.GetMessages()
	if len(messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(messages))
	}

	// Check order
	if messages[0].Content != "Q1" || messages[0].Role != RoleUser {
		t.Errorf("first message incorrect: %+v", messages[0])
	}
	if messages[1].Content != "A1" || messages[1].Role != RoleAssistant {
		t.Errorf("second message incorrect: %+v", messages[1])
	}
}

func TestConversation_GetContextWindow(t *testing.T) {
	conv := New("Test")

	// Add 10 messages
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			conv.AddMessage(RoleUser, "Q"+string(rune('0'+i/2)))
		} else {
			conv.AddMessage(RoleAssistant, "A"+string(rune('0'+i/2)))
		}
	}

	// Get last 4 messages as context
	context := conv.GetContextWindow(4)
	if len(context) != 4 {
		t.Errorf("expected 4 messages in context, got %d", len(context))
	}

	// Should be the last 4 messages
	if context[0].Content != "Q3" {
		t.Errorf("expected first context message 'Q3', got %q", context[0].Content)
	}
}

func TestConversation_Summary(t *testing.T) {
	conv := New("My Chat")
	conv.AddMessage(RoleUser, "Hello")
	conv.AddMessage(RoleAssistant, "Hi there!")

	summary := conv.Summary()
	if summary.ID != conv.ID {
		t.Error("summary ID mismatch")
	}
	if summary.Title != "My Chat" {
		t.Error("summary title mismatch")
	}
	if summary.MessageCount != 2 {
		t.Errorf("expected 2 messages, got %d", summary.MessageCount)
	}
	if summary.LastMessage != "Hi there!" {
		t.Errorf("expected last message 'Hi there!', got %q", summary.LastMessage)
	}
}

func TestMessage_Roles(t *testing.T) {
	tests := []struct {
		role     Role
		expected string
	}{
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleSystem, "system"},
	}

	for _, tt := range tests {
		if string(tt.role) != tt.expected {
			t.Errorf("expected role %q, got %q", tt.expected, tt.role)
		}
	}
}

func TestConversation_UpdatedAt(t *testing.T) {
	conv := New("Test")
	initialUpdated := conv.UpdatedAt

	time.Sleep(10 * time.Millisecond)
	conv.AddMessage(RoleUser, "New message")

	if !conv.UpdatedAt.After(initialUpdated) {
		t.Error("UpdatedAt should be updated after adding message")
	}
}
