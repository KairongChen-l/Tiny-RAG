package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/conversation"
	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/repository"
	"github.com/krc/rag/internal/retrieval"
)

// ConversationService handles multi-turn conversation business logic.
type ConversationService struct {
	convRepo      repository.ConversationRepository
	retriever     retrieval.Retriever
	promptBuilder prompt.PromptBuilder
	llmRegistry   *generation.LLMRegistry
	logger        *zap.Logger
}

// NewConversationService creates a new conversation service.
func NewConversationService(
	convRepo repository.ConversationRepository,
	retriever retrieval.Retriever,
	promptBuilder prompt.PromptBuilder,
	llmRegistry *generation.LLMRegistry,
	logger *zap.Logger,
) *ConversationService {
	return &ConversationService{
		convRepo:      convRepo,
		retriever:     retriever,
		promptBuilder: promptBuilder,
		llmRegistry:   llmRegistry,
		logger:        logger,
	}
}

// CreateConversationRequest represents a request to create a conversation.
type CreateConversationRequest struct {
	Title string
}

// CreateConversationResponse represents the response for creating a conversation.
type CreateConversationResponse struct {
	ID    string
	Title string
}

// CreateConversation creates a new conversation.
func (s *ConversationService) CreateConversation(ctx context.Context, req CreateConversationRequest) (*CreateConversationResponse, error) {
	title := req.Title
	if title == "" {
		title = "New Chat"
	}

	conv := conversation.New(title)
	if err := s.convRepo.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return &CreateConversationResponse{
		ID:    conv.ID,
		Title: conv.Title,
	}, nil
}

// GetConversation retrieves a conversation by ID.
func (s *ConversationService) GetConversation(ctx context.Context, id string) (*conversation.Conversation, error) {
	conv, err := s.convRepo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	return conv, nil
}

// ListConversations lists conversations with pagination.
func (s *ConversationService) ListConversations(ctx context.Context, limit, offset int) ([]conversation.Summary, error) {
	summaries, err := s.convRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	return summaries, nil
}

// DeleteConversation deletes a conversation.
func (s *ConversationService) DeleteConversation(ctx context.Context, id string) error {
	if err := s.convRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	return nil
}

// SendMessageRequest represents a request to send a message in a conversation.
type SendMessageRequest struct {
	ConversationID string
	Query          string
	TopK           int
	Filter         map[string]string
	Rerank         bool
	Provider       string
}

// SendMessageResponse represents the response for sending a message.
type SendMessageResponse struct {
	Answer     string
	Citations  []CitationInfo
	TokensUsed int
}

