package hello

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTableDrivenQueryValidation tests various query scenarios using table-driven tests
func TestTableDrivenQueryValidation(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	service := NewHelloWorldLookupServiceWithStorage(storage)

	// Store some test data
	err := storage.StoreRecord("txid_test", 0, "Hello World")
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid message query",
			query:       map[string]interface{}{"message": "Hello World"},
			expectError: false,
		},
		{
			name:        "valid empty query (find all)",
			query:       map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "valid message with limit",
			query:       map[string]interface{}{"message": "Hello", "limit": 10},
			expectError: false,
		},
		{
			name:        "valid message with skip",
			query:       map[string]interface{}{"message": "Hello", "skip": 5},
			expectError: false,
		},
		{
			name:        "valid with startDate",
			query:       map[string]interface{}{"startDate": "2025-01-01T00:00:00Z"},
			expectError: false,
		},
		{
			name:        "valid with endDate",
			query:       map[string]interface{}{"endDate": "2025-12-31T23:59:59Z"},
			expectError: false,
		},
		{
			name:        "valid with date range",
			query:       map[string]interface{}{"startDate": "2025-01-01T00:00:00Z", "endDate": "2025-12-31T23:59:59Z"},
			expectError: false,
		},
		{
			name:        "valid with sortOrder desc",
			query:       map[string]interface{}{"message": "Hello", "sortOrder": "desc"},
			expectError: false,
		},
		{
			name:        "valid with sortOrder asc",
			query:       map[string]interface{}{"message": "Hello", "sortOrder": "asc"},
			expectError: false,
		},
		{
			name:        "invalid limit - negative",
			query:       map[string]interface{}{"limit": -1},
			expectError: true,
			errorMsg:    "limit",
		},
		{
			name:        "invalid skip - negative",
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
			name:        "non-existent message",
			query:       map[string]interface{}{"message": "NonExistent"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_helloworld",
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
		setup     func(storage *MockHelloWorldStorage)
		message   string
		limit     int
		skip      int
		wantCount int
		wantError bool
	}{
		{
			name: "find by message - single match",
			setup: func(storage *MockHelloWorldStorage) {
				_ = storage.StoreRecord("txid1", 0, "Hello World")
				_ = storage.StoreRecord("txid2", 0, "Goodbye World")
			},
			message:   "Hello",
			limit:     10,
			skip:      0,
			wantCount: 1,
		},
		{
			name: "find by message - multiple matches",
			setup: func(storage *MockHelloWorldStorage) {
				_ = storage.StoreRecord("txid1", 0, "Hello World")
				_ = storage.StoreRecord("txid2", 0, "Hello Universe")
				_ = storage.StoreRecord("txid3", 0, "Hello Galaxy")
			},
			message:   "Hello",
			limit:     10,
			skip:      0,
			wantCount: 3,
		},
		{
			name: "find by message - partial match",
			setup: func(storage *MockHelloWorldStorage) {
				_ = storage.StoreRecord("txid1", 0, "Hello World")
				_ = storage.StoreRecord("txid2", 0, "Goodbye World")
			},
			message:   "World",
			limit:     10,
			skip:      0,
			wantCount: 2,
		},
		{
			name: "find with limit",
			setup: func(storage *MockHelloWorldStorage) {
				_ = storage.StoreRecord("txid1", 0, "Hello One")
				_ = storage.StoreRecord("txid2", 0, "Hello Two")
				_ = storage.StoreRecord("txid3", 0, "Hello Three")
			},
			message:   "Hello",
			limit:     2,
			skip:      0,
			wantCount: 2,
		},
		{
			name: "find with skip",
			setup: func(storage *MockHelloWorldStorage) {
				_ = storage.StoreRecord("txid1", 0, "Test One")
				_ = storage.StoreRecord("txid2", 0, "Test Two")
				_ = storage.StoreRecord("txid3", 0, "Test Three")
			},
			message:   "Test",
			limit:     10,
			skip:      1,
			wantCount: 2,
		},
		{
			name: "find with skip beyond results",
			setup: func(storage *MockHelloWorldStorage) {
				_ = storage.StoreRecord("txid1", 0, "Test One")
			},
			message:   "Test",
			limit:     10,
			skip:      5,
			wantCount: 0,
		},
		{
			name: "no matches",
			setup: func(storage *MockHelloWorldStorage) {
				_ = storage.StoreRecord("txid1", 0, "Hello World")
			},
			message:   "NonExistent",
			limit:     10,
			skip:      0,
			wantCount: 0,
		},
		{
			name:      "empty storage",
			setup:     func(storage *MockHelloWorldStorage) {},
			message:   "Anything",
			limit:     10,
			skip:      0,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockHelloWorldStorage()
			tt.setup(storage)

			results, err := storage.FindByMessage(tt.message, tt.limit, tt.skip, "desc")

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tt.wantCount)
			}
		})
	}
}

