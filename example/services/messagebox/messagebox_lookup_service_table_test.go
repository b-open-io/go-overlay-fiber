package messagebox

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
	storage := NewMockMessageBoxStorage()
	service := NewMessageBoxLookupServiceWithStorage(storage)

	// Store some test data
	testIdentityKey := "02abc123def456abc123def456abc123def456abc123def456abc123def456abc123"
	err := storage.StoreRecord(testIdentityKey, "https://example.com", "txid_test", 0)
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
			errorMsg:    "identityKey",
		},
		{
			name:        "valid identityKey query",
			query:       map[string]interface{}{"identityKey": testIdentityKey},
			expectError: false,
		},
		{
			name: "valid identityKey and host query",
			query: map[string]interface{}{
				"identityKey": testIdentityKey,
				"host":        "https://example.com",
			},
			expectError: false,
		},
		{
			name:        "missing identityKey",
			query:       map[string]interface{}{"host": "https://example.com"},
			expectError: true,
			errorMsg:    "identityKey",
		},
		{
			name:        "empty identityKey",
			query:       map[string]interface{}{"identityKey": ""},
			expectError: true,
			errorMsg:    "identityKey",
		},
		{
			name:        "non-existent identityKey",
			query:       map[string]interface{}{"identityKey": "02nonexistent"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_messagebox",
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
		setup     func(storage *MockMessageBoxStorage)
		query     func(storage *MockMessageBoxStorage) ([]UTXOReference, error)
		wantCount int
		wantError bool
	}{
		{
			name: "find by identityKey - single match",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("key1", "host1", "txid1", 0)
				_ = storage.StoreRecord("key2", "host2", "txid2", 0)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindAdvertisements("key1", "")
			},
			wantCount: 1,
		},
		{
			name: "find by identityKey - multiple matches",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("same_key", "host1", "txid1", 0)
				_ = storage.StoreRecord("same_key", "host2", "txid2", 0)
				_ = storage.StoreRecord("diff_key", "host3", "txid3", 0)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindAdvertisements("same_key", "")
			},
			wantCount: 2,
		},
		{
			name: "find by identityKey and host - exact match",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("key1", "host1", "txid1", 0)
				_ = storage.StoreRecord("key1", "host2", "txid2", 0)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindAdvertisements("key1", "host1")
			},
			wantCount: 1,
		},
		{
			name: "find all advertisements",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("key1", "host1", "txid1", 0)
				_ = storage.StoreRecord("key2", "host2", "txid2", 0)
				_ = storage.StoreRecord("key3", "host3", "txid3", 0)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindAll()
			},
			wantCount: 3,
		},
		{
			name: "find recent with limit",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("key1", "host1", "txid1", 0)
				_ = storage.StoreRecord("key2", "host2", "txid2", 0)
				_ = storage.StoreRecord("key3", "host3", "txid3", 0)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindRecent(2)
			},
			wantCount: 2,
		},
		{
			name: "no matches for non-existent identityKey",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("key1", "host1", "txid1", 0)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindAdvertisements("nonexistent", "")
			},
			wantCount: 0,
		},
		{
			name: "no matches for non-existent host",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("key1", "host1", "txid1", 0)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindAdvertisements("key1", "nonexistent_host")
			},
			wantCount: 0,
		},
		{
			name: "store and delete",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("key1", "host1", "txid1", 0)
				_ = storage.DeleteRecord("txid1", 0)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindAdvertisements("key1", "")
			},
			wantCount: 0,
		},
		{
			name: "multiple outputs same txid",
			setup: func(storage *MockMessageBoxStorage) {
				_ = storage.StoreRecord("key1", "host1", "txid1", 0)
				_ = storage.StoreRecord("key1", "host2", "txid1", 1)
				_ = storage.StoreRecord("key1", "host3", "txid1", 2)
			},
			query: func(storage *MockMessageBoxStorage) ([]UTXOReference, error) {
				return storage.FindAdvertisements("key1", "")
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockMessageBoxStorage()
			tt.setup(storage)

			results, err := tt.query(storage)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tt.wantCount)
			}
		})
	}
}

// TestTableDrivenErrorInjection tests error injection scenarios
func TestTableDrivenErrorInjection(t *testing.T) {
	tests := []struct {
		name      string
		setupErr  func(storage *MockMessageBoxStorage)
		operation func(storage *MockMessageBoxStorage) error
		wantError bool
	}{
		{
			name: "store error",
			setupErr: func(storage *MockMessageBoxStorage) {
				storage.StoreError = assert.AnError
			},
			operation: func(storage *MockMessageBoxStorage) error {
				return storage.StoreRecord("key1", "host1", "txid1", 0)
			},
			wantError: true,
		},
		{
			name: "delete error",
			setupErr: func(storage *MockMessageBoxStorage) {
				storage.DeleteError = assert.AnError
			},
			operation: func(storage *MockMessageBoxStorage) error {
				return storage.DeleteRecord("txid1", 0)
			},
			wantError: true,
		},
		{
			name: "lookup error",
			setupErr: func(storage *MockMessageBoxStorage) {
				storage.LookupError = assert.AnError
			},
			operation: func(storage *MockMessageBoxStorage) error {
				_, err := storage.FindAdvertisements("key1", "")
				return err
			},
			wantError: true,
		},
		{
			name: "find all error",
			setupErr: func(storage *MockMessageBoxStorage) {
				storage.LookupError = assert.AnError
			},
			operation: func(storage *MockMessageBoxStorage) error {
				_, err := storage.FindAll()
				return err
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockMessageBoxStorage()
			tt.setupErr(storage)

			err := tt.operation(storage)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
