package ump

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTableDrivenQueryValidation tests various query scenarios using table-driven tests
func TestTableDrivenQueryValidation(t *testing.T) {
	storage := NewMockUMPStorage()
	service := NewUMPLookupServiceWithStorage(storage)

	// Store some test data
	testRecord := &UMPRecord{
		Txid:             "txid_test",
		OutputIndex:      0,
		PresentationHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		RecoveryHash:     "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
	}
	err := storage.InsertRecord(context.Background(), testRecord)
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty query",
			query:       map[string]interface{}{},
			expectError: true,
			errorMsg:    "query parameters",
		},
		{
			name:        "valid presentationHash query",
			query:       map[string]interface{}{"presentationHash": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			expectError: false,
		},
		{
			name:        "valid recoveryHash query",
			query:       map[string]interface{}{"recoveryHash": "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"},
			expectError: false,
		},
		{
			name:        "valid outpoint query",
			query:       map[string]interface{}{"outpoint": "txid_test.0"},
			expectError: false,
		},
		{
			name:        "non-existent presentationHash",
			query:       map[string]interface{}{"presentationHash": "nonexistent1234567890abcdef1234567890abcdef1234567890abcdef1234"},
			expectError: false,
		},
		{
			name:        "non-existent recoveryHash",
			query:       map[string]interface{}{"recoveryHash": "nonexistent0987654321fedcba0987654321fedcba0987654321fedcba0987654321"},
			expectError: false,
		},
		{
			name:        "non-existent outpoint",
			query:       map[string]interface{}{"outpoint": "nonexistent.99"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_ump",
				Query:   queryJSON,
			}

			answer, err := service.Lookup(context.Background(), question)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, answer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, answer)
			}
		})
	}
}

// TestTableDrivenStorageOperations tests storage operations with table-driven tests
func TestTableDrivenStorageOperations(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(storage *MockUMPStorage)
		query     func(storage *MockUMPStorage) (*UMPRecord, error)
		wantFound bool
		wantError bool
	}{
		{
			name: "find by presentationHash - single match",
			setup: func(storage *MockUMPStorage) {
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid1",
					OutputIndex:      0,
					PresentationHash: "hash1",
					RecoveryHash:     "recovery1",
				})
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid2",
					OutputIndex:      0,
					PresentationHash: "hash2",
					RecoveryHash:     "recovery2",
				})
			},
			query: func(storage *MockUMPStorage) (*UMPRecord, error) {
				return storage.FindByPresentationHash(context.Background(), "hash1")
			},
			wantFound: true,
		},
		{
			name: "find by recoveryHash - single match",
			setup: func(storage *MockUMPStorage) {
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid1",
					OutputIndex:      0,
					PresentationHash: "hash1",
					RecoveryHash:     "recovery1",
				})
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid2",
					OutputIndex:      0,
					PresentationHash: "hash2",
					RecoveryHash:     "recovery2",
				})
			},
			query: func(storage *MockUMPStorage) (*UMPRecord, error) {
				return storage.FindByRecoveryHash(context.Background(), "recovery2")
			},
			wantFound: true,
		},
		{
			name: "find by outpoint - exact match",
			setup: func(storage *MockUMPStorage) {
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid1",
					OutputIndex:      0,
					PresentationHash: "hash1",
					RecoveryHash:     "recovery1",
				})
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid1",
					OutputIndex:      1,
					PresentationHash: "hash2",
					RecoveryHash:     "recovery2",
				})
			},
			query: func(storage *MockUMPStorage) (*UMPRecord, error) {
				return storage.FindByOutpoint(context.Background(), "txid1.0")
			},
			wantFound: true,
		},
		{
			name: "find by presentationHash - no match",
			setup: func(storage *MockUMPStorage) {
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid1",
					OutputIndex:      0,
					PresentationHash: "hash1",
					RecoveryHash:     "recovery1",
				})
			},
			query: func(storage *MockUMPStorage) (*UMPRecord, error) {
				return storage.FindByPresentationHash(context.Background(), "nonexistent")
			},
			wantFound: false,
		},
		{
			name: "find by recoveryHash - no match",
			setup: func(storage *MockUMPStorage) {
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid1",
					OutputIndex:      0,
					PresentationHash: "hash1",
					RecoveryHash:     "recovery1",
				})
			},
			query: func(storage *MockUMPStorage) (*UMPRecord, error) {
				return storage.FindByRecoveryHash(context.Background(), "nonexistent")
			},
			wantFound: false,
		},
		{
			name: "find by outpoint - no match",
			setup: func(storage *MockUMPStorage) {
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid1",
					OutputIndex:      0,
					PresentationHash: "hash1",
					RecoveryHash:     "recovery1",
				})
			},
			query: func(storage *MockUMPStorage) (*UMPRecord, error) {
				return storage.FindByOutpoint(context.Background(), "txid99.99")
			},
			wantFound: false,
		},
		{
			name: "delete record",
			setup: func(storage *MockUMPStorage) {
				_ = storage.InsertRecord(context.Background(), &UMPRecord{
					Txid:             "txid1",
					OutputIndex:      0,
					PresentationHash: "hash1",
					RecoveryHash:     "recovery1",
				})
				_ = storage.DeleteRecord(context.Background(), "txid1", 0)
			},
			query: func(storage *MockUMPStorage) (*UMPRecord, error) {
				return storage.FindByOutpoint(context.Background(), "txid1.0")
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockUMPStorage()
			tt.setup(storage)

			result, err := tt.query(storage)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.wantFound {
					assert.NotNil(t, result)
				} else {
					assert.Nil(t, result)
				}
			}
		})
	}
}
