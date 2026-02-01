package apps

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AppsStorageEngine defines the interface for Apps storage operations
type AppsStorageEngine interface {
	StoreRecord(ctx context.Context, txid string, outputIndex int, metadata *PublishedAppMetadata) error
	DeleteRecord(ctx context.Context, txid string, outputIndex int) error
	FindByDomain(ctx context.Context, domain string, limit, skip int, sortOrder string) ([]UTXOReference, error)
	FindByPublisher(ctx context.Context, publisher string, limit, skip int, sortOrder string) ([]UTXOReference, error)
	FindByOutpoint(ctx context.Context, outpoint string) ([]UTXOReference, error)
	FindByNameFuzzy(ctx context.Context, partialName string, limit, skip int, sortOrder string) ([]UTXOReference, error)
	FindByTags(ctx context.Context, tags []string, limit, skip int, sortOrder string) ([]UTXOReference, error)
	FindByCategory(ctx context.Context, category string, limit, skip int, sortOrder string) ([]UTXOReference, error)
	FindAllApps(ctx context.Context, limit, skip int, sortOrder string) ([]UTXOReference, error)
}

// AppsStorage handles Apps catalog record storage in MongoDB
type AppsStorage struct {
	collection *mongo.Collection
}

// Ensure AppsStorage implements AppsStorageEngine
var _ AppsStorageEngine = (*AppsStorage)(nil)

// NewAppsStorage creates a new Apps storage instance
func NewAppsStorage(db *mongo.Database) *AppsStorage {
	collection := db.Collection("appsCatalogRecords")

	// Create text index on searchable fields for full-text search
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "metadata.name", Value: "text"},
			{Key: "metadata.description", Value: "text"},
			{Key: "metadata.tags", Value: "text"},
			{Key: "metadata.domain", Value: "text"},
		},
	}
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		// Log error but don't fail - index might already exist
		fmt.Printf("Warning: failed to create text index on Apps catalog fields: %v\n", err)
	}

	return &AppsStorage{
		collection: collection,
	}
}

// StoreRecord inserts a new Apps catalog record
func (s *AppsStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, metadata *PublishedAppMetadata) error {
	record := &AppCatalogRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}

	_, err := s.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert Apps catalog record: %w", err)
	}
	return nil
}

// DeleteRecord deletes an Apps catalog record by txid and output index
func (s *AppsStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete Apps catalog record: %w", err)
	}
	return nil
}

// FindByDomain fetches records published for a specific domain with pagination and sorting
func (s *AppsStorage) FindByDomain(ctx context.Context, domain string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	query := bson.M{"metadata.domain": domain}
	return s.findRecordWithQuery(ctx, query, limit, skip, sortOrder)
}

// FindByPublisher fetches records by publisher (identity key) with pagination and sorting
func (s *AppsStorage) FindByPublisher(ctx context.Context, publisher string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	query := bson.M{"metadata.publisher": publisher}
	return s.findRecordWithQuery(ctx, query, limit, skip, sortOrder)
}

// FindByOutpoint looks up the single record behind an outpoint ("txid.outputIndex")
func (s *AppsStorage) FindByOutpoint(ctx context.Context, outpoint string) ([]UTXOReference, error) {
	parts := strings.Split(outpoint, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid outpoint format – expected \"txid.outputIndex\"")
	}

	txid := parts[0]
	var outputIndex int
	if _, err := fmt.Sscanf(parts[1], "%d", &outputIndex); err != nil {
		return nil, fmt.Errorf("invalid outpoint format – expected \"txid.outputIndex\"")
	}

	query := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}

	// For outpoint lookup, we don't need pagination or sorting as it's a unique identifier
	return s.findRecordWithQuery(ctx, query, 1, 0, "desc")
}

// FindByNameFuzzy fuzzy-matches apps by (partial) name
func (s *AppsStorage) FindByNameFuzzy(ctx context.Context, partialName string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	// Escape regex metacharacters so user input is safe
	escaped := regexp.QuoteMeta(partialName)
	pattern := bson.M{
		"$regex":   escaped,
		"$options": "i", // case-insensitive
	}

	query := bson.M{"metadata.name": pattern}
	return s.findRecordWithQuery(ctx, query, limit, skip, sortOrder)
}

// FindByTags fetches records that match the specified tags
func (s *AppsStorage) FindByTags(ctx context.Context, tags []string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	query := bson.M{
		"metadata.tags": bson.M{"$in": tags},
	}
	return s.findRecordWithQuery(ctx, query, limit, skip, sortOrder)
}

// FindByCategory fetches records that match the specified category
func (s *AppsStorage) FindByCategory(ctx context.Context, category string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	query := bson.M{"metadata.category": category}
	return s.findRecordWithQuery(ctx, query, limit, skip, sortOrder)
}

// FindAllApps fetches all app records without filtering, with pagination and sorting
func (s *AppsStorage) FindAllApps(ctx context.Context, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	return s.findRecordWithQuery(ctx, bson.M{}, limit, skip, sortOrder)
}

// findRecordWithQuery is a helper function for querying from the database
func (s *AppsStorage) findRecordWithQuery(ctx context.Context, filter interface{}, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	// Default pagination
	if limit <= 0 {
		limit = 50
	}
	if skip < 0 {
		skip = 0
	}

	// Apply sort on release_date for chronological ordering
	sortDirection := -1 // desc by default
	if sortOrder == "asc" {
		sortDirection = 1
	}

	opts := options.Find().
		SetSort(bson.M{"metadata.release_date": sortDirection}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query Apps catalog records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record AppCatalogRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode Apps catalog record: %w", err)
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
