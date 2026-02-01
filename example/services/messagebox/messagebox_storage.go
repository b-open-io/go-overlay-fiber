package messagebox

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MessageBoxStorageEngine defines the interface for MessageBox storage operations
type MessageBoxStorageEngine interface {
	StoreRecord(identityKey string, host string, txid string, outputIndex int) error
	DeleteRecord(txid string, outputIndex int) error
	FindAdvertisements(identityKey string, host string) ([]UTXOReference, error)
	FindAll() ([]UTXOReference, error)
	FindRecent(limit int) ([]UTXOReference, error)
}

// MessageBoxStorage implements a storage engine for the MessageBox lookup service
type MessageBoxStorage struct {
	db            *mongo.Database
	adsCollection *mongo.Collection
}

// Ensure MessageBoxStorage implements MessageBoxStorageEngine
var _ MessageBoxStorageEngine = (*MessageBoxStorage)(nil)

// NewMessageBoxStorage constructs a new MessageBoxStorage instance
func NewMessageBoxStorage(db *mongo.Database) *MessageBoxStorage {
	return &MessageBoxStorage{
		db:            db,
		adsCollection: db.Collection("messagebox_advertisement"),
	}
}

// StoreRecord stores a new overlay advertisement record in the database
func (s *MessageBoxStorage) StoreRecord(identityKey string, host string, txid string, outputIndex int) error {
	ctx := context.Background()
	record := MessageBoxAdvertisement{
		IdentityKey: identityKey,
		Host:        host,
		Txid:        txid,
		OutputIndex: outputIndex,
		CreatedAt:   time.Now(),
	}
	_, err := s.adsCollection.InsertOne(ctx, record)
	return err
}

// DeleteRecord deletes an overlay advertisement by transaction ID and output index
func (s *MessageBoxStorage) DeleteRecord(txid string, outputIndex int) error {
	ctx := context.Background()
	filter := bson.M{"txid": txid, "outputIndex": outputIndex}
	_, err := s.adsCollection.DeleteOne(ctx, filter)
	return err
}

// FindAdvertisements finds all known lookup records for a given identity key, and optionally a host,
// ordered by recency (newest first).
func (s *MessageBoxStorage) FindAdvertisements(identityKey string, host string) ([]UTXOReference, error) {
	ctx := context.Background()

	filter := bson.M{"identityKey": identityKey}
	if host != "" {
		filter["host"] = host
	}

	findOptions := options.Find().
		SetSort(bson.M{"createdAt": -1}).
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.adsCollection.Find(ctx, filter, findOptions)
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

// FindAll lists all stored advertisements in the database, ordered by recency (newest first).
func (s *MessageBoxStorage) FindAll() ([]UTXOReference, error) {
	ctx := context.Background()

	findOptions := options.Find().
		SetSort(bson.M{"createdAt": -1}).
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.adsCollection.Find(ctx, bson.M{}, findOptions)
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

// FindRecent returns a limited number of the most recent overlay advertisements.
func (s *MessageBoxStorage) FindRecent(limit int) ([]UTXOReference, error) {
	if limit == 0 {
		limit = 10
	}

	ctx := context.Background()

	findOptions := options.Find().
		SetSort(bson.M{"createdAt": -1}).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.adsCollection.Find(ctx, bson.M{}, findOptions)
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
