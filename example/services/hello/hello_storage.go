package hello

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// HelloWorldStorage implements a storage engine for the HelloWorld lookup service
type HelloWorldStorage struct {
	db      *mongo.Database
	records *mongo.Collection
}

// NewHelloWorldStorage constructs a new HelloWorldStorage instance
func NewHelloWorldStorage(db *mongo.Database) *HelloWorldStorage {
	storage := &HelloWorldStorage{
		db:      db,
		records: db.Collection("helloWorldRecords"),
	}
	storage.createSearchableIndex()
	return storage
}

// createSearchableIndex ensures a text index exists for the message field
func (s *HelloWorldStorage) createSearchableIndex() {
	ctx := context.Background()
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "message", Value: "text"}},
		Options: options.Index().SetName("messageIndex"),
	}
	_, err := s.records.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		// Log but don't fail - index might already exist
		// In production, you might want to handle this more gracefully
	}
}

// StoreRecord stores a new HelloWorld message record in the database
func (s *HelloWorldStorage) StoreRecord(txid string, outputIndex int, message string) error {
	ctx := context.Background()
	record := HelloWorldRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		Message:     message,
		CreatedAt:   time.Now(),
	}
	_, err := s.records.InsertOne(ctx, record)
	return err
}

// DeleteRecord deletes a record matching the given transaction ID and output index
func (s *HelloWorldStorage) DeleteRecord(txid string, outputIndex int) error {
	ctx := context.Background()
	filter := bson.M{"txid": txid, "outputIndex": outputIndex}
	_, err := s.records.DeleteOne(ctx, filter)
	return err
}

// FindByMessage finds records containing the specified message text (case-insensitive)
func (s *HelloWorldStorage) FindByMessage(
	message string,
	limit int,
	skip int,
	sortOrder string,
) ([]UTXOReference, error) {
	if message == "" {
		return []UTXOReference{}, nil
	}

	if limit == 0 {
		limit = 50
	}

	ctx := context.Background()

	// Use text search for full-text matching
	filter := bson.M{"$text": bson.M{"$search": message}}

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

// FindAll retrieves all records, optionally filtered by date range and sorted by creation time
func (s *HelloWorldStorage) FindAll(
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
