package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/conversation"
	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/retrieval"
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
func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var req CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is fine, use default title
		req.Title = "New Chat"
	}

	if req.Title == "" {
		req.Title = "New Chat"
	}

	conv := conversation.New(req.Title)

	if err := h.convStore.Create(r.Context(), conv); err != nil {
		h.logger.Error("failed to create conversation", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to create conversation")
		return
	}

	WriteJSON(w, http.StatusCreated, CreateConversationResponse{
		ID:    conv.ID,
		Title: conv.Title,
	})
}

// GetConversation retrieves a conversation by ID.
func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeValidation, "conversation id is required")
		return
	}

	conv, err := h.convStore.Get(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, ErrCodeNotFound, "conversation not found")
		return
	}

	WriteJSON(w, http.StatusOK, conv)
}

// ListConversations lists all conversations.
func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.convStore.List(r.Context(), 50, 0)
	if err != nil {
		h.logger.Error("failed to list conversations", zap.Error(err))
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to list conversations")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"conversations": summaries,
	})
}

// DeleteConversation deletes a conversation.
func (h *Handler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeValidation, "conversation id is required")
		return
	}

	if err := h.convStore.Delete(r.Context(), id); err != nil {
		WriteError(w, http.StatusNotFound, ErrCodeNotFound, "conversation not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	if convID == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeValidation, "conversation id is required")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeValidation, "content is required")
		return
	}

	// Get conversation
	conv, err := h.convStore.Get(r.Context(), convID)
	if err != nil {
		WriteError(w, http.StatusNotFound, ErrCodeNotFound, "conversation not found")
		return
	}

	// Add user message
	userMsg := conv.AddMessage(conversation.RoleUser, req.Content)

	// Build retrieval options
	retrieveOpts := retrieval.DefaultRetrieveOptions()
	if req.Options.TopK > 0 {
		retrieveOpts.TopK = req.Options.TopK
	}
	if req.Options.Filter != nil {
		retrieveOpts.MetadataFilter = req.Options.Filter
	}
	retrieveOpts.EnableRerank = req.Options.EnableRerank

	// Retrieve relevant chunks
	result, err := h.retriever.Retrieve(r.Context(), req.Content, retrieveOpts)
	if err != nil {
		h.logger.Error("retrieval failed", zap.Error(err))
		// Still save the user message
		h.convStore.Update(r.Context(), conv)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "retrieval failed")
		return
	}

	// Build prompt with conversation history
	promptOpts := prompt.DefaultPromptOptions()
	promptOpts.MaxContextTokens = h.config.Prompt.MaxContextTokens
	promptOpts.IncludeCitations = h.config.Prompt.IncludeCitations

	// Include conversation history in the prompt
	historyContext := buildConversationContext(conv.GetContextWindow(6)) // Last 6 messages

	p, err := h.promptBuilder.BuildWithHistory(r.Context(), req.Content, result.Chunks, historyContext, promptOpts)
	if err != nil {
		h.logger.Error("prompt building failed", zap.Error(err))
		h.convStore.Update(r.Context(), conv)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "prompt building failed")
		return
	}

	// Get LLM
	var llmClient, _ = h.llmRegistry.GetDefault()
	if req.Options.Provider != "" {
		if client, err := h.llmRegistry.Get(req.Options.Provider); err == nil {
			llmClient = client
		}
	}

	if llmClient == nil {
		h.convStore.Update(r.Context(), conv)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "no LLM provider available")
		return
	}

	// Generate response
	resp, err := llmClient.Generate(r.Context(), p)
	if err != nil {
		h.logger.Error("generation failed", zap.Error(err))
		h.convStore.Update(r.Context(), conv)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "generation failed")
		return
	}

	// Build citations
	citations := make([]conversation.Citation, len(p.Citations))
	for i, c := range p.Citations {
		citations[i] = conversation.Citation{
			ID:      c.ID,
			Source:  c.Source,
			Section: c.SectionPath,
			Preview: c.Preview,
		}
	}

	// Add assistant message
	assistantMsg := conv.AddMessage(conversation.RoleAssistant, resp.Answer)
	assistantMsg.Citations = citations

	// Update conversation in store
	if err := h.convStore.Update(r.Context(), conv); err != nil {
		h.logger.Error("failed to update conversation", zap.Error(err))
	}

	WriteJSON(w, http.StatusOK, SendMessageResponse{
		UserMessage:      *userMsg,
		AssistantMessage: *assistantMsg,
	})
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