// SendMessage sends a message in a conversation and generates a response.
func (s *ConversationService) SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error) {
	// Get conversation
	conv, err := s.convRepo.Get(ctx, req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}

	// Retrieve relevant chunks
	retrieveOpts := retrieval.DefaultRetrieveOptions()
	if req.TopK > 0 {
		retrieveOpts.TopK = req.TopK
	}
	if req.Filter != nil {
		retrieveOpts.MetadataFilter = req.Filter
	}
	retrieveOpts.EnableRerank = req.Rerank

	retrievalResult, err := s.retriever.Retrieve(ctx, req.Query, retrieveOpts)
	if err != nil {
		return nil, fmt.Errorf("retrieval failed: %w", err)
	}

	// Get LLM provider
	var llm generation.LLM
	if req.Provider != "" {
		var err error
		llm, err = s.llmRegistry.Get(req.Provider)
		if err != nil {
			s.logger.Warn("failed to get LLM provider, using default", zap.String("provider", req.Provider), zap.Error(err))
			llm, _ = s.llmRegistry.GetDefault()
		}
	} else {
		var err error
		llm, err = s.llmRegistry.GetDefault()
		if err != nil {
			return nil, fmt.Errorf("no LLM configured: %w", err)
		}
	}

	// Build conversation history
	historyContext := buildConversationContext(conv.GetContextWindow(6)) // Last 6 messages

	// Build prompt with history
	promptOpts := prompt.DefaultPromptOptions()
	p, err := s.promptBuilder.BuildWithHistory(ctx, req.Query, retrievalResult.Chunks, historyContext, promptOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Generate answer
	response, err := llm.Generate(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Add messages to conversation
	conv.AddMessage(conversation.RoleUser, req.Query)
	conv.AddMessage(conversation.RoleAssistant, response.Answer)

	// Save conversation
	if err := s.convRepo.Update(ctx, conv); err != nil {
		s.logger.Warn("failed to update conversation", zap.Error(err))
	}

	// Convert citations
	citations := make([]CitationInfo, len(p.Citations))
	for i, cit := range p.Citations {
		citations[i] = CitationInfo{
			ID:      cit.ID,
			Source:  cit.Source,
			Section: cit.SectionPath,
			Preview: cit.Preview,
		}
	}

	return &SendMessageResponse{
		Answer:     response.Answer,
		Citations:  citations,
		TokensUsed: response.TokensUsed,
	}, nil
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

// SendMessageStream sends a message in a conversation and generates a streaming response.
func (s *ConversationService) SendMessageStream(ctx context.Context, req SendMessageRequest, callback StreamCallback) error {
	// Get conversation
	conv, err := s.convRepo.Get(ctx, req.ConversationID)
	if err != nil {
		return fmt.Errorf("conversation not found: %w", err)
	}

	// Retrieve relevant chunks
	retrieveOpts := retrieval.DefaultRetrieveOptions()
	if req.TopK > 0 {
		retrieveOpts.TopK = req.TopK
	}
	if req.Filter != nil {
		retrieveOpts.MetadataFilter = req.Filter
	}
	retrieveOpts.EnableRerank = req.Rerank

	retrievalResult, err := s.retriever.Retrieve(ctx, req.Query, retrieveOpts)
	if err != nil {
		return fmt.Errorf("retrieval failed: %w", err)
	}

	// Get LLM provider
	var llm generation.LLM
	if req.Provider != "" {
		var err error
		llm, err = s.llmRegistry.Get(req.Provider)
		if err != nil {
			s.logger.Warn("failed to get LLM provider, using default", zap.String("provider", req.Provider), zap.Error(err))
			llm, _ = s.llmRegistry.GetDefault()
		}
	} else {
		var err error
		llm, err = s.llmRegistry.GetDefault()
		if err != nil {
			return fmt.Errorf("no LLM configured: %w", err)
		}
	}

	// Check if LLM supports streaming
	streamableLLM, ok := llm.(generation.StreamableLLM)
	if !ok {
		// Fallback to non-streaming
		response, err := s.SendMessage(ctx, req)
		if err != nil {
			return err
		}
		// Send as single chunk
		return callback(StreamChunk{
			Text:        response.Answer,
			Done:        true,
			TokensUsed:  response.TokensUsed,
			FinishReason: "stop",
		})
	}

	// Build conversation history
	historyContext := buildConversationContext(conv.GetContextWindow(6)) // Last 6 messages

	// Build prompt with history
	promptOpts := prompt.DefaultPromptOptions()
	p, err := s.promptBuilder.BuildWithHistory(ctx, req.Query, retrievalResult.Chunks, historyContext, promptOpts)
	if err != nil {
		return fmt.Errorf("failed to build prompt: %w", err)
	}

	// Generate streaming response
	chunkChan, err := streamableLLM.GenerateStream(ctx, p)
	if err != nil {
		return fmt.Errorf("LLM streaming failed: %w", err)
	}

	// Collect full answer for saving
	var fullAnswer string

	// Forward chunks to callback
	for chunk := range chunkChan {
		fullAnswer += chunk.Text

		if err := callback(StreamChunk{
			Text:        chunk.Text,
			Done:        chunk.Done,
			TokensUsed:  chunk.TokensUsed,
			FinishReason: chunk.FinishReason,
		}); err != nil {
			return err
		}
		if chunk.Done {
			break
		}
	}

	// Add messages to conversation
	conv.AddMessage(conversation.RoleUser, req.Query)
	conv.AddMessage(conversation.RoleAssistant, fullAnswer)

	// Save conversation
	if err := s.convRepo.Update(ctx, conv); err != nil {
		s.logger.Warn("failed to update conversation", zap.Error(err))
	}

	return nil
}

