// Package ws provides WebSocket handlers for streaming RAG operations.
package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/service"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now; should be restricted in production
		return true
	},
}

// Handler handles WebSocket connections for streaming RAG operations.
type Handler struct {
	hub              *Hub
	searchService    *service.SearchService
	conversationService *service.ConversationService
	logger           *zap.Logger
}

// NewHandler creates a new WebSocket handler.
func NewHandler(
	hub *Hub,
	searchService *service.SearchService,
	conversationService *service.ConversationService,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		hub:                hub,
		searchService:      searchService,
		conversationService: conversationService,
		logger:             logger,
	}
}

// HandleWebSocket handles WebSocket upgrade and connection management.
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade websocket", zap.Error(err))
		return
	}

	connID := uuid.New().String()
	connection := NewConnection(connID, conn, h.hub, h.logger)
	h.hub.Register(connection)

	// Start connection pumps
	go connection.WritePump()
	go connection.ReadPump()

	// Handle incoming messages
	go h.handleMessages(connection)
}

// handleMessages processes incoming WebSocket messages.
func (h *Handler) handleMessages(conn *Connection) {
	for {
		_, message, err := conn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Warn("websocket read error", zap.Error(err))
			}
			break
		}

		var req map[string]interface{}
		if err := json.Unmarshal(message, &req); err != nil {
			conn.SendJSON(map[string]interface{}{
				"type":  "error",
				"error": "invalid message format",
			})
			continue
		}

		action, _ := req["action"].(string)
		switch action {
		case "search":
			h.handleSearch(conn, req)
		case "conversation":
			h.handleConversation(conn, req)
		case "cancel":
			conn.Cancel()
			conn.SendJSON(map[string]string{"type": "cancelled"})
		default:
			conn.SendJSON(map[string]interface{}{
				"type":  "error",
				"error": "unknown action: " + action,
			})
		}
	}
}

// handleSearch handles streaming search requests.
func (h *Handler) handleSearch(conn *Connection, req map[string]interface{}) {
	query, _ := req["query"].(string)
	if query == "" {
		conn.SendJSON(map[string]interface{}{
			"type":  "error",
			"error": "query is required",
		})
		return
	}

	topK := 10
	if tk, ok := req["top_k"].(float64); ok {
		topK = int(tk)
	}

	rerank := false
	if r, ok := req["rerank"].(bool); ok {
		rerank = r
	}

	provider := ""
	if p, ok := req["provider"].(string); ok {
		provider = p
	}

	filter := make(map[string]string)
	if f, ok := req["filter"].(map[string]interface{}); ok {
		for k, v := range f {
			if s, ok := v.(string); ok {
				filter[k] = s
			}
		}
	}

	searchReq := service.SearchRequest{
		Query:    query,
		TopK:     topK,
		Filter:   filter,
		Rerank:   rerank,
		Provider: provider,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up cancellation
	go func() {
		<-conn.Context().Done()
		cancel()
	}()

	// Perform streaming search
	err := h.searchService.SearchStream(ctx, searchReq, func(chunk service.StreamChunk) error {
		return conn.SendJSON(map[string]interface{}{
			"type":  "chunk",
			"text":  chunk.Text,
			"done":  chunk.Done,
			"tokens": chunk.TokensUsed,
		})
	})

	if err != nil {
		conn.SendJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	conn.SendJSON(map[string]interface{}{
		"type": "done",
	})
}

// handleConversation handles streaming conversation requests.
func (h *Handler) handleConversation(conn *Connection, req map[string]interface{}) {
	conversationID, _ := req["conversation_id"].(string)
	query, _ := req["query"].(string)

	if conversationID == "" {
		conn.SendJSON(map[string]interface{}{
			"type":  "error",
			"error": "conversation_id is required",
		})
		return
	}

	if query == "" {
		conn.SendJSON(map[string]interface{}{
			"type":  "error",
			"error": "query is required",
		})
		return
	}

	topK := 10
	if tk, ok := req["top_k"].(float64); ok {
		topK = int(tk)
	}

	rerank := false
	if r, ok := req["rerank"].(bool); ok {
		rerank = r
	}

	provider := ""
	if p, ok := req["provider"].(string); ok {
		provider = p
	}

	filter := make(map[string]string)
	if f, ok := req["filter"].(map[string]interface{}); ok {
		for k, v := range f {
			if s, ok := v.(string); ok {
				filter[k] = s
			}
		}
	}

	convReq := service.SendMessageRequest{
		ConversationID: conversationID,
		Query:          query,
		TopK:           topK,
		Filter:         filter,
		Rerank:         rerank,
		Provider:       provider,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up cancellation
	go func() {
		<-conn.Context().Done()
		cancel()
	}()

	// Perform streaming conversation
	err := h.conversationService.SendMessageStream(ctx, convReq, func(chunk service.StreamChunk) error {
		return conn.SendJSON(map[string]interface{}{
			"type":  "chunk",
			"text":  chunk.Text,
			"done":  chunk.Done,
			"tokens": chunk.TokensUsed,
		})
	})

	if err != nil {
		conn.SendJSON(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	conn.SendJSON(map[string]interface{}{
		"type": "done",
	})
}

// SetReadDeadline sets the read deadline for WebSocket connections.
func SetReadDeadline(deadline time.Time) {
	upgrader.ReadBufferSize = 1024
}

