package certmap

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CertMapStorageEngine defines the interface for CertMap storage operations
type CertMapStorageEngine interface {
	StoreRecord(ctx context.Context, txid string, outputIndex int, registration *CertMapRegistration) error
	DeleteRecord(ctx context.Context, txid string, outputIndex int) error
	FindByType(ctx context.Context, certType string, registryOperators []string) ([]UTXOReference, error)
	FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error)
}

// CertMapStorage handles CertMap record storage in MongoDB
type CertMapStorage struct {
	collection *mongo.Collection
}

// Ensure CertMapStorage implements CertMapStorageEngine
var _ CertMapStorageEngine = (*CertMapStorage)(nil)

// NewCertMapStorage creates a new CertMap storage instance
func NewCertMapStorage(db *mongo.Database) *CertMapStorage {
	collection := db.Collection("certmapRecords")

	return &CertMapStorage{
		collection: collection,
	}
}

// StoreRecord inserts a new CertMap record
func (s *CertMapStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration *CertMapRegistration) error {
	record := &CertMapRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		Registration: registration,
		CreatedAt:    time.Now(),
	}

	_, err := s.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert CertMap record: %w", err)
	}
	return nil
}

// DeleteRecord deletes a CertMap record by txid and output index
func (s *CertMapStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete CertMap record: %w", err)
	}
	return nil
}

// FindByType fetches records by certificate type and registry operators
func (s *CertMapStorage) FindByType(ctx context.Context, certType string, registryOperators []string) ([]UTXOReference, error) {
	query := bson.M{
		"registration.type":             certType,
		"registration.registryOperator": bson.M{"$in": registryOperators},
	}
	return s.findRecordWithQuery(ctx, query)
}

// FindByName fetches records by certificate name (fuzzy search) and registry operators
func (s *CertMapStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
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
		"$and": []bson.M{
			{
				"registration.registryOperator": bson.M{"$in": registryOperators},
			},
			{
				"registration.name": bson.M{
					"$regex":   fuzzyPattern,
					"$options": "i", // case-insensitive
				},
			},
		},
	}

	return s.findRecordWithQuery(ctx, query)
}

// findRecordWithQuery is a helper function for querying from the database
func (s *CertMapStorage) findRecordWithQuery(ctx context.Context, filter interface{}) ([]UTXOReference, error) {
	opts := options.Find().
		SetProjection(bson.M{"txid": 1, "outputIndex": 1})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query CertMap records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record CertMapRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode CertMap record: %w", err)
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
