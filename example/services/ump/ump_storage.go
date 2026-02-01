package ump

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UMPStorageEngine defines the interface for UMP storage operations
type UMPStorageEngine interface {
	InsertRecord(ctx context.Context, record *UMPRecord) error
	DeleteRecord(ctx context.Context, txid string, outputIndex int) error
	FindByPresentationHash(ctx context.Context, presentationHash string) (*UMPRecord, error)
	FindByRecoveryHash(ctx context.Context, recoveryHash string) (*UMPRecord, error)
	FindByOutpoint(ctx context.Context, outpoint string) (*UMPRecord, error)
}

// UMPStorage handles UMP record storage in MongoDB
type UMPStorage struct {
	collection *mongo.Collection
}

// Ensure UMPStorage implements UMPStorageEngine
var _ UMPStorageEngine = (*UMPStorage)(nil)

// NewUMPStorage creates a new UMP storage instance
func NewUMPStorage(db *mongo.Database) *UMPStorage {
	return &UMPStorage{
		collection: db.Collection("ump"),
	}
}

// InsertRecord inserts a new UMP record
func (s *UMPStorage) InsertRecord(ctx context.Context, record *UMPRecord) error {
	record.CreatedAt = time.Now()
	_, err := s.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert UMP record: %w", err)
	}
	return nil
}

// DeleteRecord deletes a UMP record by txid and output index
func (s *UMPStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete UMP record: %w", err)
	}
	return nil
}

// FindByPresentationHash finds a UMP record by presentation hash
func (s *UMPStorage) FindByPresentationHash(ctx context.Context, presentationHash string) (*UMPRecord, error) {
	filter := bson.M{"presentationHash": presentationHash}
	opts := options.FindOne().SetSort(bson.M{"_id": -1}) // Get newest

	var record UMPRecord
	err := s.collection.FindOne(ctx, filter, opts).Decode(&record)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find UMP record by presentationHash: %w", err)
	}
	return &record, nil
}

// FindByRecoveryHash finds a UMP record by recovery hash
func (s *UMPStorage) FindByRecoveryHash(ctx context.Context, recoveryHash string) (*UMPRecord, error) {
	filter := bson.M{"recoveryHash": recoveryHash}
	opts := options.FindOne().SetSort(bson.M{"_id": -1}) // Get newest

	var record UMPRecord
	err := s.collection.FindOne(ctx, filter, opts).Decode(&record)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find UMP record by recoveryHash: %w", err)
	}
	return &record, nil
}

// FindByOutpoint finds a UMP record by outpoint (txid.outputIndex)
func (s *UMPStorage) FindByOutpoint(ctx context.Context, outpoint string) (*UMPRecord, error) {
	// Parse outpoint string "txid.outputIndex"
	parts := strings.Split(outpoint, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid outpoint format, expected txid.outputIndex")
	}

	outputIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid output index in outpoint: %w", err)
	}

	filter := bson.M{
		"txid":        parts[0],
		"outputIndex": outputIndex,
	}

	var record UMPRecord
	err = s.collection.FindOne(ctx, filter).Decode(&record)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find UMP record by outpoint: %w", err)
	}
	return &record, nil
}
