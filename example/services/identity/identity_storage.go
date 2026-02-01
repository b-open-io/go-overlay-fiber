package identity

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IdentityStorageEngine defines the interface for Identity storage operations
type IdentityStorageEngine interface {
	StoreRecord(ctx context.Context, txid string, outputIndex int, certificate *certificates.Certificate) error
	DeleteRecord(ctx context.Context, txid string, outputIndex int) error
	FindByAttribute(ctx context.Context, attributes IdentityAttributes, certifiers []string) ([]UTXOReference, error)
	FindByIdentityKey(ctx context.Context, identityKey string, certifiers []string) ([]UTXOReference, error)
	FindByCertifier(ctx context.Context, certifiers []string) ([]UTXOReference, error)
	FindByCertificateType(ctx context.Context, certificateTypes []string, identityKey string, certifiers []string) ([]UTXOReference, error)
	FindByCertificateSerialNumber(ctx context.Context, serialNumber string) ([]UTXOReference, error)
}

// IdentityStorage handles Identity record storage in MongoDB
type IdentityStorage struct {
	collection *mongo.Collection
}

// Ensure IdentityStorage implements IdentityStorageEngine
var _ IdentityStorageEngine = (*IdentityStorage)(nil)

// NewIdentityStorage creates a new Identity storage instance
func NewIdentityStorage(db *mongo.Database) *IdentityStorage {
	collection := db.Collection("identityRecords")

	// Create text index on searchableAttributes for full-text search
	indexModel := mongo.IndexModel{
		Keys: bson.M{
			"searchableAttributes": "text",
		},
	}
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		// Log error but don't fail - index might already exist
		fmt.Printf("Warning: failed to create text index on searchableAttributes: %v\n", err)
	}

	return &IdentityStorage{
		collection: collection,
	}
}

// StoreRecord inserts a new Identity record
func (s *IdentityStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, certificate *certificates.Certificate) error {
	// Build searchable attributes string from certificate fields
	// Exclude binary/large fields like profilePhoto and icon
	var searchableAttrs []string
	for key, value := range certificate.Fields {
		keyStr := string(key)
		valueStr := string(value)
		if keyStr != "profilePhoto" && keyStr != "icon" {
			searchableAttrs = append(searchableAttrs, valueStr)
		}
	}

	record := &IdentityRecord{
		Txid:                 txid,
		OutputIndex:          outputIndex,
		Certificate:          certificate,
		CreatedAt:            time.Now(),
		SearchableAttributes: strings.Join(searchableAttrs, " "),
	}

	_, err := s.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert Identity record: %w", err)
	}
	return nil
}

// DeleteRecord deletes an Identity record by txid and output index
func (s *IdentityStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete Identity record: %w", err)
	}
	return nil
}

// FindByAttribute finds records by certified attributes with fuzzy matching
func (s *IdentityStorage) FindByAttribute(ctx context.Context, attributes IdentityAttributes, certifiers []string) ([]UTXOReference, error) {
	// Validate input
	if len(attributes) == 0 {
		return []UTXOReference{}, nil
	}

	// Build query
	query := bson.M{
		"$and": []bson.M{
			{"certificate.certifier": bson.M{"$in": certifiers}},
		},
	}

	// Handle "any" special case for full-text search across all attributes
	if anyValue, ok := attributes["any"]; ok {
		fuzzyPattern := getFuzzyRegex(anyValue)
		query["$and"] = append(query["$and"].([]bson.M), bson.M{
			"searchableAttributes": fuzzyPattern,
		})
	} else {
		// Build fuzzy regex queries for specific fields
		andQuery := query["$and"].([]bson.M)
		for key, value := range attributes {
			fuzzyPattern := getFuzzyRegex(value)
			fieldKey := fmt.Sprintf("certificate.fields.%s", key)
			andQuery = append(andQuery, bson.M{
				fieldKey: fuzzyPattern,
			})
		}
		query["$and"] = andQuery
	}

	return s.findRecordWithQuery(ctx, query)
}

// FindByIdentityKey finds records by identity key (subject)
func (s *IdentityStorage) FindByIdentityKey(ctx context.Context, identityKey string, certifiers []string) ([]UTXOReference, error) {
	if identityKey == "" {
		return []UTXOReference{}, nil
	}

	query := bson.M{
		"certificate.subject": identityKey,
	}

	// Add certifier filter if provided
	if len(certifiers) > 0 {
		query["certificate.certifier"] = bson.M{"$in": certifiers}
	}

	return s.findRecordWithQuery(ctx, query)
}

// FindByCertifier finds records by certifier identity keys
func (s *IdentityStorage) FindByCertifier(ctx context.Context, certifiers []string) ([]UTXOReference, error) {
	if len(certifiers) == 0 {
		return []UTXOReference{}, nil
	}

	query := bson.M{
		"certificate.certifier": bson.M{"$in": certifiers},
	}

	return s.findRecordWithQuery(ctx, query)
}

// FindByCertificateType finds records by certificate type
func (s *IdentityStorage) FindByCertificateType(ctx context.Context, certificateTypes []string, identityKey string, certifiers []string) ([]UTXOReference, error) {
	if len(certificateTypes) == 0 || identityKey == "" || len(certifiers) == 0 {
		return []UTXOReference{}, nil
	}

	query := bson.M{
		"certificate.subject":   identityKey,
		"certificate.certifier": bson.M{"$in": certifiers},
		"certificate.type":      bson.M{"$in": certificateTypes},
	}

	return s.findRecordWithQuery(ctx, query)
}

// FindByCertificateSerialNumber finds records by certificate serial number
func (s *IdentityStorage) FindByCertificateSerialNumber(ctx context.Context, serialNumber string) ([]UTXOReference, error) {
	if serialNumber == "" {
		return []UTXOReference{}, nil
	}

	query := bson.M{
		"certificate.serialNumber": serialNumber,
	}

	return s.findRecordWithQuery(ctx, query)
}

// getFuzzyRegex converts a string into a regex pattern for fuzzy search
// Matches the TypeScript implementation: input.split(”).join('.*')
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
func (s *IdentityStorage) findRecordWithQuery(ctx context.Context, filter interface{}) ([]UTXOReference, error) {
	opts := options.Find().SetProjection(bson.M{"txid": 1, "outputIndex": 1})
	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query Identity records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record IdentityRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode Identity record: %w", err)
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
