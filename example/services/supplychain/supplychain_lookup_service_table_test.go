package supplychain

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
	storage := NewMockSupplyChainStorage()
	service := NewSupplyChainLookupServiceWithStorage(storage)

	// Store some test data
	err := storage.StoreRecord(context.Background(), "txid_test", 0, map[string]interface{}{
		"chainId": "test-chain-123",
		"data":    "test data",
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid chainId query",
			query:       map[string]interface{}{"chainId": "test-chain-123"},
			expectError: false,
		},
		{
			name:        "valid txid query",
			query:       map[string]interface{}{"txid": "txid_test"},
			expectError: false,
		},
		{
			name:        "valid query with limit",
			query:       map[string]interface{}{"chainId": "test-chain-123", "limit": 10},
			expectError: false,
		},
		{
			name:        "valid query with skip",
			query:       map[string]interface{}{"chainId": "test-chain-123", "skip": 0},
			expectError: false,
		},
		{
			name:        "valid query with limit and skip",
			query:       map[string]interface{}{"chainId": "test-chain-123", "limit": 5, "skip": 2},
			expectError: false,
		},
		{
			name:        "valid date range query",
			query:       map[string]interface{}{"startDate": "2025-01-01T00:00:00Z", "endDate": "2025-12-31T23:59:59Z"},
			expectError: false,
		},
		{
			name:        "valid sortOrder query",
			query:       map[string]interface{}{"txid": "txid_test", "sortOrder": "asc"},
			expectError: false,
		},
		{
			name:        "negative limit",
			query:       map[string]interface{}{"chainId": "test-chain-123", "limit": -5},
			expectError: true,
			errorMsg:    "limit must be a non-negative number",
		},
		{
			name:        "negative skip",
			query:       map[string]interface{}{"chainId": "test-chain-123", "skip": -1},
			expectError: true,
			errorMsg:    "skip must be a non-negative number",
		},
		{
			name:        "invalid startDate format",
			query:       map[string]interface{}{"startDate": "invalid-date"},
			expectError: true,
			errorMsg:    "invalid startDate format",
		},
		{
			name:        "invalid endDate format",
			query:       map[string]interface{}{"endDate": "not-a-date"},
			expectError: true,
			errorMsg:    "invalid endDate format",
		},
		{
			name:        "non-existent chainId",
			query:       map[string]interface{}{"chainId": "nonexistent-chain"},
			expectError: false,
		},
		{
			name:        "non-existent txid",
			query:       map[string]interface{}{"txid": "nonexistent_txid"},
			expectError: false,
		},
		{
			name:        "empty query (find all)",
			query:       map[string]interface{}{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_supplychain",
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
		setup     func(storage *MockSupplyChainStorage)
		query     func(storage *MockSupplyChainStorage) ([]UTXOReference, error)
		wantCount int
		wantError bool
	}{
		{
			name: "find by chainId - single match",
			setup: func(storage *MockSupplyChainStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, map[string]interface{}{"chainId": "chain1"})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, map[string]interface{}{"chainId": "chain2"})
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByChainID(context.Background(), "chain1", 10, 0)
			},
			wantCount: 1,
		},
		{
			name: "find by chainId - multiple matches",
			setup: func(storage *MockSupplyChainStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, map[string]interface{}{"chainId": "same-chain"})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, map[string]interface{}{"chainId": "same-chain"})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, map[string]interface{}{"chainId": "diff-chain"})
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByChainID(context.Background(), "same-chain", 10, 0)
			},
			wantCount: 2,
		},
		{
			name: "find by txid - single output",
			setup: func(storage *MockSupplyChainStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, map[string]interface{}{"chainId": "chain1"})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, map[string]interface{}{"chainId": "chain2"})
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByTxid(context.Background(), "txid1", 10, 0, "desc")
			},
			wantCount: 1,
		},
		{
			name: "find by txid - multiple outputs",
			setup: func(storage *MockSupplyChainStorage) {
				_ = storage.StoreRecord(context.Background(), "txid_multi", 0, map[string]interface{}{"chainId": "chain1"})
				_ = storage.StoreRecord(context.Background(), "txid_multi", 1, map[string]interface{}{"chainId": "chain2"})
				_ = storage.StoreRecord(context.Background(), "txid_multi", 2, map[string]interface{}{"chainId": "chain3"})
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByTxid(context.Background(), "txid_multi", 10, 0, "desc")
			},
			wantCount: 3,
		},
		{
			name: "find all - no filters",
			setup: func(storage *MockSupplyChainStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, map[string]interface{}{"chainId": "chain1"})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, map[string]interface{}{"chainId": "chain2"})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, map[string]interface{}{"chainId": "chain3"})
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindAll(context.Background(), 10, 0, nil, nil, "desc")
			},
			wantCount: 3,
		},
		{
			name: "find by chainId with limit",
			setup: func(storage *MockSupplyChainStorage) {
				for i := 0; i < 10; i++ {
					_ = storage.StoreRecord(context.Background(), "txid", i, map[string]interface{}{"chainId": "test-chain"})
				}
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByChainID(context.Background(), "test-chain", 5, 0)
			},
			wantCount: 5,
		},
		{
			name: "find by chainId with skip",
			setup: func(storage *MockSupplyChainStorage) {
				for i := 0; i < 10; i++ {
					_ = storage.StoreRecord(context.Background(), "txid", i, map[string]interface{}{"chainId": "test-chain"})
				}
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByChainID(context.Background(), "test-chain", 10, 5)
			},
			wantCount: 5,
		},
		{
			name: "find by chainId with limit and skip",
			setup: func(storage *MockSupplyChainStorage) {
				for i := 0; i < 10; i++ {
					_ = storage.StoreRecord(context.Background(), "txid", i, map[string]interface{}{"chainId": "test-chain"})
				}
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByChainID(context.Background(), "test-chain", 3, 2)
			},
			wantCount: 3,
		},
		{
			name: "no matches - empty chainId",
			setup: func(storage *MockSupplyChainStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, map[string]interface{}{"chainId": "chain1"})
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByChainID(context.Background(), "", 10, 0)
			},
			wantCount: 0,
		},
		{
			name: "no matches - empty txid",
			setup: func(storage *MockSupplyChainStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, map[string]interface{}{"chainId": "chain1"})
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByTxid(context.Background(), "", 10, 0, "desc")
			},
			wantCount: 0,
		},
		{
			name: "no matches - non-existent chainId",
			setup: func(storage *MockSupplyChainStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, map[string]interface{}{"chainId": "chain1"})
			},
			query: func(storage *MockSupplyChainStorage) ([]UTXOReference, error) {
				return storage.FindByChainID(context.Background(), "nonexistent", 10, 0)
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockSupplyChainStorage()
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
