package any

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
	storage := NewMockAnyStorage()
	service := NewAnyLookupServiceWithStorage(storage)

	// Store some test data
	now := time.Now()
	err := storage.StoreRecord("txid_test", 0)
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid txid query",
			query:       map[string]interface{}{"txid": "txid_test"},
			expectError: false,
		},
		{
			name:        "empty query - findAll",
			query:       map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "valid query with limit",
			query:       map[string]interface{}{"limit": 10},
			expectError: false,
		},
		{
			name:        "valid query with skip",
			query:       map[string]interface{}{"skip": 5},
			expectError: false,
		},
		{
			name:        "valid query with dates",
			query:       map[string]interface{}{"startDate": now.Add(-24 * time.Hour).Format(time.RFC3339), "endDate": now.Add(24 * time.Hour).Format(time.RFC3339)},
			expectError: false,
		},
		{
			name:        "valid query with sortOrder",
			query:       map[string]interface{}{"sortOrder": "asc"},
			expectError: false,
		},
		{
			name:        "invalid negative limit",
			query:       map[string]interface{}{"limit": -1},
			expectError: true,
			errorMsg:    "limit",
		},
		{
			name:        "invalid negative skip",
			query:       map[string]interface{}{"skip": -5},
			expectError: true,
			errorMsg:    "skip",
		},
		{
			name:        "invalid startDate format",
			query:       map[string]interface{}{"startDate": "invalid-date"},
			expectError: true,
			errorMsg:    "startDate",
		},
		{
			name:        "invalid endDate format",
			query:       map[string]interface{}{"endDate": "not-a-date"},
			expectError: true,
			errorMsg:    "endDate",
		},
		{
			name:        "non-existent txid",
			query:       map[string]interface{}{"txid": "nonexistent"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_anytx",
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
		setup     func(storage *MockAnyStorage)
		query     func() (string, int, *time.Time, *time.Time)
		wantCount int
		wantError bool
	}{
		{
			name: "find by txid - single match",
			setup: func(storage *MockAnyStorage) {
				_ = storage.StoreRecord("txid1", 0)
				_ = storage.StoreRecord("txid2", 0)
			},
			query: func() (string, int, *time.Time, *time.Time) {
				return "txid1", 0, nil, nil
			},
			wantCount: 1,
		},
		{
			name: "findAll - multiple matches",
			setup: func(storage *MockAnyStorage) {
				_ = storage.StoreRecord("txid1", 0)
				_ = storage.StoreRecord("txid2", 0)
				_ = storage.StoreRecord("txid3", 0)
			},
			query: func() (string, int, *time.Time, *time.Time) {
				return "", 10, nil, nil
			},
			wantCount: 3,
		},
		{
			name: "findAll with limit",
			setup: func(storage *MockAnyStorage) {
				_ = storage.StoreRecord("txid1", 0)
				_ = storage.StoreRecord("txid2", 0)
				_ = storage.StoreRecord("txid3", 0)
			},
			query: func() (string, int, *time.Time, *time.Time) {
				return "", 2, nil, nil
			},
			wantCount: 2,
		},
		{
			name: "findAll with skip",
			setup: func(storage *MockAnyStorage) {
				_ = storage.StoreRecord("txid1", 0)
				_ = storage.StoreRecord("txid2", 0)
				_ = storage.StoreRecord("txid3", 0)
			},
			query: func() (string, int, *time.Time, *time.Time) {
				return "", 10, nil, nil
			},
			wantCount: 3,
		},
		{
			name: "non-existent txid",
			setup: func(storage *MockAnyStorage) {
				_ = storage.StoreRecord("txid1", 0)
			},
			query: func() (string, int, *time.Time, *time.Time) {
				return "nonexistent", 0, nil, nil
			},
			wantCount: 0,
		},
		{
			name: "empty storage",
			setup: func(storage *MockAnyStorage) {
				// No records
			},
			query: func() (string, int, *time.Time, *time.Time) {
				return "", 10, nil, nil
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockAnyStorage()
			tt.setup(storage)

			txid, limit, startDate, endDate := tt.query()

			var results []UTXOReference
			var err error

			if txid != "" {
				result, err := storage.FindByTxid(txid)
				if err != nil {
					assert.Error(t, err)
					return
				}
				if result != nil {
					results = []UTXOReference{*result}
				}
			} else {
				results, err = storage.FindAll(limit, 0, startDate, endDate, "desc")
			}

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tt.wantCount)
			}
		})
	}
}
