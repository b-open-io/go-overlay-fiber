package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/b-open-io/overlay/pubsub"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// OverlayTransactionProcessor implements the processor.TransactionProcessor interface
// It integrates with the existing Engine to process transactions from the queue
type OverlayTransactionProcessor struct {
	engine    *engine.Engine
	publisher pubsub.PubSub
	logger    *log.Logger
}

// NewOverlayTransactionProcessor creates a new transaction processor
func NewOverlayTransactionProcessor(engine *engine.Engine, publisher pubsub.PubSub, logger *log.Logger) *OverlayTransactionProcessor {
	if logger == nil {
		logger = log.Default()
	}

	return &OverlayTransactionProcessor{
		engine:    engine,
		publisher: publisher,
		logger:    logger,
	}
}

// ProcessTransaction processes a single transaction by its ID
// Returns a list of topics/tokens that this transaction belongs to
func (p *OverlayTransactionProcessor) ProcessTransaction(ctx context.Context, txid *chainhash.Hash) ([]string, error) {
	p.logger.Printf("Processing transaction %s", txid.String())

	// Try to find outputs for this transaction in the engine storage
	if p.engine == nil || p.engine.Storage == nil {
		return nil, fmt.Errorf("engine or storage not configured")
	}

	// Find all outputs for this transaction
	outputs, err := p.engine.Storage.FindOutputsForTransaction(ctx, txid, true)
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs for transaction %s: %w", txid.String(), err)
	}

	if len(outputs) == 0 {
		p.logger.Printf("No outputs found for transaction %s", txid.String())
		return []string{}, nil
	}

	// Collect unique topics from all outputs
	topicSet := make(map[string]bool)
	for _, output := range outputs {
		if output.Topic != "" {
			topicSet[output.Topic] = true
		}
	}

	// Convert to slice
	topics := make([]string, 0, len(topicSet))
	for topic := range topicSet {
		topics = append(topics, topic)
	}

	// Publish processing event if publisher is available
	if p.publisher != nil {
		eventData := fmt.Sprintf(`{"txid":"%s","topics":%v,"processed_at":"%s"}`,
			txid.String(), topics, time.Now().UTC().Format(time.RFC3339))

		for _, topic := range topics {
			if err := p.publisher.Publish(ctx, fmt.Sprintf("tx_processed:%s", topic), eventData); err != nil {
				p.logger.Printf("Failed to publish processing event for topic %s: %v", topic, err)
			}
		}

		// Also publish to global processing channel
		if err := p.publisher.Publish(ctx, "tx_processed:all", eventData); err != nil {
			p.logger.Printf("Failed to publish to global processing channel: %v", err)
		}
	}

	p.logger.Printf("Successfully processed transaction %s for topics: %v", txid.String(), topics)
	return topics, nil
}

// QueueManager manages background services
type QueueManager struct {
	logger *log.Logger

	// Status tracking
	running bool
}

// NewQueueManager creates a new queue manager
func NewQueueManager(engine *engine.Engine, publisher pubsub.PubSub, logger *log.Logger) (*QueueManager, error) {
	if logger == nil {
		logger = log.Default()
	}

	logger.Printf("Queue manager created")

	return &QueueManager{
		logger:  logger,
		running: false,
	}, nil
}

// Start begins background queue process
func (qm *QueueManager) Start() error {
	if qm.running {
		return fmt.Errorf("queue manager is already running")
	}

	qm.logger.Printf("Queue manager started")
	qm.running = true
	return nil
}

// Stop gracefully stops background queue processing
func (qm *QueueManager) Stop() error {
	if !qm.running {
		return nil
	}

	qm.logger.Printf("Queue manager stopped")
	qm.running = false
	return nil
}

// GetQueueLength returns the current length of the processing queue
func (qm *QueueManager) GetQueueLength() (int64, error) {
	return 0, nil
}

// GetStatus returns the current status of the queue manager
func (qm *QueueManager) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running":      qm.running,
		"available":    false,
		"queue_length": 0,
	}

	return status
}

// EnqueueTransaction adds a transaction to the processing queue
func (qm *QueueManager) EnqueueTransaction(txid *chainhash.Hash, blockHeight uint32, blockIndex uint64) error {
	qm.logger.Printf("Transaction %s would be enqueued (height: %d, index: %d)",
		txid.String(), blockHeight, blockIndex)
	return nil
}
