package supplychain

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SupplyChainStorage handles SupplyChain record storage in MongoDB
type SupplyChainStorage struct {
	collection *mongo.Collection
}

// NewSupplyChainStorage creates a new SupplyChain storage instance
func NewSupplyChainStorage(db *mongo.Database) *SupplyChainStorage {
	collection := db.Collection("supplyChainRecords")

	// Create index on offChainValues.chainId for efficient lookups
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "offChainValues.chainId", Value: 1}},
		Options: options.Index().SetName("offChainValuesIndex"),
	}
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		// Log error but don't fail - index might already exist
		fmt.Printf("Warning: failed to create chainId index on SupplyChain records: %v\n", err)
	}

	return &SupplyChainStorage{
		collection: collection,
	}
}

// StoreRecord inserts a new SupplyChain record
func (s *SupplyChainStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, offChainValues map[string]interface{}) error {
	record := &SupplyChainRecord{
		Txid:           txid,
		OutputIndex:    outputIndex,
		OffChainValues: offChainValues,
		CreatedAt:      time.Now(),
	}

	_, err := s.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert SupplyChain record: %w", err)
	}
	return nil
}

// SpendRecord updates a SupplyChain record to mark it as spent
func (s *SupplyChainStorage) SpendRecord(ctx context.Context, txid string, outputIndex int, spendingTxid string) error {
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
		return fmt.Errorf("failed to update SupplyChain record: %w", err)
	}
	return nil
}

// DeleteRecord deletes a SupplyChain record by txid and output index
func (s *SupplyChainStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete SupplyChain record: %w", err)
	}
	return nil
}

// FindByChainID finds SupplyChain records by chainId
func (s *SupplyChainStorage) FindByChainID(ctx context.Context, chainID string, limit, skip int) ([]UTXOReference, error) {
	if chainID == "" {
		return []UTXOReference{}, nil
	}

	// Default limit
	if limit <= 0 {
		limit = 8
	}
	if skip < 0 {
		skip = 0
	}

	filter := bson.M{"offChainValues.chainId": chainID}
	opts := options.Find().
		SetSort(bson.M{"createdAt": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query SupplyChain records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record SupplyChainRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode SupplyChain record: %w", err)
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

// FindByTxid finds SupplyChain records by transaction ID
func (s *SupplyChainStorage) FindByTxid(ctx context.Context, txid string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	if txid == "" {
		return []UTXOReference{}, nil
	}

	// Default pagination
	if limit <= 0 {
		limit = 50
	}
	if skip < 0 {
		skip = 0
	}

	// Apply sort on createdAt
	sortDirection := -1 // desc by default
	if sortOrder == "asc" {
		sortDirection = 1
	}

	filter := bson.M{"txid": txid}
	opts := options.Find().
		SetSort(bson.M{"createdAt": sortDirection}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query SupplyChain records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record SupplyChainRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode SupplyChain record: %w", err)
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

// FindAll retrieves all SupplyChain records with optional date range filtering, pagination, and sorting
func (s *SupplyChainStorage) FindAll(ctx context.Context, limit, skip int, startDate, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
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
		return nil, fmt.Errorf("failed to query SupplyChain records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record SupplyChainRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode SupplyChain record: %w", err)
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
