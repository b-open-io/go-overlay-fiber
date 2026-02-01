package uhrp

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// UHRPStorageEngine defines the interface for UHRP storage operations
type UHRPStorageEngine interface {
	StoreRecord(uhrpUrl string, txid string, outputIndex int, hostIdentityKey string, hostedFileLocation string, expiryTime uint64, fileSize uint64) error
	DeleteRecord(txid string, outputIndex int) error
	Lookup(query *UHRPQuery) ([]UTXOReference, error)
}

// UHRPStorage implements a storage engine for the UHRP lookup service
type UHRPStorage struct {
	db      *mongo.Database
	records *mongo.Collection
}

// Ensure UHRPStorage implements UHRPStorageEngine
var _ UHRPStorageEngine = (*UHRPStorage)(nil)

// NewUHRPStorage constructs a new UHRPStorage instance
func NewUHRPStorage(db *mongo.Database) *UHRPStorage {
	return &UHRPStorage{
		db:      db,
		records: db.Collection("uhrp"),
	}
}

// StoreRecord stores a new UHRP advertisement record in the database
func (s *UHRPStorage) StoreRecord(
	uhrpUrl string,
	txid string,
	outputIndex int,
	hostIdentityKey string,
	hostedFileLocation string,
	expiryTime uint64,
	fileSize uint64,
) error {
	ctx := context.Background()
	record := UHRPRecord{
		UHRPUrl:            uhrpUrl,
		Txid:               txid,
		OutputIndex:        outputIndex,
		HostIdentityKey:    hostIdentityKey,
		HostedFileLocation: hostedFileLocation,
		ExpiryTime:         expiryTime,
		FileSize:           fileSize,
	}
	_, err := s.records.InsertOne(ctx, record)
	return err
}

// DeleteRecord deletes a UHRP advertisement by transaction ID and output index
func (s *UHRPStorage) DeleteRecord(txid string, outputIndex int) error {
	ctx := context.Background()
	filter := bson.M{"txid": txid, "outputIndex": outputIndex}
	_, err := s.records.DeleteOne(ctx, filter)
	return err
}

// Lookup finds UHRP records matching the given query
func (s *UHRPStorage) Lookup(query *UHRPQuery) ([]UTXOReference, error) {
	ctx := context.Background()

	// Handle outpoint query (exact match by txid.outputIndex)
	if query.Outpoint != "" {
		parts := strings.Split(query.Outpoint, ".")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid outpoint format, expected txid.outputIndex")
		}
		txid := parts[0]
		outputIndex, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid output index in outpoint: %w", err)
		}

		var result UHRPRecord
		err = s.records.FindOne(ctx, bson.M{"txid": txid, "outputIndex": outputIndex}).Decode(&result)
		if err == mongo.ErrNoDocuments {
			return []UTXOReference{}, nil
		}
		if err != nil {
			return nil, err
		}

		return []UTXOReference{{Txid: result.Txid, OutputIndex: result.OutputIndex}}, nil
	}

	// Build filter for other query parameters
	filter := bson.M{}
	if query.UHRPUrl != "" {
		filter["uhrpUrl"] = query.UHRPUrl
	}
	if query.ExpiryTime != 0 {
		filter["expiryTime"] = query.ExpiryTime
	}
	if query.HostIdentityKey != "" {
		filter["hostIdentityKey"] = query.HostIdentityKey
	}
	if query.FileSize != 0 {
		filter["fileSize"] = query.FileSize
	}

	// Must have at least one filter criterion
	if len(filter) == 0 {
		return nil, fmt.Errorf("lookup must specify either outpoint, or at least one of (uhrpUrl, expiryTime, hostIdentityKey, fileSize)")
	}

	// Execute query
	cursor, err := s.records.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []UHRPRecord
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
