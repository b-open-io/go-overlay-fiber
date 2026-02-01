package slackthreads

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SlackThreadsStorageEngine defines the interface for SlackThreads storage operations
type SlackThreadsStorageEngine interface {
	StoreRecord(txid string, outputIndex int, threadHash string) error
	DeleteRecord(txid string, outputIndex int) error
	FindByThreadHash(threadHash string, limit int, skip int, sortOrder string) ([]UTXOReference, error)
	FindByTxid(txid string, limit int, skip int, sortOrder string) ([]UTXOReference, error)
	FindAll(limit int, skip int, startDate *time.Time, endDate *time.Time, sortOrder string) ([]UTXOReference, error)
}

// SlackThreadsStorage implements a storage engine for the SlackThread lookup service
type SlackThreadsStorage struct {
	db      *mongo.Database
	records *mongo.Collection
}

// Ensure SlackThreadsStorage implements SlackThreadsStorageEngine
var _ SlackThreadsStorageEngine = (*SlackThreadsStorage)(nil)

// NewSlackThreadsStorage constructs a new SlackThreadsStorage instance
func NewSlackThreadsStorage(db *mongo.Database) *SlackThreadsStorage {
	storage := &SlackThreadsStorage{
		db:      db,
		records: db.Collection("slackThreadRecords"),
	}
	storage.createSearchableIndex()
	return storage
}

// createSearchableIndex ensures an index exists for the threadHash field, enabling efficient searches.
// The index is named threadHashIndex.
func (s *SlackThreadsStorage) createSearchableIndex() {
	ctx := context.Background()
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "threadHash", Value: 1}},
		Options: options.Index().SetName("threadHashIndex"),
	}
	_, err := s.records.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		// Log but don't fail - index might already exist
		// In production, you might want to handle this more gracefully
	}
}

// StoreRecord stores a new SlackThread record in the database.
func (s *SlackThreadsStorage) StoreRecord(txid string, outputIndex int, threadHash string) error {
	ctx := context.Background()
	record := SlackThreadRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		ThreadHash:  threadHash,
		CreatedAt:   time.Now(),
	}
	_, err := s.records.InsertOne(ctx, record)
	return err
}

// DeleteRecord deletes a SlackThread record that matches the given transaction ID and output index.
func (s *SlackThreadsStorage) DeleteRecord(txid string, outputIndex int) error {
	ctx := context.Background()
	filter := bson.M{"txid": txid, "outputIndex": outputIndex}
	_, err := s.records.DeleteOne(ctx, filter)
	return err
}

// FindByThreadHash finds SlackThread records containing the specified thread hash (exact match).
func (s *SlackThreadsStorage) FindByThreadHash(
	threadHash string,
	limit int,
	skip int,
	sortOrder string,
) ([]UTXOReference, error) {
	if threadHash == "" {
		return []UTXOReference{}, nil
	}

	if limit == 0 {
		limit = 50
	}

	ctx := context.Background()
	filter := bson.M{"threadHash": threadHash}

	// Determine sort direction
	sortDirection := -1 // default descending (newest first)
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

// FindByTxid finds SlackThread records containing the specified transaction ID (exact match).
func (s *SlackThreadsStorage) FindByTxid(
	txid string,
	limit int,
	skip int,
	sortOrder string,
) ([]UTXOReference, error) {
	if txid == "" {
		return []UTXOReference{}, nil
	}

	if limit == 0 {
		limit = 50
	}

	ctx := context.Background()
	filter := bson.M{"txid": txid}

	// Determine sort direction
	sortDirection := -1 // default descending (newest first)
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

// FindAll retrieves all SlackThread records, optionally filtered by date range and sorted by creation time.
func (s *SlackThreadsStorage) FindAll(
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
	sortDirection := -1 // default descending (newest first)
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
