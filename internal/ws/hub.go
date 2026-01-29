// Package ws provides WebSocket connection management and streaming support.
package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Hub manages WebSocket connections and message broadcasting.
type Hub struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	logger      *zap.Logger
}

// NewHub creates a new WebSocket hub.
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		connections: make(map[string]*Connection),
		logger:      logger,
	}
}

// Register adds a new connection to the hub.
func (h *Hub) Register(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[conn.ID] = conn
	h.logger.Info("websocket connection registered", zap.String("connection_id", conn.ID))
}

// Unregister removes a connection from the hub.
func (h *Hub) Unregister(connID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conn, ok := h.connections[connID]; ok {
		delete(h.connections, connID)
		conn.Close()
		h.logger.Info("websocket connection unregistered", zap.String("connection_id", connID))
	}
}

// GetConnection retrieves a connection by ID.
func (h *Hub) GetConnection(connID string) (*Connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.connections[connID]
	return conn, ok
}

// Close closes all connections and cleans up resources.
// This implements the bootstrap.Resource interface for graceful shutdown.
func (h *Hub) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all connections
	for connID, conn := range h.connections {
		conn.Close()
		delete(h.connections, connID)
		h.logger.Debug("closed websocket connection", zap.String("connection_id", connID))
	}

	h.logger.Info("websocket hub closed", zap.Int("connections_closed", len(h.connections)))
	return nil
}

// Connection represents a WebSocket connection.
type Connection struct {
	ID       string
	conn     *websocket.Conn
	send     chan []byte
	hub      *Hub
	logger   *zap.Logger
	cancel   context.CancelFunc // For canceling ongoing operations
	mu       sync.Mutex
	closed   bool
}

// NewConnection creates a new WebSocket connection.
func NewConnection(id string, conn *websocket.Conn, hub *Hub, logger *zap.Logger) *Connection {
	_, cancel := context.WithCancel(context.Background())
	return &Connection{
		ID:     id,
		conn:   conn,
		send:   make(chan []byte, 256),
		hub:    hub,
		logger: logger,
		cancel: cancel,
	}
}

// WriteMessage sends a message to the connection.
func (c *Connection) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	return c.conn.WriteMessage(messageType, data)
}

// SendJSON sends a JSON message to the connection.
func (c *Connection) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, data)
}

// SendText sends a text message to the connection.
func (c *Connection) SendText(text string) error {
	return c.WriteMessage(websocket.TextMessage, []byte(text))
}

// Cancel cancels any ongoing operations for this connection.
func (c *Connection) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
}

// Context returns the connection's context with cancellation support.
func (c *Connection) Context() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel == nil {
		ctx, cancel := context.WithCancel(context.Background())
		c.cancel = cancel
		return ctx
	}
	// Return background context if cancel is already set
	return context.Background()
}

// Close closes the connection.
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if c.cancel != nil {
		c.cancel()
	}
	close(c.send)
	c.conn.Close()
}

// ReadPump reads messages from the WebSocket connection.
func (c *Connection) ReadPump() {
	defer func() {
		c.hub.Unregister(c.ID)
		c.Close()
	}()

	// Set read deadline to allow pong handler
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Warn("websocket read error", zap.Error(err))
			}
			break
		}

		// Handle incoming messages (e.g., cancel requests)
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			if action, ok := msg["action"].(string); ok && action == "cancel" {
				c.Cancel()
				c.SendJSON(map[string]string{"type": "cancelled", "message": "Request cancelled"})
			}
		}
	}
}

// WritePump writes messages to the WebSocket connection.
func (c *Connection) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Send queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

