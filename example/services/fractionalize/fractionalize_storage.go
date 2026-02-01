package fractionalize

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// FractionalizeStorageEngine defines the interface for Fractionalize storage operations
type FractionalizeStorageEngine interface {
	StoreRecord(ctx context.Context, txid string, outputIndex int) error
	SpendRecord(ctx context.Context, txid string, outputIndex int, spendingTxid string) error
	DeleteRecord(ctx context.Context, txid string, outputIndex int) error
	FindByTxid(ctx context.Context, txid string) (*UTXOReference, error)
	FindAll(ctx context.Context, limit, skip int, startDate, endDate *time.Time, sortOrder string) ([]UTXOReference, error)
}

// FractionalizeStorage handles Fractionalize record storage in MongoDB
type FractionalizeStorage struct {
	collection *mongo.Collection
}

// Ensure FractionalizeStorage implements FractionalizeStorageEngine
var _ FractionalizeStorageEngine = (*FractionalizeStorage)(nil)

// NewFractionalizeStorage creates a new Fractionalize storage instance
func NewFractionalizeStorage(db *mongo.Database) *FractionalizeStorage {
	collection := db.Collection("fractionalizeRecords")

	// Create index on txid for efficient lookups
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "txid", Value: 1}},
		Options: options.Index().SetName("txidIndex"),
	}
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		// Log error but don't fail - index might already exist
		fmt.Printf("Warning: failed to create txid index on Fractionalize records: %v\n", err)
	}

	return &FractionalizeStorage{
		collection: collection,
	}
}

// StoreRecord inserts a new Fractionalize record
func (s *FractionalizeStorage) StoreRecord(ctx context.Context, txid string, outputIndex int) error {
	record := &FractionalizeRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		CreatedAt:   time.Now(),
	}

	_, err := s.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert Fractionalize record: %w", err)
	}
	return nil
}

// SpendRecord updates a Fractionalize record to mark it as spent
func (s *FractionalizeStorage) SpendRecord(ctx context.Context, txid string, outputIndex int, spendingTxid string) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	update := bson.M{
		"$set": bson.M{
			"spendingTxid": spendingTxid,
		},
	}
	_, err := s.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update Fractionalize record: %w", err)
	}
	return nil
}

// DeleteRecord deletes a Fractionalize record by txid and output index
func (s *FractionalizeStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete Fractionalize record: %w", err)
	}
	return nil
}

// FindByTxid finds a Fractionalize record by exact txid
func (s *FractionalizeStorage) FindByTxid(ctx context.Context, txid string) (*UTXOReference, error) {
	if txid == "" {
		return nil, nil
	}

	filter := bson.M{"txid": txid}
	opts := options.FindOne().SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	var record FractionalizeRecord
	err := s.collection.FindOne(ctx, filter, opts).Decode(&record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find Fractionalize record: %w", err)
	}

	return &UTXOReference{
		Txid:        record.Txid,
		OutputIndex: record.OutputIndex,
	}, nil
}

// FindAll retrieves all Fractionalize records with optional date range filtering, pagination, and sorting
func (s *FractionalizeStorage) FindAll(ctx context.Context, limit, skip int, startDate, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
	// Default pagination
	if limit <= 0 {
		limit = 50
	}
	if skip < 0 {
		skip = 0
	}

	// Build query filter
	filter := bson.M{}
	if startDate != nil || endDate != nil {
		dateFilter := bson.M{}
		if startDate != nil {
			dateFilter["$gte"] = *startDate
		}
		if endDate != nil {
			dateFilter["$lte"] = *endDate
		}
		filter["createdAt"] = dateFilter
	}

	// Apply sort on createdAt
	sortDirection := -1 // desc by default
	if sortOrder == "asc" {
		sortDirection = 1
	}

	opts := options.Find().
		SetSort(bson.M{"createdAt": sortDirection}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query Fractionalize records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record FractionalizeRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode Fractionalize record: %w", err)
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
