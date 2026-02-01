package desktopintegrity

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
	storage := NewMockDesktopIntegrityStorage()
	service := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store some test data
	err := storage.StoreRecord(context.Background(), "txid_test", 0, "abc123def456", nil)
	require.NoError(t, err)
	err = storage.StoreRecord(context.Background(), "txid_test2", 1, "fedcba654321", nil)
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty query - finds all",
			query:       map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "valid fileHash query",
			query:       map[string]interface{}{"fileHash": "abc123def456"},
			expectError: false,
		},
		{
			name:        "valid txid query",
			query:       map[string]interface{}{"txid": "txid_test"},
			expectError: false,
		},
		{
			name:        "valid limit query",
			query:       map[string]interface{}{"limit": 10},
			expectError: false,
		},
		{
			name:        "valid skip query",
			query:       map[string]interface{}{"skip": 1},
			expectError: false,
		},
		{
			name:        "valid sortOrder query",
			query:       map[string]interface{}{"sortOrder": "asc"},
			expectError: false,
		},
		{
			name:        "valid startDate query",
			query:       map[string]interface{}{"startDate": "2025-01-01T00:00:00Z"},
			expectError: false,
		},
		{
			name:        "valid endDate query",
			query:       map[string]interface{}{"endDate": "2025-12-31T23:59:59Z"},
			expectError: false,
		},
		{
			name:        "combined fileHash and limit",
			query:       map[string]interface{}{"fileHash": "abc123def456", "limit": 5},
			expectError: false,
		},
		{
			name:        "combined txid and skip",
			query:       map[string]interface{}{"txid": "txid_test", "skip": 0},
			expectError: false,
		},
		{
			name:        "date range query",
			query:       map[string]interface{}{"startDate": "2025-01-01T00:00:00Z", "endDate": "2025-12-31T23:59:59Z"},
			expectError: false,
		},
		{
			name:        "negative limit - should error",
			query:       map[string]interface{}{"limit": -1},
			expectError: true,
			errorMsg:    "limit",
		},
		{
			name:        "negative skip - should error",
			query:       map[string]interface{}{"skip": -1},
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
			name:        "non-existent fileHash - no error, empty results",
			query:       map[string]interface{}{"fileHash": "nonexistent"},
			expectError: false,
		},
		{
			name:        "non-existent txid - no error, empty results",
			query:       map[string]interface{}{"txid": "nonexistent"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_desktopintegrity",
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
				assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)
			}
		})
	}
}

// TestTableDrivenStorageOperations tests storage operations with table-driven tests
func TestTableDrivenStorageOperations(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(storage *MockDesktopIntegrityStorage)
		query     func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error)
		wantCount int
		wantError bool
	}{
		{
			name: "find by fileHash - single match",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "hash2", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByFileHash(context.Background(), "hash1", 50, 0, "desc")
			},
			wantCount: 1,
		},
		{
			name: "find by fileHash - multiple matches",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "samehash", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "samehash", nil)
				_ = storage.StoreRecord(context.Background(), "txid3", 0, "different", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByFileHash(context.Background(), "samehash", 50, 0, "desc")
			},
			wantCount: 2,
		},
		{
			name: "find by txid - single match",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
				_ = storage.StoreRecord(context.Background(), "txid1", 1, "hash2", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "hash3", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByTxid(context.Background(), "txid2", 50, 0, "desc")
			},
			wantCount: 1,
		},
		{
			name: "find by txid - multiple outputs",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
				_ = storage.StoreRecord(context.Background(), "txid1", 1, "hash2", nil)
				_ = storage.StoreRecord(context.Background(), "txid1", 2, "hash3", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByTxid(context.Background(), "txid1", 50, 0, "desc")
			},
			wantCount: 3,
		},
		{
			name: "find all - no filters",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "hash2", nil)
				_ = storage.StoreRecord(context.Background(), "txid3", 0, "hash3", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindAll(context.Background(), 50, 0, nil, nil, "desc")
			},
			wantCount: 3,
		},
		{
			name: "find with limit",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "samehash", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "samehash", nil)
				_ = storage.StoreRecord(context.Background(), "txid3", 0, "samehash", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByFileHash(context.Background(), "samehash", 2, 0, "desc")
			},
			wantCount: 2,
		},
		{
			name: "find with skip",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "samehash", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "samehash", nil)
				_ = storage.StoreRecord(context.Background(), "txid3", 0, "samehash", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByFileHash(context.Background(), "samehash", 50, 2, "desc")
			},
			wantCount: 1,
		},
		{
			name: "find with skip beyond results",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByFileHash(context.Background(), "hash1", 50, 10, "desc")
			},
			wantCount: 0,
		},
		{
			name: "empty fileHash returns empty",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByFileHash(context.Background(), "", 50, 0, "desc")
			},
			wantCount: 0,
		},
		{
			name: "empty txid returns empty",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByTxid(context.Background(), "", 50, 0, "desc")
			},
			wantCount: 0,
		},
		{
			name: "no matches",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
			},
			query: func(storage *MockDesktopIntegrityStorage) ([]UTXOReference, error) {
				return storage.FindByFileHash(context.Background(), "nonexistent", 50, 0, "desc")
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockDesktopIntegrityStorage()
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

// TestTableDrivenDateFiltering tests date range filtering with table-driven tests
func TestTableDrivenDateFiltering(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	tests := []struct {
		name      string
		setup     func(storage *MockDesktopIntegrityStorage)
		startDate *time.Time
		endDate   *time.Time
		wantCount int
	}{
		{
			name: "no date filters - returns all",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "hash2", nil)
			},
			startDate: nil,
			endDate:   nil,
			wantCount: 2,
		},
		{
			name: "start date filter - includes records after",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "hash2", nil)
			},
			startDate: &yesterday,
			endDate:   nil,
			wantCount: 2,
		},
		{
			name: "end date filter - includes records before",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "hash2", nil)
			},
			startDate: nil,
			endDate:   &tomorrow,
			wantCount: 2,
		},
		{
			name: "both date filters - includes records in range",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
				_ = storage.StoreRecord(context.Background(), "txid2", 0, "hash2", nil)
			},
			startDate: &yesterday,
			endDate:   &tomorrow,
			wantCount: 2,
		},
		{
			name: "start date too recent - excludes all",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
			},
			startDate: &tomorrow,
			endDate:   nil,
			wantCount: 0,
		},
		{
			name: "end date too early - excludes all",
			setup: func(storage *MockDesktopIntegrityStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, "hash1", nil)
			},
			startDate: nil,
			endDate:   &yesterday,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockDesktopIntegrityStorage()
			tt.setup(storage)

			results, err := storage.FindAll(context.Background(), 50, 0, tt.startDate, tt.endDate, "desc")

			assert.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
		})
	}
}
