package any

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AnyStorage implements a storage engine for the Any lookup service
type AnyStorage struct {
	db      *mongo.Database
	records *mongo.Collection
}

// NewAnyStorage constructs a new AnyStorage instance
func NewAnyStorage(db *mongo.Database) *AnyStorage {
	storage := &AnyStorage{
		db:      db,
		records: db.Collection("anyRecords"),
	}
	storage.createSearchableIndex()
	return storage
}

// createSearchableIndex ensures a text index exists for the txid field
func (s *AnyStorage) createSearchableIndex() {
	ctx := context.Background()
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "txid", Value: 1}},
		Options: options.Index().SetName("txidIndex"),
	}
	_, err := s.records.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		// Log but don't fail - index might already exist
		// In production, you might want to handle this more gracefully
	}
}

// StoreRecord stores a new Any record in the database
func (s *AnyStorage) StoreRecord(txid string, outputIndex int) error {
	ctx := context.Background()
	record := AnyRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		CreatedAt:   time.Now(),
	}
	_, err := s.records.InsertOne(ctx, record)
	return err
}

// SpendRecord updates a record to mark it as spent
func (s *AnyStorage) SpendRecord(txid string, outputIndex int, spendingTxid string) error {
	ctx := context.Background()
	filter := bson.M{"txid": txid, "outputIndex": outputIndex}
	update := bson.M{"$set": bson.M{"spendingTxid": spendingTxid}}
	_, err := s.records.UpdateOne(ctx, filter, update)
	return err
}

// DeleteRecord deletes a record matching the given transaction ID and output index
func (s *AnyStorage) DeleteRecord(txid string, outputIndex int) error {
	ctx := context.Background()
	filter := bson.M{"txid": txid, "outputIndex": outputIndex}
	_, err := s.records.DeleteOne(ctx, filter)
	return err
}

// FindByTxid finds a record by transaction ID
func (s *AnyStorage) FindByTxid(txid string) (*UTXOReference, error) {
	if txid == "" {
		return nil, nil
	}

	ctx := context.Background()
	filter := bson.M{"txid": txid}
	projection := bson.M{"txid": 1, "outputIndex": 1}

	var result UTXOReference
	err := s.records.FindOne(ctx, filter, options.FindOne().SetProjection(projection)).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// FindAll retrieves all records, optionally filtered by date range and sorted by creation time
func (s *AnyStorage) FindAll(
	limit int,
	skip int,
	startDate *time.Time,
	endDate *time.Time,
	sortOrder string,
) ([]UTXOReference, error) {
	if limit == 0 {
		limit = 50
	}

	ctx := context.Background()
	filter := bson.M{}

	// Add date range filter if provided
	if startDate != nil || endDate != nil {
		dateFilter := bson.M{}
		if startDate != nil {
			dateFilter["$gte"] = startDate
		}
		if endDate != nil {
			dateFilter["$lte"] = endDate
		}
		filter["createdAt"] = dateFilter
	}

	// Determine sort direction
	sortDirection := -1 // default descending
	if sortOrder == "asc" {
		sortDirection = 1
	}

	// Query options
	findOptions := options.Find().
		SetSort(bson.M{"createdAt": sortDirection}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.records.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}