// TestTableDrivenFindAll tests FindAll with various date filters
func TestTableDrivenFindAll(t *testing.T) {
	storage := NewMockHelloWorldStorage()

	// Store some test records
	_ = storage.StoreRecord("txid1", 0, "Message 1")
	_ = storage.StoreRecord("txid2", 0, "Message 2")
	_ = storage.StoreRecord("txid3", 0, "Message 3")

	tests := []struct {
		name      string
		limit     int
		skip      int
		wantCount int
	}{
		{
			name:      "find all - no filters",
			limit:     10,
			skip:      0,
			wantCount: 3,
		},
		{
			name:      "find all - with limit",
			limit:     2,
			skip:      0,
			wantCount: 2,
		},
		{
			name:      "find all - with skip",
			limit:     10,
			skip:      1,
			wantCount: 2,
		},
		{
			name:      "find all - limit and skip",
			limit:     1,
			skip:      1,
			wantCount: 1,
		},
		{
			name:      "find all - skip beyond results",
			limit:     10,
			skip:      10,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.FindAll(tt.limit, tt.skip, nil, nil, "desc")
			assert.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
		})
	}
}

// TestTableDrivenStoreAndDelete tests store and delete operations
func TestTableDrivenStoreAndDelete(t *testing.T) {
	tests := []struct {
		name          string
		txid          string
		outputIndex   int
		message       string
		expectStored  bool
		deleteAfter   bool
		expectDeleted bool
	}{
		{
			name:          "store and verify",
			txid:          "txid1",
			outputIndex:   0,
			message:       "Test Message",
			expectStored:  true,
			deleteAfter:   false,
			expectDeleted: false,
		},
		{
			name:          "store then delete",
			txid:          "txid2",
			outputIndex:   1,
			message:       "Delete Me",
			expectStored:  true,
			deleteAfter:   true,
			expectDeleted: true,
		},
		{
			name:          "store with different output index",
			txid:          "txid3",
			outputIndex:   5,
			message:       "Another Message",
			expectStored:  true,
			deleteAfter:   false,
			expectDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockHelloWorldStorage()

			// Store the record
			err := storage.StoreRecord(tt.txid, tt.outputIndex, tt.message)
			assert.NoError(t, err)

			if tt.expectStored {
				// Verify it was stored
				key := testutil.MakeKey(tt.txid, tt.outputIndex)
				record, ok := storage.Get(key)
				assert.True(t, ok)
				assert.Equal(t, tt.txid, record.Txid)
				assert.Equal(t, tt.outputIndex, record.OutputIndex)
				assert.Equal(t, tt.message, record.Message)
			}

			if tt.deleteAfter {
				// Delete the record
				err := storage.DeleteRecord(tt.txid, tt.outputIndex)
				assert.NoError(t, err)

				if tt.expectDeleted {
					// Verify it was deleted
					key := testutil.MakeKey(tt.txid, tt.outputIndex)
					_, ok := storage.Get(key)
					assert.False(t, ok)
				}
			}
		})
	}
}
