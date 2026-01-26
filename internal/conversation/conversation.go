// Package conversation provides multi-turn conversation management.
package conversation

import (
	"time"

	"github.com/google/uuid"
)

// Role represents the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Citation represents a source citation in a message.
type Citation struct {
	ID      int    `json:"id"`
	Source  string `json:"source"`
	Section string `json:"section"`
	Preview string `json:"preview"`
}

// Message represents a single message in a conversation.
type Message struct {
	ID        string     `json:"id"`
	Role      Role       `json:"role"`
	Content   string     `json:"content"`
	Citations []Citation `json:"citations,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Conversation represents a multi-turn chat conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Summary represents a conversation summary for listing.
type Summary struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	MessageCount int       `json:"message_count"`
	LastMessage  string    `json:"last_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// New creates a new conversation with the given title.
func New(title string) *Conversation {
	now := time.Now()
	return &Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		Messages:  make([]Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMessage adds a new message to the conversation.
func (c *Conversation) AddMessage(role Role, content string) *Message {
	msg := Message{
		ID:        uuid.New().String(),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now()
	return &c.Messages[len(c.Messages)-1]
}

// GetMessages returns all messages in the conversation.
func (c *Conversation) GetMessages() []Message {
	return c.Messages
}

// GetContextWindow returns the last n messages for context.
func (c *Conversation) GetContextWindow(n int) []Message {
	if n >= len(c.Messages) {
		return c.Messages
	}
	return c.Messages[len(c.Messages)-n:]
}

// Summary returns a summary of the conversation.
func (c *Conversation) Summary() Summary {
	var lastMsg string
	if len(c.Messages) > 0 {
		lastMsg = c.Messages[len(c.Messages)-1].Content
		// Truncate long messages
		if len(lastMsg) > 100 {
			lastMsg = lastMsg[:100] + "..."
		}
	}

	return Summary{
		ID:           c.ID,
		Title:        c.Title,
		MessageCount: len(c.Messages),
		LastMessage:  lastMsg,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

// ToPromptMessages converts messages to the format needed for LLM.
func (c *Conversation) ToPromptMessages() []PromptMessage {
	msgs := make([]PromptMessage, len(c.Messages))
	for i, m := range c.Messages {
		msgs[i] = PromptMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}
	return msgs
}

// PromptMessage is a simplified message for LLM input.
type PromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}


