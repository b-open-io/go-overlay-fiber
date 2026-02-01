package walletconfig

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// WalletConfigStorageEngine defines the interface for WalletConfig storage operations
type WalletConfigStorageEngine interface {
	StoreRecord(ctx context.Context, txid string, outputIndex int, registration *WalletConfigRegistration) error
	DeleteRecord(ctx context.Context, txid string, outputIndex int) error
	FindByConfigID(ctx context.Context, configID string, registryOperators []string) ([]UTXOReference, error)
	FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error)
	FindByWAB(ctx context.Context, wab string, registryOperators []string) ([]UTXOReference, error)
	FindByStorage(ctx context.Context, storage string, registryOperators []string) ([]UTXOReference, error)
	FindByMessagebox(ctx context.Context, messagebox string, registryOperators []string) ([]UTXOReference, error)
	ListAll(ctx context.Context, registryOperators []string) ([]UTXOReference, error)
}

// WalletConfigStorage handles WalletConfig record storage in MongoDB
type WalletConfigStorage struct {
	collection *mongo.Collection
}

// Ensure WalletConfigStorage implements WalletConfigStorageEngine
var _ WalletConfigStorageEngine = (*WalletConfigStorage)(nil)

// NewWalletConfigStorage creates a new WalletConfig storage instance
func NewWalletConfigStorage(db *mongo.Database) *WalletConfigStorage {
	collection := db.Collection("walletConfigRecords")

	return &WalletConfigStorage{
		collection: collection,
	}
}

// StoreRecord inserts a new WalletConfig record, preventing duplicates
func (s *WalletConfigStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration *WalletConfigRegistration) error {
	// Check if a record with the same field values already exists (excluding txid/outputIndex)
	filter := bson.M{
		"registration.configID":         registration.ConfigID,
		"registration.name":             registration.Name,
		"registration.icon":             registration.Icon,
		"registration.wab":              registration.WAB,
		"registration.storage":          registration.Storage,
		"registration.messagebox":       registration.Messagebox,
		"registration.legal":            registration.Legal,
		"registration.registryOperator": registration.RegistryOperator,
	}

	count, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to check for duplicate WalletConfig record: %w", err)
	}

	// Only insert if no duplicate exists
	if count == 0 {
		record := &WalletConfigRecord{
			Txid:         txid,
			OutputIndex:  outputIndex,
			Registration: registration,
			CreatedAt:    time.Now(),
		}

		_, err := s.collection.InsertOne(ctx, record)
		if err != nil {
			return fmt.Errorf("failed to insert WalletConfig record: %w", err)
		}
	}

	return nil
}

// DeleteRecord deletes a WalletConfig record by txid and output index
func (s *WalletConfigStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete WalletConfig record: %w", err)
	}
	return nil
}

// FindByConfigID fetches records by configID and registry operators
func (s *WalletConfigStorage) FindByConfigID(ctx context.Context, configID string, registryOperators []string) ([]UTXOReference, error) {
	query := bson.M{
		"registration.configID":         configID,
		"registration.registryOperator": bson.M{"$in": registryOperators},
	}
	return s.findRecordWithQuery(ctx, query)
}

// FindByName fetches records by name (fuzzy search) and registry operators
func (s *WalletConfigStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
	// Escape regex metacharacters for safe fuzzy search
	escaped := regexp.QuoteMeta(name)
	// Create fuzzy regex pattern (insert .* between each character)
	fuzzyPattern := ""
	for i, char := range escaped {
		if i > 0 {
			fuzzyPattern += ".*"
		}
		fuzzyPattern += string(char)
	}

	query := bson.M{
		"registration.name": bson.M{
			"$regex":   fuzzyPattern,
			"$options": "i", // case-insensitive
		},
		"registration.registryOperator": bson.M{"$in": registryOperators},
	}

	return s.findRecordWithQuery(ctx, query)
}

// FindByWAB fetches records by WAB URL and registry operators
func (s *WalletConfigStorage) FindByWAB(ctx context.Context, wab string, registryOperators []string) ([]UTXOReference, error) {
	query := bson.M{
		"registration.wab":              wab,
		"registration.registryOperator": bson.M{"$in": registryOperators},
	}
	return s.findRecordWithQuery(ctx, query)
}

// FindByStorage fetches records by storage URL and registry operators
func (s *WalletConfigStorage) FindByStorage(ctx context.Context, storage string, registryOperators []string) ([]UTXOReference, error) {
	query := bson.M{
		"registration.storage":          storage,
		"registration.registryOperator": bson.M{"$in": registryOperators},
	}
	return s.findRecordWithQuery(ctx, query)
}

// FindByMessagebox fetches records by messagebox URL and registry operators
func (s *WalletConfigStorage) FindByMessagebox(ctx context.Context, messagebox string, registryOperators []string) ([]UTXOReference, error) {
	query := bson.M{
		"registration.messagebox":       messagebox,
		"registration.registryOperator": bson.M{"$in": registryOperators},
	}
	return s.findRecordWithQuery(ctx, query)
}

// ListAll fetches all records from specified registry operators
func (s *WalletConfigStorage) ListAll(ctx context.Context, registryOperators []string) ([]UTXOReference, error) {
	query := bson.M{
		"registration.registryOperator": bson.M{"$in": registryOperators},
	}
	return s.findRecordWithQuery(ctx, query)
}

// findRecordWithQuery is a helper function for querying from the database
func (s *WalletConfigStorage) findRecordWithQuery(ctx context.Context, filter interface{}) ([]UTXOReference, error) {
	opts := options.Find().
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query WalletConfig records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record WalletConfigRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode WalletConfig record: %w", err)
		}

		results = append(results, UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return results, nil
}
