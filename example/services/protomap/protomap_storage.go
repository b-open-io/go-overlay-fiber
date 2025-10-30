package protomap

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ProtoMapStorage handles ProtoMap record storage in MongoDB
type ProtoMapStorage struct {
	collection *mongo.Collection
}

// NewProtoMapStorage creates a new ProtoMap storage instance
func NewProtoMapStorage(db *mongo.Database) *ProtoMapStorage {
	return &ProtoMapStorage{
		collection: db.Collection("protomapRecords"),
	}
}

// StoreRecord inserts a new ProtoMap record
func (s *ProtoMapStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration ProtoMapRegistration) error {
	record := &ProtoMapRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		Registration: registration,
		CreatedAt:    time.Now(),
	}

	_, err := s.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert ProtoMap record: %w", err)
	}
	return nil
}

// DeleteRecord deletes a ProtoMap record by txid and output index
func (s *ProtoMapStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	filter := bson.M{
		"txid":        txid,
		"outputIndex": outputIndex,
	}
	_, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete ProtoMap record: %w", err)
	}
	return nil
}

// FindByName finds protocol registrations by name
func (s *ProtoMapStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
	filter := bson.M{
		"registration.registryOperator": bson.M{"$in": registryOperators},
		"registration.name":             name,
	}

	return s.findRecordWithQuery(ctx, filter)
}

// FindByProtocolID finds protocols by protocolID
func (s *ProtoMapStorage) FindByProtocolID(ctx context.Context, protocolID ProtocolID, registryOperators []string) ([]UTXOReference, error) {
	filter := bson.M{
		"registration.protocolID.securityLevel": protocolID.SecurityLevel,
		"registration.protocolID.protocol":      protocolID.Protocol,
		"registration.registryOperator":         bson.M{"$in": registryOperators},
	}

	return s.findRecordWithQuery(ctx, filter)
}

// findRecordWithQuery is a helper function for querying from the database
func (s *ProtoMapStorage) findRecordWithQuery(ctx context.Context, filter bson.M) ([]UTXOReference, error) {
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query ProtoMap records: %w", err)
	}
	defer cursor.Close(ctx)

	var results []UTXOReference
	for cursor.Next(ctx) {
		var record ProtoMapRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("failed to decode ProtoMap record: %w", err)
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
