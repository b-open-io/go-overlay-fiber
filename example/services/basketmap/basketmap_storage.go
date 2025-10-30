package basketmap

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// BasketMapStorage handles BasketMap record storage in MongoDB
type BasketMapStorage struct {
	collection *mongo.Collection
}

// NewBasketMapStorage creates a new BasketMap storage instance
func NewBasketMapStorage(db *mongo.Database) *BasketMapStorage {
	return &BasketMapStorage{
		collection: db.Collection("basketmapRecords"),
	}
}

// StoreRecord inserts a new BasketMap record
func (s *BasketMapStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration BasketMapRegistration) error {
	record := &BasketMapRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		Registration: registration,
		CreatedAt:    time.Now(),
	}

	_, err := s.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert BasketMap record: %w", err)
	}
	return nil
}

// DeleteRecord deletes a BasketMap record by txid and output index
func (s *BasketMapStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete BasketMap record: %w", err)
	}
	return nil
}

// FindByID finds basket registrations by basketID
func (s *BasketMapStorage) FindByID(ctx context.Context, basketID string, registryOperators []string) ([]UTXOReference, error) {
	filter := bson.M{
		"registration.basketID":         basketID,
		"registration.registryOperator": bson.M{"$in": registryOperators},
	}

	return s.findRecordWithQuery(ctx, filter)
}

// FindByName finds baskets by name with fuzzy matching
func (s *BasketMapStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
	// Create fuzzy regex pattern (similar to TypeScript implementation)
	fuzzyPattern := getFuzzyRegex(name)

	filter := bson.M{
		"$and": []bson.M{
			{
				"registration.name":             fuzzyPattern,
				"registration.registryOperator": bson.M{"$in": registryOperators},
			},
		},
	}

	return s.findRecordWithQuery(ctx, filter)
}

// getFuzzyRegex converts a string into a regex pattern for fuzzy search
// Matches the TypeScript implementation: input.split('').join('.*')
func getFuzzyRegex(input string) bson.M {
	// Escape special regex characters
	escaped := regexp.QuoteMeta(input)

	// Convert "abc" to "a.*b.*c" for fuzzy matching
	pattern := ""
	for i, char := range escaped {
		if i > 0 {
			pattern += ".*"
		}
		pattern += string(char)
	}

	return bson.M{
		"$regex":   pattern,
		"$options": "i", // case-insensitive
	}
}

// findRecordWithQuery is a helper function for querying from the database
func (s *BasketMapStorage) findRecordWithQuery(ctx context.Context, filter bson.M) ([]UTXOReference, error) {
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query BasketMap records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record BasketMapRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode BasketMap record: %w", err)
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
