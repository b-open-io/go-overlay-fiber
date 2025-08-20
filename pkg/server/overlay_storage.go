package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/b-open-io/overlay/beef"
	"github.com/b-open-io/overlay/publish"
	"github.com/b-open-io/overlay/storage"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// OverlayStorageAdapter wraps overlay storage with engine.Storage interface
type OverlayStorageAdapter struct {
	// Enhanced storage with overlay capabilities
	overlayStorage storage.EventDataStorage

	// Configuration
	eventStorageURL string
	beefStorageURL  string

	logger *log.Logger
}

// CreateOverlayStorage creates storage based on connection strings with auto-detection
func CreateOverlayStorage(eventStorageURL, beefStorageURL string) (engine.Storage, error) {
	return CreateOverlayStorageWithDB(eventStorageURL, beefStorageURL, nil)
}

// CreateOverlayStorageWithDB creates overlay storage with explicit database connection
func CreateOverlayStorageWithDB(eventStorageURL, beefStorageURL string, db *sql.DB) (engine.Storage, error) {
	logger := log.Default()

	// Create BEEF storage using overlay factory
	beefStorage, err := beef.CreateBeefStorage(beefStorageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create BEEF storage: %w", err)
	}

	// Create no-op publisher
	publisher := &NoOpPublisher{}

	// Create event data storage using overlay factory
	overlayStorage, err := storage.CreateEventDataStorage(eventStorageURL, beefStorage, publisher)
	if err != nil {
		return nil, fmt.Errorf("failed to create overlay storage from URL '%s': %w", eventStorageURL, err)
	}

	adapter := &OverlayStorageAdapter{
		overlayStorage:  overlayStorage,
		eventStorageURL: eventStorageURL,
		beefStorageURL:  beefStorageURL,
		logger:          logger,
	}

	logger.Printf("Overlay storage created - Event: %s, BEEF: %s, Publisher: NoOp",
		getStorageType(eventStorageURL), getStorageType(beefStorageURL))

	return adapter, nil
}

// NoOpPublisher implements overlay's publish.Publisher interface with no-op behavior
type NoOpPublisher struct{}

func (p *NoOpPublisher) Publish(ctx context.Context, topic string, data string) error {
	// No-op implementation
	_ = ctx
	_ = topic
	_ = data
	return nil
}

// getStorageType returns a human-readable storage type from URL
func getStorageType(storageURL string) string {
	if storageURL == "" {
		return "SQL"
	}

	if strings.HasPrefix(storageURL, "mongodb://") {
		return "MongoDB"
	}
	if strings.HasPrefix(storageURL, "sqlite://") || strings.HasSuffix(storageURL, ".db") {
		return "SQLite"
	}
	if strings.HasSuffix(storageURL, "/") {
		return "Filesystem"
	}

	// Try parsing as URL
	if u, err := url.Parse(storageURL); err == nil && u.Scheme != "" {
		return strings.ToUpper(u.Scheme)
	}

	return "Filesystem"
}

// GetBeefStorage returns the BEEF storage instance
func (o *OverlayStorageAdapter) GetBeefStorage() beef.BeefStorage {
	return o.overlayStorage.GetBeefStorage()
}

// GetPublisher returns the publisher instance
func (o *OverlayStorageAdapter) GetPublisher() publish.Publisher {
	return o.overlayStorage.GetPublisher()
}

// Implement engine.Storage interface by delegating to overlayStorage
func (o *OverlayStorageAdapter) InsertOutput(ctx context.Context, output *engine.Output) error {
	return o.overlayStorage.InsertOutput(ctx, output)
}

func (o *OverlayStorageAdapter) FindOutput(ctx context.Context, outpoint *transaction.Outpoint, topic *string, spent *bool, includeBEEF bool) (*engine.Output, error) {
	return o.overlayStorage.FindOutput(ctx, outpoint, topic, spent, includeBEEF)
}

func (o *OverlayStorageAdapter) FindOutputs(ctx context.Context, outpoints []*transaction.Outpoint, topic string, spent *bool, includeBEEF bool) ([]*engine.Output, error) {
	return o.overlayStorage.FindOutputs(ctx, outpoints, topic, spent, includeBEEF)
}

