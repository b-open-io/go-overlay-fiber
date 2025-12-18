package did

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
	storage := NewMockDIDStorage()
	service := NewDIDLookupServiceWithStorage(storage)

	// Store some test data
	err := storage.StoreRecord("txid_test", 0, "serial123")
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
			errorMsg:    "no valid query parameters",
		},
		{
			name:        "valid serialNumber query",
			query:       map[string]interface{}{"serialNumber": "serial123"},
			expectError: false,
		},
		{
			name:        "valid outpoint query",
			query:       map[string]interface{}{"outpoint": "txid_test.0"},
			expectError: false,
		},
		{
			name:        "invalid outpoint format - no dot",
			query:       map[string]interface{}{"outpoint": "txid_no_dot"},
			expectError: true,
			errorMsg:    "invalid outpoint",
		},
		{
			name:        "invalid outpoint format - non-numeric index",
			query:       map[string]interface{}{"outpoint": "txid.abc"},
			expectError: true,
			errorMsg:    "invalid output index",
		},
		{
			name:        "invalid outpoint format - empty txid",
			query:       map[string]interface{}{"outpoint": ".0"},
			expectError: true,
			errorMsg:    "empty txid",
		},
		{
			name:        "invalid outpoint format - negative index",
			query:       map[string]interface{}{"outpoint": "txid.-1"},
			expectError: true,
			errorMsg:    "negative output index",
		},
		{
			name:        "non-existent record",
			query:       map[string]interface{}{"serialNumber": "nonexistent"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_did",
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
		setup     func(storage *MockDIDStorage)
		lookup    func(storage *MockDIDStorage) ([]UTXOReference, error)
		wantCount int
		wantError bool
	}{
		{
			name: "find by serialNumber - single match",
			setup: func(storage *MockDIDStorage) {
				_ = storage.StoreRecord("txid1", 0, "serial1")
				_ = storage.StoreRecord("txid2", 0, "serial2")
			},
			lookup: func(storage *MockDIDStorage) ([]UTXOReference, error) {
				return storage.FindByCertificateSerialNumber("serial1")
			},
			wantCount: 1,
		},
		{
			name: "find by serialNumber - multiple matches",
			setup: func(storage *MockDIDStorage) {
				_ = storage.StoreRecord("txid1", 0, "same_serial")
				_ = storage.StoreRecord("txid2", 0, "same_serial")
				_ = storage.StoreRecord("txid3", 0, "diff_serial")
			},
			lookup: func(storage *MockDIDStorage) ([]UTXOReference, error) {
				return storage.FindByCertificateSerialNumber("same_serial")
			},
			wantCount: 2,
		},
		{
			name: "find by outpoint - exact match",
			setup: func(storage *MockDIDStorage) {
				_ = storage.StoreRecord("txid1", 0, "serial1")
				_ = storage.StoreRecord("txid1", 1, "serial2")
			},
			lookup: func(storage *MockDIDStorage) ([]UTXOReference, error) {
				return storage.FindByOutpoint("txid1.0")
			},
			wantCount: 1,
		},
		{
			name: "find by outpoint - no match",
			setup: func(storage *MockDIDStorage) {
				_ = storage.StoreRecord("txid1", 0, "serial1")
			},
			lookup: func(storage *MockDIDStorage) ([]UTXOReference, error) {
				return storage.FindByOutpoint("txid2.0")
			},
			wantCount: 0,
		},
		{
			name: "find by serialNumber - no matches",
			setup: func(storage *MockDIDStorage) {
				_ = storage.StoreRecord("txid1", 0, "serial1")
			},
			lookup: func(storage *MockDIDStorage) ([]UTXOReference, error) {
				return storage.FindByCertificateSerialNumber("nonexistent")
			},
			wantCount: 0,
		},
		{
			name:  "invalid outpoint format",
			setup: func(storage *MockDIDStorage) {},
			lookup: func(storage *MockDIDStorage) ([]UTXOReference, error) {
				return storage.FindByOutpoint("invalid_format")
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockDIDStorage()
			tt.setup(storage)

			results, err := tt.lookup(storage)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tt.wantCount)
			}
		})
	}
}
