package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/b-open-io/overlay/processor"
	"github.com/b-open-io/overlay/publish"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/redis/go-redis/v9"
)

// OverlayTransactionProcessor implements the processor.TransactionProcessor interface
// It integrates with the existing Engine to process transactions from the queue
type OverlayTransactionProcessor struct {
	engine    *engine.Engine
	publisher publish.Publisher
	logger    *log.Logger
}

// NewOverlayTransactionProcessor creates a new transaction processor
func NewOverlayTransactionProcessor(engine *engine.Engine, publisher publish.Publisher, logger *log.Logger) *OverlayTransactionProcessor {
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

// QueueManager manages the queue processor and background services
type QueueManager struct {
	processor   *processor.QueueProcessor
	redisClient *redis.Client
	config      *processor.ProcessorConfig
	logger      *log.Logger

	// Context and cancellation for background processing
	ctx    context.Context
	cancel context.CancelFunc

	// Status tracking
	running bool
}

// NewQueueManager creates a new queue manager with Redis connection
func NewQueueManager(engine *engine.Engine, publisher publish.Publisher, logger *log.Logger) (*QueueManager, error) {
	if logger == nil {
		logger = log.Default()
	}

	// Get Redis connection string from environment
	redisURL := os.Getenv("REDIS_QUEUE_URL")
	if redisURL == "" {
		redisURL = os.Getenv("REDIS_URL")
	}
	if redisURL == "" {
		redisURL = "redis://localhost:6379" // Default Redis URL
	}

	// Parse Redis options
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL %s: %w", redisURL, err)
	}

	// Create Redis client
	redisClient := redis.NewClient(opts)

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Printf("Warning: Redis connection failed, queue processing will be disabled: %v", err)
		// Don't return error - allow server to start without queue processing
		return &QueueManager{
			logger:  logger,
			running: false,
		}, nil
	}

	// Create processor configuration
	config := processor.DefaultProcessorConfig()

	// Override with environment variables if present
	if queueName := os.Getenv("REDIS_QUEUE_NAME"); queueName != "" {
		config.QueueName = queueName
	} else {
		config.QueueName = "overlay_tx_queue"
	}

	if concurrencyStr := os.Getenv("QUEUE_CONCURRENCY"); concurrencyStr != "" {
		if concurrency, err := strconv.Atoi(concurrencyStr); err == nil && concurrency > 0 {
			config.Concurrency = concurrency
		}
	}

	if batchSizeStr := os.Getenv("QUEUE_BATCH_SIZE"); batchSizeStr != "" {
		if batchSize, err := strconv.ParseInt(batchSizeStr, 10, 64); err == nil && batchSize > 0 {
			config.BatchSize = batchSize
		}
	}

	if sleepStr := os.Getenv("QUEUE_EMPTY_SLEEP"); sleepStr != "" {
		if sleep, err := time.ParseDuration(sleepStr); err == nil {
			config.EmptyQueueSleep = sleep
		}
	}

	// Create transaction processor
	txProcessor := NewOverlayTransactionProcessor(engine, publisher, logger)

	// Create queue processor
	queueProcessor := processor.NewQueueProcessor(config, redisClient, txProcessor)

	return &QueueManager{
		processor:   queueProcessor,
		redisClient: redisClient,
		config:      config,
		logger:      logger,
		running:     false,
	}, nil
}

// Start begins background queue processing
func (qm *QueueManager) Start() error {
	if qm.processor == nil {
		qm.logger.Printf("Queue processor not available - skipping background processing")
		return nil
	}

	if qm.running {
		return fmt.Errorf("queue manager is already running")
	}

	// Create context for background processing
	qm.ctx, qm.cancel = context.WithCancel(context.Background())

	// Start queue processing in background
	go func() {
		qm.logger.Printf("Starting background queue processing for queue '%s'", qm.config.QueueName)

		if err := qm.processor.Start(qm.ctx); err != nil && err != context.Canceled {
			qm.logger.Printf("Queue processor error: %v", err)
		}

		qm.logger.Printf("Background queue processing stopped")
	}()

	qm.running = true
	qm.logger.Printf("Queue manager started successfully")
	return nil
}

// Stop gracefully stops background queue processing
func (qm *QueueManager) Stop() error {
	if !qm.running {
		return nil
	}

	qm.logger.Printf("Stopping queue manager...")

	if qm.cancel != nil {
		qm.cancel()
	}

	// Close Redis connection
	if qm.redisClient != nil {
		if err := qm.redisClient.Close(); err != nil {
			qm.logger.Printf("Error closing Redis connection: %v", err)
		}
	}

	qm.running = false
	qm.logger.Printf("Queue manager stopped")
	return nil
}

// GetQueueLength returns the current length of the processing queue
func (qm *QueueManager) GetQueueLength() (int64, error) {
	if qm.processor == nil {
		return 0, fmt.Errorf("queue processor not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return qm.processor.GetQueueLength(ctx)
}

// GetStatus returns the current status of the queue manager
func (qm *QueueManager) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running":   qm.running,
		"available": qm.processor != nil,
		"redis_url": os.Getenv("REDIS_QUEUE_URL"),
	}

	if qm.config != nil {
		status["config"] = map[string]interface{}{
			"queue_name":        qm.config.QueueName,
			"concurrency":       qm.config.Concurrency,
			"batch_size":        qm.config.BatchSize,
			"empty_queue_sleep": qm.config.EmptyQueueSleep.String(),
		}
	}

	// Get queue length if available
	if qm.processor != nil {
		if length, err := qm.GetQueueLength(); err == nil {
			status["queue_length"] = length
		}
	}

	return status
}

// EnqueueTransaction adds a transaction to the processing queue
func (qm *QueueManager) EnqueueTransaction(txid *chainhash.Hash, blockHeight uint32, blockIndex uint64) error {
	if qm.redisClient == nil {
		return fmt.Errorf("Redis client not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Add to Redis queue with score based on block height and index
	score := float64(blockHeight)*1e9 + float64(blockIndex)

	err := qm.redisClient.ZAdd(ctx, qm.config.QueueName, redis.Z{
		Member: txid.String(),
		Score:  score,
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to enqueue transaction %s: %w", txid.String(), err)
	}

	qm.logger.Printf("Enqueued transaction %s for processing (height: %d, index: %d)",
		txid.String(), blockHeight, blockIndex)

	return nil
}