func (o *OverlayStorageAdapter) FindOutputsForTransaction(ctx context.Context, txid *chainhash.Hash, includeBEEF bool) ([]*engine.Output, error) {
	return o.overlayStorage.FindOutputsForTransaction(ctx, txid, includeBEEF)
}

func (o *OverlayStorageAdapter) FindUTXOsForTopic(ctx context.Context, topic string, since float64, limit uint32, includeBEEF bool) ([]*engine.Output, error) {
	return o.overlayStorage.FindUTXOsForTopic(ctx, topic, since, limit, includeBEEF)
}

func (o *OverlayStorageAdapter) DeleteOutput(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	return o.overlayStorage.DeleteOutput(ctx, outpoint, topic)
}

func (o *OverlayStorageAdapter) MarkUTXOsAsSpent(ctx context.Context, outpoints []*transaction.Outpoint, topic string, spendTxid *chainhash.Hash) error {
	return o.overlayStorage.MarkUTXOsAsSpent(ctx, outpoints, topic, spendTxid)
}

func (o *OverlayStorageAdapter) UpdateConsumedBy(ctx context.Context, outpoint *transaction.Outpoint, topic string, consumedBy []*transaction.Outpoint) error {
	return o.overlayStorage.UpdateConsumedBy(ctx, outpoint, topic, consumedBy)
}

func (o *OverlayStorageAdapter) UpdateTransactionBEEF(ctx context.Context, txid *chainhash.Hash, beef []byte) error {
	return o.overlayStorage.UpdateTransactionBEEF(ctx, txid, beef)
}

func (o *OverlayStorageAdapter) UpdateOutputBlockHeight(ctx context.Context, outpoint *transaction.Outpoint, topic string, blockHeight uint32, blockIndex uint64, ancillaryBeef []byte) error {
	return o.overlayStorage.UpdateOutputBlockHeight(ctx, outpoint, topic, blockHeight, blockIndex, ancillaryBeef)
}

func (o *OverlayStorageAdapter) InsertAppliedTransaction(ctx context.Context, tx *overlay.AppliedTransaction) error {
	return o.overlayStorage.InsertAppliedTransaction(ctx, tx)
}

func (o *OverlayStorageAdapter) DoesAppliedTransactionExist(ctx context.Context, tx *overlay.AppliedTransaction) (bool, error) {
	return o.overlayStorage.DoesAppliedTransactionExist(ctx, tx)
}

func (o *OverlayStorageAdapter) UpdateLastInteraction(ctx context.Context, host string, topic string, since float64) error {
	return o.overlayStorage.UpdateLastInteraction(ctx, host, topic, since)
}

func (o *OverlayStorageAdapter) GetLastInteraction(ctx context.Context, host string, topic string) (float64, error) {
	return o.overlayStorage.GetLastInteraction(ctx, host, topic)
}

// Additional overlay-specific functionality
func (o *OverlayStorageAdapter) GetTransactionsByTopicAndHeight(ctx context.Context, topic string, height uint32) ([]*storage.TransactionData, error) {
	return o.overlayStorage.GetTransactionsByTopicAndHeight(ctx, topic, height)
}

func (o *OverlayStorageAdapter) SaveEvents(ctx context.Context, outpoint *transaction.Outpoint, events []string, height uint32, idx uint64, data interface{}) error {
	return o.overlayStorage.SaveEvents(ctx, outpoint, events, height, idx, data)
}

func (o *OverlayStorageAdapter) FindEvents(ctx context.Context, outpoint *transaction.Outpoint) ([]string, error) {
	return o.overlayStorage.FindEvents(ctx, outpoint)
}

func (o *OverlayStorageAdapter) LookupOutpoints(ctx context.Context, question *storage.EventQuestion, includeData ...bool) ([]*storage.OutpointResult, error) {
	return o.overlayStorage.LookupOutpoints(ctx, question, includeData...)
}

func (o *OverlayStorageAdapter) GetOutputData(ctx context.Context, outpoint *transaction.Outpoint) (interface{}, error) {
	return o.overlayStorage.GetOutputData(ctx, outpoint)
}
