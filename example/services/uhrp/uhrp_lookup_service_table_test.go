package uhrp

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
	storage := NewMockUHRPStorage()
	service := NewUHRPLookupServiceWithStorage(storage)

	// Store some test data
	err := storage.StoreRecord("uhrp://testurl", "txid_test", 0, "02pubkey", "https://example.com/test.dat", 1735689600, 2048)
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
			errorMsg:    "lookup must specify",
		},
		{
			name:        "valid uhrpUrl query",
			query:       map[string]interface{}{"uhrpUrl": "uhrp://testurl"},
			expectError: false,
		},
		{
			name:        "valid outpoint query",
			query:       map[string]interface{}{"outpoint": "txid_test.0"},
			expectError: false,
		},
		{
			name:        "valid hostIdentityKey query",
			query:       map[string]interface{}{"hostIdentityKey": "02pubkey"},
			expectError: false,
		},
		{
			name:        "valid expiryTime query",
			query:       map[string]interface{}{"expiryTime": 1735689600},
			expectError: false,
		},
		{
			name:        "valid fileSize query",
			query:       map[string]interface{}{"fileSize": 2048},
			expectError: false,
		},
		{
			name:        "combined filters",
			query:       map[string]interface{}{"uhrpUrl": "uhrp://testurl", "hostIdentityKey": "02pubkey"},
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
			name:        "non-existent record",
			query:       map[string]interface{}{"uhrpUrl": "uhrp://nonexistent"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_uhrp",
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
		setup     func(storage *MockUHRPStorage)
		query     *UHRPQuery
		wantCount int
		wantError bool
	}{
		{
			name: "find by uhrpUrl - single match",
			setup: func(storage *MockUHRPStorage) {
				_ = storage.StoreRecord("uhrp://url1", "txid1", 0, "key1", "loc1", 100, 1024)
				_ = storage.StoreRecord("uhrp://url2", "txid2", 0, "key2", "loc2", 200, 2048)
			},
			query:     &UHRPQuery{UHRPUrl: "uhrp://url1"},
			wantCount: 1,
		},
		{
			name: "find by hostIdentityKey - multiple matches",
			setup: func(storage *MockUHRPStorage) {
				_ = storage.StoreRecord("uhrp://url1", "txid1", 0, "same_key", "loc1", 100, 1024)
				_ = storage.StoreRecord("uhrp://url2", "txid2", 0, "same_key", "loc2", 200, 2048)
				_ = storage.StoreRecord("uhrp://url3", "txid3", 0, "diff_key", "loc3", 300, 4096)
			},
			query:     &UHRPQuery{HostIdentityKey: "same_key"},
			wantCount: 2,
		},
		{
			name: "find by outpoint - exact match",
			setup: func(storage *MockUHRPStorage) {
				_ = storage.StoreRecord("uhrp://url1", "txid1", 0, "key1", "loc1", 100, 1024)
				_ = storage.StoreRecord("uhrp://url2", "txid1", 1, "key2", "loc2", 200, 2048)
			},
			query:     &UHRPQuery{Outpoint: "txid1.0"},
			wantCount: 1,
		},
		{
			name: "find by fileSize - single match",
			setup: func(storage *MockUHRPStorage) {
				_ = storage.StoreRecord("uhrp://url1", "txid1", 0, "key1", "loc1", 100, 1024)
				_ = storage.StoreRecord("uhrp://url2", "txid2", 0, "key2", "loc2", 200, 2048)
			},
			query:     &UHRPQuery{FileSize: 1024},
			wantCount: 1,
		},
		{
			name: "no matches",
			setup: func(storage *MockUHRPStorage) {
				_ = storage.StoreRecord("uhrp://url1", "txid1", 0, "key1", "loc1", 100, 1024)
			},
			query:     &UHRPQuery{UHRPUrl: "uhrp://nonexistent"},
			wantCount: 0,
		},
		{
			name:      "empty query - should error",
			setup:     func(storage *MockUHRPStorage) {},
			query:     &UHRPQuery{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockUHRPStorage()
			tt.setup(storage)

			results, err := storage.Lookup(tt.query)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tt.wantCount)
			}
		})
	}
}
