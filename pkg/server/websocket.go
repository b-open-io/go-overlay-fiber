package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/b-open-io/overlay/pubsub"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// WebSocketMessage represents a message sent to WebSocket clients
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Topic     string      `json:"topic"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	ID       string
	Conn     *websocket.Conn
	Topics   map[string]bool
	LastPing time.Time
	mutex    sync.RWMutex
}

// SubscriptionRequest represents a client subscription request
type SubscriptionRequest struct {
	Action string   `json:"action"` // "subscribe" or "unsubscribe"
	Topics []string `json:"topics"`
}

// WebSocketManager manages WebSocket connections and subscriptions
type WebSocketManager struct {
	clients     map[string]*WebSocketClient
	clientMutex sync.RWMutex
	publisher   pubsub.PubSub
	logger      *log.Logger

	// Context and cancellation for background services
	ctx    context.Context
	cancel context.CancelFunc

	// Status tracking
	running bool
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager(publisher pubsub.PubSub, logger *log.Logger) (*WebSocketManager, error) {
	if logger == nil {
		logger = log.Default()
	}

	wm := &WebSocketManager{
		clients:   make(map[string]*WebSocketClient),
		publisher: publisher,
		logger:    logger,
		running:   false,
	}

	return wm, nil
}

// Start begins WebSocket management
func (wm *WebSocketManager) Start() error {
	if wm.running {
		return fmt.Errorf("WebSocket manager is already running")
	}

	// Create context for background services
	wm.ctx, wm.cancel = context.WithCancel(context.Background())

	// Start client cleanup routine
	go wm.clientCleanupLoop()

	wm.running = true
	wm.logger.Printf("WebSocket manager started")
	return nil
}

// Stop gracefully stops WebSocket management
func (wm *WebSocketManager) Stop() error {
	if !wm.running {
		return nil
	}

	wm.logger.Printf("Stopping WebSocket manager...")

	if wm.cancel != nil {
		wm.cancel()
	}

	// Close all client connections
	wm.clientMutex.Lock()
	for _, client := range wm.clients {
		client.Conn.Close()
	}
	wm.clients = make(map[string]*WebSocketClient)
	wm.clientMutex.Unlock()

	wm.running = false
	wm.logger.Printf("WebSocket manager stopped")
	return nil
}

// HandleWebSocketUpgrade handles WebSocket upgrade requests
func (wm *WebSocketManager) HandleWebSocketUpgrade(c *fiber.Ctx) error {
	// Check if connection is actually a WebSocket upgrade
	if !websocket.IsWebSocketUpgrade(c) {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "WebSocket upgrade required",
		})
	}

	return websocket.New(wm.handleWebSocketConnection)(c)
}

// handleWebSocketConnection handles individual WebSocket connections
func (wm *WebSocketManager) handleWebSocketConnection(c *websocket.Conn) {
	// Generate unique client ID
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())

	// Create client
	client := &WebSocketClient{
		ID:       clientID,
		Conn:     c,
		Topics:   make(map[string]bool),
		LastPing: time.Now(),
	}

	// Register client
	wm.clientMutex.Lock()
	wm.clients[clientID] = client
	wm.clientMutex.Unlock()

	wm.logger.Printf("WebSocket client connected: %s", clientID)

	// Send welcome message
	welcomeMsg := WebSocketMessage{
		Type:      "welcome",
		Topic:     "system",
		Data:      map[string]string{"client_id": clientID},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	wm.sendToClient(client, welcomeMsg)

	// Handle messages from client
	defer func() {
		// Cleanup on disconnect
		wm.clientMutex.Lock()
		delete(wm.clients, clientID)
		wm.clientMutex.Unlock()
		wm.logger.Printf("WebSocket client disconnected: %s", clientID)
	}()

	for {
		var msg SubscriptionRequest
		err := c.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				wm.logger.Printf("WebSocket client %s error: %v", clientID, err)
			}
			break
		}

		// Update last ping
		client.mutex.Lock()
		client.LastPing = time.Now()
		client.mutex.Unlock()

		// Handle subscription request
		wm.handleSubscriptionRequest(client, msg)
	}
}

// handleSubscriptionRequest processes client subscription requests
func (wm *WebSocketManager) handleSubscriptionRequest(client *WebSocketClient, request SubscriptionRequest) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	switch request.Action {
	case "subscribe":
		for _, topic := range request.Topics {
			client.Topics[topic] = true
			wm.logger.Printf("Client %s subscribed to topic: %s", client.ID, topic)
		}

		// Send confirmation
		response := WebSocketMessage{
			Type:      "subscription_confirmed",
			Topic:     "system",
			Data:      map[string]interface{}{"action": "subscribe", "topics": request.Topics},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		wm.sendToClient(client, response)

	case "unsubscribe":
		for _, topic := range request.Topics {
			delete(client.Topics, topic)
			wm.logger.Printf("Client %s unsubscribed from topic: %s", client.ID, topic)
		}

		// Send confirmation
		response := WebSocketMessage{
			Type:      "subscription_confirmed",
			Topic:     "system",
			Data:      map[string]interface{}{"action": "unsubscribe", "topics": request.Topics},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		wm.sendToClient(client, response)

	case "ping":
		// Respond with pong
		response := WebSocketMessage{
			Type:      "pong",
			Topic:     "system",
			Data:      map[string]string{"message": "pong"},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		wm.sendToClient(client, response)

	default:
		// Send error for unknown actions
		response := WebSocketMessage{
			Type:      "error",
			Topic:     "system",
			Data:      map[string]string{"message": "Unknown action: " + request.Action},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		wm.sendToClient(client, response)
	}
}

// BroadcastToTopic broadcasts a message to all clients subscribed to a topic
func (wm *WebSocketManager) BroadcastToTopic(topic string, message WebSocketMessage) {
	wm.clientMutex.RLock()
	defer wm.clientMutex.RUnlock()

	count := 0
	for _, client := range wm.clients {
		client.mutex.RLock()
		subscribed := client.Topics[topic] || client.Topics["all"]
		client.mutex.RUnlock()

		if subscribed {
			wm.sendToClient(client, message)
			count++
		}
	}

	if count > 0 {
		wm.logger.Printf("Broadcasted message to %d clients on topic: %s", count, topic)
	}
}

// BroadcastToAll broadcasts a message to all connected clients
func (wm *WebSocketManager) BroadcastToAll(message WebSocketMessage) {
	wm.clientMutex.RLock()
	defer wm.clientMutex.RUnlock()

	for _, client := range wm.clients {
		wm.sendToClient(client, message)
	}

	wm.logger.Printf("Broadcasted message to %d clients", len(wm.clients))
}

// sendToClient sends a message to a specific client
func (wm *WebSocketManager) sendToClient(client *WebSocketClient, message WebSocketMessage) {
	if err := client.Conn.WriteJSON(message); err != nil {
		wm.logger.Printf("Error sending message to client %s: %v", client.ID, err)
		// Client will be cleaned up by the connection handler
	}
}

// clientCleanupLoop periodically cleans up inactive clients
func (wm *WebSocketManager) clientCleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-ticker.C:
			wm.cleanupInactiveClients()
		}
	}
}

// cleanupInactiveClients removes clients that haven't pinged recently
func (wm *WebSocketManager) cleanupInactiveClients() {
	cutoff := time.Now().Add(-5 * time.Minute) // 5 minutes timeout

	wm.clientMutex.Lock()
	defer wm.clientMutex.Unlock()

	for id, client := range wm.clients {
		client.mutex.RLock()
		inactive := client.LastPing.Before(cutoff)
		client.mutex.RUnlock()

		if inactive {
			client.Conn.Close()
			delete(wm.clients, id)
			wm.logger.Printf("Cleaned up inactive client: %s", id)
		}
	}
}

// GetStats returns WebSocket manager statistics
func (wm *WebSocketManager) GetStats() map[string]interface{} {
	wm.clientMutex.RLock()
	defer wm.clientMutex.RUnlock()

	// Count subscriptions by topic
	topicCounts := make(map[string]int)
	for _, client := range wm.clients {
		client.mutex.RLock()
		for topic := range client.Topics {
			topicCounts[topic]++
		}
		client.mutex.RUnlock()
	}

	return map[string]interface{}{
		"running":             wm.running,
		"connected_clients":   len(wm.clients),
		"topic_subscriptions": topicCounts,
	}
}
