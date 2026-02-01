package did

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

// DIDStorageEngine defines the interface for DID storage operations
type DIDStorageEngine interface {
	StoreRecord(txid string, outputIndex int, serialNumber string) error
	DeleteRecord(txid string, outputIndex int) error
	FindByCertificateSerialNumber(serialNumber string) ([]UTXOReference, error)
	FindByOutpoint(outpoint string) ([]UTXOReference, error)
}

// DIDStorage implements a storage engine for the DID lookup service
type DIDStorage struct {
	db      *mongo.Database
	records *mongo.Collection
}

// Ensure DIDStorage implements DIDStorageEngine
var _ DIDStorageEngine = (*DIDStorage)(nil)

// NewDIDStorage constructs a new DIDStorage instance
func NewDIDStorage(db *mongo.Database) *DIDStorage {
	storage := &DIDStorage{
		db:      db,
		records: db.Collection("didRecords"),
	}

	// Create text index on searchableAttributes
	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "searchableAttributes", Value: "text"}},
	}
	_, err := storage.records.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		// Log error but don't fail - index might already exist
		fmt.Printf("Warning: failed to create index: %v\n", err)
	}

	return storage
}

// StoreRecord stores a new DID record in the database
func (s *DIDStorage) StoreRecord(txid string, outputIndex int, serialNumber string) error {
	ctx := context.Background()
	record := DIDRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		SerialNumber: serialNumber,
		CreatedAt:    time.Now(),
	}
	_, err := s.records.InsertOne(ctx, record)
	return err
}

// DeleteRecord deletes a DID record by transaction ID and output index
func (s *DIDStorage) DeleteRecord(txid string, outputIndex int) error {
	ctx := context.Background()
	filter := bson.M{"txid": txid, "outputIndex": outputIndex}
	_, err := s.records.DeleteOne(ctx, filter)
	return err
}

// FindByCertificateSerialNumber finds DID records matching the given serial number
func (s *DIDStorage) FindByCertificateSerialNumber(serialNumber string) ([]UTXOReference, error) {
	return s.findRecordWithQuery(bson.M{"serialNumber": serialNumber})
}

// FindByOutpoint finds a DID record by outpoint (format: "txid.outputIndex")
func (s *DIDStorage) FindByOutpoint(outpoint string) ([]UTXOReference, error) {
	// Parse txid and outputIndex from the outpoint string (format: "txid.outputIndex")
	parts := strings.Split(outpoint, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid outpoint format, expected txid.outputIndex")
	}

	txid := parts[0]
	outputIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid output index in outpoint: %w", err)
	}

	return s.findRecordWithQuery(bson.M{"txid": txid, "outputIndex": outputIndex})
}

// findRecordWithQuery is a helper function for querying from the database
func (s *DIDStorage) findRecordWithQuery(query bson.M) ([]UTXOReference, error) {
	ctx := context.Background()

	// Only project txid and outputIndex
	projection := options.Find().SetProjection(bson.M{
		"txid":        1,
		"outputIndex": 1,
	})

	cursor, err := s.records.Find(ctx, query, projection)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []DIDRecord
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Convert to UTXO references
	utxoRefs := make([]UTXOReference, len(results))
	for i, record := range results {
		utxoRefs[i] = UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		}
	}

	return utxoRefs, nil
}
