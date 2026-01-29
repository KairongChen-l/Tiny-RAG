package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/conversation"
	"github.com/krc/rag/internal/service"
)

// CreateConversationRequest represents a request to create a conversation.
type CreateConversationRequest struct {
	Title string `json:"title,omitempty"`
}

// CreateConversationResponse represents the response for creating a conversation.
type CreateConversationResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// CreateConversation creates a new conversation.
func (h *Handler) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Empty body is fine, use default title
		req.Title = "New Chat"
	}

	if req.Title == "" {
		req.Title = "New Chat"
	}

	// Use ConversationService if available
	if h.conversationService != nil {
		serviceReq := service.CreateConversationRequest{
			Title: req.Title,
		}

		resp, err := h.conversationService.CreateConversation(c.Request.Context(), serviceReq)
		if err != nil {
			h.logger.Error("failed to create conversation", zap.Error(err))
			WriteError(c, 500, ErrCodeInternalError, "failed to create conversation")
			return
		}

		WriteJSON(c, 201, CreateConversationResponse{
			ID:    resp.ID,
			Title: resp.Title,
		})
		return
	}

	// Fallback to legacy implementation
	conv := conversation.New(req.Title)
	if err := h.convStore.Create(c.Request.Context(), conv); err != nil {
		h.logger.Error("failed to create conversation", zap.Error(err))
		WriteError(c, 500, ErrCodeInternalError, "failed to create conversation")
		return
	}

	WriteJSON(c, 201, CreateConversationResponse{
		ID:    conv.ID,
		Title: conv.Title,
	})
}

// GetConversation retrieves a conversation by ID.
func (h *Handler) GetConversation(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		WriteError(c, 400, ErrCodeValidation, "conversation id is required")
		return
	}

	// Use ConversationService if available
	if h.conversationService != nil {
		conv, err := h.conversationService.GetConversation(c.Request.Context(), id)
		if err != nil {
			WriteError(c, 404, ErrCodeNotFound, "conversation not found")
			return
		}
		WriteJSON(c, 200, conv)
		return
	}

	// Fallback to legacy implementation
	conv, err := h.convStore.Get(c.Request.Context(), id)
	if err != nil {
		WriteError(c, 404, ErrCodeNotFound, "conversation not found")
		return
	}
	WriteJSON(c, 200, conv)
}

// ListConversations lists all conversations.
func (h *Handler) ListConversations(c *gin.Context) {
	// Use ConversationService if available
	if h.conversationService != nil {
		summaries, err := h.conversationService.ListConversations(c.Request.Context(), 50, 0)
		if err != nil {
			h.logger.Error("failed to list conversations", zap.Error(err))
			WriteError(c, 500, ErrCodeInternalError, "failed to list conversations")
			return
		}
		WriteJSON(c, 200, map[string]interface{}{
			"conversations": summaries,
		})
		return
	}

	// Fallback to legacy implementation
	summaries, err := h.convStore.List(c.Request.Context(), 50, 0)
	if err != nil {
		h.logger.Error("failed to list conversations", zap.Error(err))
		WriteError(c, 500, ErrCodeInternalError, "failed to list conversations")
		return
	}
	WriteJSON(c, 200, map[string]interface{}{
		"conversations": summaries,
	})
}

// DeleteConversation deletes a conversation.
func (h *Handler) DeleteConversation(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		WriteError(c, 400, ErrCodeValidation, "conversation id is required")
		return
	}

	// Use ConversationService if available
	if h.conversationService != nil {
		if err := h.conversationService.DeleteConversation(c.Request.Context(), id); err != nil {
			WriteError(c, 404, ErrCodeNotFound, "conversation not found")
			return
		}
		c.Status(204)
		return
	}

	// Fallback to legacy implementation
	if err := h.convStore.Delete(c.Request.Context(), id); err != nil {
		WriteError(c, 404, ErrCodeNotFound, "conversation not found")
		return
	}
	c.Status(204)
}

// SendMessageRequest represents a request to send a message.
type SendMessageRequest struct {
	Content string       `json:"content"`
	Options QueryOptions `json:"options,omitempty"`
}

// SendMessageResponse represents the response for sending a message.
type SendMessageResponse struct {
	UserMessage      conversation.Message `json:"user_message"`
	AssistantMessage conversation.Message `json:"assistant_message"`
}

// SendMessage sends a message in a conversation and gets a RAG response.
func (h *Handler) SendMessage(c *gin.Context) {
	convID := c.Param("id")
	if convID == "" {
		WriteError(c, 400, ErrCodeValidation, "conversation id is required")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteError(c, 400, ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		WriteError(c, 400, ErrCodeValidation, "content is required")
		return
	}

	ctx := c.Request.Context()

	// Use ConversationService if available
	if h.conversationService != nil {
		serviceReq := service.SendMessageRequest{
			ConversationID: convID,
			Query:          req.Content,
			TopK:           req.Options.TopK,
			Filter:         req.Options.Filter,
			Rerank:         req.Options.EnableRerank,
			Provider:       req.Options.Provider,
		}

		_, err := h.conversationService.SendMessage(ctx, serviceReq)
		if err != nil {
			h.logger.Error("send message failed", zap.Error(err))
			WriteError(c, 500, ErrCodeInternalError, "send message failed")
			return
		}

		// Get conversation to build response
		conv, err := h.conversationService.GetConversation(ctx, convID)
		if err != nil {
			WriteError(c, 404, ErrCodeNotFound, "conversation not found")
			return
		}

		// Find user and assistant messages (last two messages)
		var userMsg, assistantMsg *conversation.Message
		if len(conv.Messages) >= 2 {
			userMsg = &conv.Messages[len(conv.Messages)-2]
			assistantMsg = &conv.Messages[len(conv.Messages)-1]
		} else if len(conv.Messages) == 1 {
			userMsg = &conv.Messages[0]
		}

		if userMsg == nil || assistantMsg == nil {
			WriteError(c, 500, ErrCodeInternalError, "failed to get messages")
			return
		}

		WriteJSON(c, 200, SendMessageResponse{
			UserMessage:      *userMsg,
			AssistantMessage: *assistantMsg,
		})
		return
	}

	// Fallback to legacy implementation
	WriteError(c, 503, ErrCodeInternalError, "conversation service not configured")
}

// buildConversationContext builds a context string from conversation history.
func buildConversationContext(messages []conversation.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var context string
	for _, msg := range messages {
		role := "User"
		if msg.Role == conversation.RoleAssistant {
			role = "Assistant"
		}
		context += role + ": " + msg.Content + "\n\n"
	}
	return context
}
