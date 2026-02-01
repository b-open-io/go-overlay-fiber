package desktopintegrity

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DesktopIntegrityStorageEngine defines the interface for DesktopIntegrity storage operations
type DesktopIntegrityStorageEngine interface {
	StoreRecord(ctx context.Context, txid string, outputIndex int, fileHash string, offChainValues []byte) error
	DeleteRecord(ctx context.Context, txid string, outputIndex int) error
	FindByFileHash(ctx context.Context, fileHash string, limit int, skip int, sortOrder string) ([]UTXOReference, error)
	FindByTxid(ctx context.Context, txid string, limit int, skip int, sortOrder string) ([]UTXOReference, error)
	FindAll(ctx context.Context, limit int, skip int, startDate *time.Time, endDate *time.Time, sortOrder string) ([]UTXOReference, error)
}

// DesktopIntegrityStorage implements storage for desktop integrity records
type DesktopIntegrityStorage struct {
	collection *mongo.Collection
}

// Ensure DesktopIntegrityStorage implements DesktopIntegrityStorageEngine
var _ DesktopIntegrityStorageEngine = (*DesktopIntegrityStorage)(nil)

// NewDesktopIntegrityStorage creates a new DesktopIntegrityStorage instance
func NewDesktopIntegrityStorage(db *mongo.Database) *DesktopIntegrityStorage {
	storage := &DesktopIntegrityStorage{
		collection: db.Collection("desktopIntegrityRecords"),
	}
	storage.createSearchableIndex()
	return storage
}

// createSearchableIndex ensures a text index exists for the fileHash field
func (s *DesktopIntegrityStorage) createSearchableIndex() {
	ctx := context.Background()
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "fileHash", Value: 1}},
		Options: options.Index().SetName("fileHashIndex"),
	}
	_, _ = s.collection.Indexes().CreateOne(ctx, indexModel)
}

// StoreRecord stores a new desktop integrity record
func (s *DesktopIntegrityStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, fileHash string, offChainValues []byte) error {
	record := DesktopIntegrityRecord{
		Txid:           txid,
		OutputIndex:    outputIndex,
		FileHash:       fileHash,
		OffChainValues: offChainValues,
		CreatedAt:      time.Now(),
	}
	_, err := s.collection.InsertOne(ctx, record)
	return err
}

// DeleteRecord deletes a desktop integrity record
func (s *DesktopIntegrityStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	return err
}

// FindByFileHash finds records by file hash
func (s *DesktopIntegrityStorage) FindByFileHash(ctx context.Context, fileHash string, limit int, skip int, sortOrder string) ([]UTXOReference, error) {
	if fileHash == "" {
		return []UTXOReference{}, nil
	}

	direction := 1
	if sortOrder == "desc" {
		direction = -1
	}

	filter := bson.M{"fileHash": fileHash}
	opts := options.Find().
		SetProjection(bson.M{"txid": 1, "outputIndex": 1, "_id": 0}).
		SetSort(bson.D{{Key: "createdAt", Value: direction}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := s.collection.Find(ctx, filter, opts)
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

// FindByTxid finds records by transaction ID
func (s *DesktopIntegrityStorage) FindByTxid(ctx context.Context, txid string, limit int, skip int, sortOrder string) ([]UTXOReference, error) {
	if txid == "" {
		return []UTXOReference{}, nil
	}

	direction := 1
	if sortOrder == "desc" {
		direction = -1
	}

	filter := bson.M{"txid": txid}
	opts := options.Find().
		SetProjection(bson.M{"txid": 1, "outputIndex": 1, "_id": 0}).
		SetSort(bson.D{{Key: "createdAt", Value: direction}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := s.collection.Find(ctx, filter, opts)
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

// FindAll retrieves all records with optional date range filtering
func (s *DesktopIntegrityStorage) FindAll(ctx context.Context, limit int, skip int, startDate *time.Time, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
	filter := bson.M{}

	if startDate != nil || endDate != nil {
		createdAtFilter := bson.M{}
		if startDate != nil {
			createdAtFilter["$gte"] = *startDate
		}
		if endDate != nil {
			createdAtFilter["$lte"] = *endDate
		}
		filter["createdAt"] = createdAtFilter
	}

	direction := 1
	if sortOrder == "desc" {
		direction = -1
	}

	opts := options.Find().
		SetProjection(bson.M{"txid": 1, "outputIndex": 1, "_id": 0}).
		SetSort(bson.D{{Key: "createdAt", Value: direction}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := s.collection.Find(ctx, filter, opts)
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
