package slackthreads

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTableDrivenQueryValidation tests various query scenarios using table-driven tests
func TestTableDrivenQueryValidation(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	service := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store some test data
	err := storage.StoreRecord("txid_test", 0, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty query - should return all",
			query:       map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "valid threadHash query",
			query:       map[string]interface{}{"threadHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			expectError: false,
		},
		{
			name:        "valid txid query",
			query:       map[string]interface{}{"txid": "txid_test"},
			expectError: false,
		},
		{
			name:        "valid limit",
			query:       map[string]interface{}{"limit": 10},
			expectError: false,
		},
		{
			name:        "valid skip",
			query:       map[string]interface{}{"skip": 5},
			expectError: false,
		},
		{
			name:        "valid sortOrder asc",
			query:       map[string]interface{}{"sortOrder": "asc"},
			expectError: false,
		},
		{
			name:        "valid sortOrder desc",
			query:       map[string]interface{}{"sortOrder": "desc"},
			expectError: false,
		},
		{
			name:        "combined filters",
			query:       map[string]interface{}{"threadHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "limit": 5},
			expectError: false,
		},
		{
			name:        "invalid limit - negative",
			query:       map[string]interface{}{"limit": -1},
			expectError: true,
			errorMsg:    "limit must be a non-negative number",
		},
		{
			name:        "invalid skip - negative",
			query:       map[string]interface{}{"skip": -5},
			expectError: true,
			errorMsg:    "skip must be a non-negative number",
		},
		{
			name:        "invalid startDate format",
			query:       map[string]interface{}{"startDate": "not-a-date"},
			expectError: true,
			errorMsg:    "invalid startDate",
		},
		{
			name:        "invalid endDate format",
			query:       map[string]interface{}{"endDate": "2024-13-45"},
			expectError: true,
			errorMsg:    "invalid endDate",
		},
		{
			name:        "valid date range",
			query:       map[string]interface{}{"startDate": "2024-01-01T00:00:00Z", "endDate": "2024-12-31T23:59:59Z"},
			expectError: false,
		},
		{
			name:        "non-existent threadHash",
			query:       map[string]interface{}{"threadHash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
			expectError: false,
		},
		{
			name:        "non-existent txid",
			query:       map[string]interface{}{"txid": "nonexistent_txid"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_slackthread",
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
		setup     func(storage *MockSlackThreadsStorage)
		query     func(storage *MockSlackThreadsStorage) ([]UTXOReference, error)
		wantCount int
		wantError bool
	}{
		{
			name: "find by threadHash - single match",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hash1111111111111111111111111111111111111111111111111111111111111111")
				_ = storage.StoreRecord("txid2", 0, "hash2222222222222222222222222222222222222222222222222222222222222222")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByThreadHash("hash1111111111111111111111111111111111111111111111111111111111111111", 50, 0, "desc")
			},
			wantCount: 1,
		},
		{
			name: "find by threadHash - multiple matches",
			setup: func(storage *MockSlackThreadsStorage) {
				sameHash := "hash3333333333333333333333333333333333333333333333333333333333333333"
				_ = storage.StoreRecord("txid1", 0, sameHash)
				_ = storage.StoreRecord("txid2", 0, sameHash)
				_ = storage.StoreRecord("txid3", 0, "different_hash_00000000000000000000000000000000000000000000000000")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByThreadHash("hash3333333333333333333333333333333333333333333333333333333333333333", 50, 0, "desc")
			},
			wantCount: 2,
		},
		{
			name: "find by txid - single match",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid_unique", 0, "hash4444444444444444444444444444444444444444444444444444444444444444")
				_ = storage.StoreRecord("txid_other", 0, "hash5555555555555555555555555555555555555555555555555555555555555555")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByTxid("txid_unique", 50, 0, "desc")
			},
			wantCount: 1,
		},
		{
			name: "find by txid - multiple outputs",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid_multi", 0, "hash6666666666666666666666666666666666666666666666666666666666666666")
				_ = storage.StoreRecord("txid_multi", 1, "hash7777777777777777777777777777777777777777777777777777777777777777")
				_ = storage.StoreRecord("txid_multi", 2, "hash8888888888888888888888888888888888888888888888888888888888888888")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByTxid("txid_multi", 50, 0, "desc")
			},
			wantCount: 3,
		},
		{
			name: "find all - no filters",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hash9999999999999999999999999999999999999999999999999999999999999999")
				_ = storage.StoreRecord("txid2", 0, "hashaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
				_ = storage.StoreRecord("txid3", 0, "hashbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindAll(50, 0, nil, nil, "desc")
			},
			wantCount: 3,
		},
		{
			name: "find all - with date filter",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hashcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
				_ = storage.StoreRecord("txid2", 0, "hashdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				startDate := time.Now().Add(-1 * time.Hour)
				endDate := time.Now().Add(1 * time.Hour)
				return storage.FindAll(50, 0, &startDate, &endDate, "desc")
			},
			wantCount: 2,
		},
		{
			name: "find all - date filter excludes all",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hasheeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				startDate := time.Now().Add(1 * time.Hour)
				endDate := time.Now().Add(2 * time.Hour)
				return storage.FindAll(50, 0, &startDate, &endDate, "desc")
			},
			wantCount: 0,
		},
		{
			name: "find with limit",
			setup: func(storage *MockSlackThreadsStorage) {
				for i := 0; i < 10; i++ {
					_ = storage.StoreRecord("txid"+string(rune('0'+i)), 0, "hashffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
				}
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByThreadHash("hashffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 5, 0, "desc")
			},
			wantCount: 5,
		},
		{
			name: "find with skip",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hash0000000000000000000000000000000000000000000000000000000000000000")
				_ = storage.StoreRecord("txid2", 0, "hash0000000000000000000000000000000000000000000000000000000000000000")
				_ = storage.StoreRecord("txid3", 0, "hash0000000000000000000000000000000000000000000000000000000000000000")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByThreadHash("hash0000000000000000000000000000000000000000000000000000000000000000", 50, 2, "desc")
			},
			wantCount: 1,
		},
		{
			name: "find with skip exceeds results",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hash1111111111111111111111111111111111111111111111111111111111111111")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByThreadHash("hash1111111111111111111111111111111111111111111111111111111111111111", 50, 10, "desc")
			},
			wantCount: 0,
		},
		{
			name: "no matches - empty threadHash",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hash2222222222222222222222222222222222222222222222222222222222222222")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByThreadHash("", 50, 0, "desc")
			},
			wantCount: 0,
		},
		{
			name: "no matches - empty txid",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hash3333333333333333333333333333333333333333333333333333333333333333")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByTxid("", 50, 0, "desc")
			},
			wantCount: 0,
		},
		{
			name: "no matches - non-existent threadHash",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hash4444444444444444444444444444444444444444444444444444444444444444")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByThreadHash("nonexistent000000000000000000000000000000000000000000000000000000", 50, 0, "desc")
			},
			wantCount: 0,
		},
		{
			name: "no matches - non-existent txid",
			setup: func(storage *MockSlackThreadsStorage) {
				_ = storage.StoreRecord("txid1", 0, "hash5555555555555555555555555555555555555555555555555555555555555555")
			},
			query: func(storage *MockSlackThreadsStorage) ([]UTXOReference, error) {
				return storage.FindByTxid("nonexistent_txid", 50, 0, "desc")
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockSlackThreadsStorage()
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
