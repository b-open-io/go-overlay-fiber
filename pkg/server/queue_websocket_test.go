package server

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueueManagerCreation(t *testing.T) {
	// Create a minimal engine for testing
	eng := &engine.Engine{}

	// Create queue manager
	qm, err := NewQueueManager(eng, &NoOpPublisher{}, log.Default())

	// Should not fail
	assert.NoError(t, err)
	assert.NotNil(t, qm)

	// Check status
	status := qm.GetStatus()
	assert.NotNil(t, status)
	assert.Contains(t, status, "running")
	assert.Contains(t, status, "available")
}

func TestWebSocketManagerCreation(t *testing.T) {
	// Create WebSocket manager
	wm, err := NewWebSocketManager(&NoOpPublisher{}, log.Default())

	// Should not fail
	assert.NoError(t, err)
	assert.NotNil(t, wm)

	// Check stats
	stats := wm.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "running")
	assert.Contains(t, stats, "connected_clients")
	assert.Equal(t, 0, stats["connected_clients"])
}

func TestTransactionProcessor(t *testing.T) {
	// Create a minimal engine with nil storage for testing
	eng := &engine.Engine{}

	// Create transaction processor
	processor := NewOverlayTransactionProcessor(eng, &NoOpPublisher{}, log.Default())
	assert.NotNil(t, processor)

	// Test processing a transaction with no storage should return error
	ctx := context.Background()
	txid := chainhash.Hash{}

	topics, err := processor.ProcessTransaction(ctx, &txid)
	assert.Error(t, err) // Should error because no storage is configured
	assert.Contains(t, err.Error(), "engine or storage not configured")
	assert.Nil(t, topics)
}

func TestBackgroundServicesLifecycle(t *testing.T) {
	// Create queue manager
	eng := &engine.Engine{}
	qm, err := NewQueueManager(eng, &NoOpPublisher{}, log.Default())
	require.NoError(t, err)

	// Create WebSocket manager
	wm, err := NewWebSocketManager(&NoOpPublisher{}, log.Default())
	require.NoError(t, err)

	// Start services
	err = qm.Start()
	assert.NoError(t, err)

	err = wm.Start()
	assert.NoError(t, err)

	// Give them a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check they're running
	qmStatus := qm.GetStatus()
	wmStats := wm.GetStats()

	if qmStatus["available"].(bool) {
		assert.True(t, qmStatus["running"].(bool))
	}

	// WebSocket manager should always be running
	assert.True(t, wmStats["running"].(bool))

	// Stop services
	err = qm.Stop()
	assert.NoError(t, err)

	err = wm.Stop()
	assert.NoError(t, err)

	// Check they're stopped
	qmStatusAfter := qm.GetStatus()
	wmStatsAfter := wm.GetStats()

	// Both should be stopped now
	assert.False(t, qmStatusAfter["running"].(bool))
	assert.False(t, wmStatsAfter["running"].(bool))
}

func TestWebSocketMessageTypes(t *testing.T) {
	// Test WebSocket message structure
	msg := WebSocketMessage{
		Type:      "test",
		Topic:     "topic1",
		Data:      map[string]string{"key": "value"},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	assert.Equal(t, "test", msg.Type)
	assert.Equal(t, "topic1", msg.Topic)
	assert.NotNil(t, msg.Data)
	assert.NotEmpty(t, msg.Timestamp)
}

func TestSubscriptionRequest(t *testing.T) {
	// Test subscription request structure
	req := SubscriptionRequest{
		Action: "subscribe",
		Topics: []string{"topic1", "topic2"},
	}

	assert.Equal(t, "subscribe", req.Action)
	assert.Len(t, req.Topics, 2)
	assert.Contains(t, req.Topics, "topic1")
	assert.Contains(t, req.Topics, "topic2")
}
