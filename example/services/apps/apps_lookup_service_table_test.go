package apps

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
	storage := NewMockAppsStorage()
	service := NewAppsLookupServiceWithStorage(storage)

	// Store some test data
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test Calculator",
		Description: "A test calculator app",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://testcalc.com",
		Domain:      "testcalc.com",
		Publisher:   "02pubkey123",
		ReleaseDate: "2025-01-01",
		Category:    "Utilities",
		Tags:        []string{"productivity", "calculator"},
	}
	err := storage.StoreRecord(context.Background(), "txid_test", 0, metadata)
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
		minResults  int
	}{
		{
			name:       "empty query - returns all apps",
			query:      map[string]interface{}{},
			minResults: 0,
		},
		{
			name:       "valid domain query",
			query:      map[string]interface{}{"domain": "testcalc.com"},
			minResults: 1,
		},
		{
			name:       "valid publisher query",
			query:      map[string]interface{}{"publisher": "02pubkey123"},
			minResults: 1,
		},
		{
			name:       "valid name query (fuzzy)",
			query:      map[string]interface{}{"name": "calc"},
			minResults: 1,
		},
		{
			name:       "valid outpoint query",
			query:      map[string]interface{}{"outpoint": "txid_test.0"},
			minResults: 1,
		},
		{
			name:       "valid tags query",
			query:      map[string]interface{}{"tags": []string{"productivity"}},
			minResults: 1,
		},
		{
			name:       "valid category query",
			query:      map[string]interface{}{"category": "Utilities"},
			minResults: 1,
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
			name:       "non-existent domain",
			query:      map[string]interface{}{"domain": "nonexistent.com"},
			minResults: 0,
		},
		{
			name:       "non-existent publisher",
			query:      map[string]interface{}{"publisher": "nonexistent_key"},
			minResults: 0,
		},
		{
			name:       "non-existent category",
			query:      map[string]interface{}{"category": "NonExistent"},
			minResults: 0,
		},
		{
			name:       "pagination with limit",
			query:      map[string]interface{}{"domain": "testcalc.com", "limit": 1},
			minResults: 0,
		},
		{
			name:       "pagination with skip",
			query:      map[string]interface{}{"domain": "testcalc.com", "skip": 10},
			minResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_apps",
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
				results, ok := answer.Result.([]UTXOReference)
				require.True(t, ok)
				assert.GreaterOrEqual(t, len(results), tt.minResults)
			}
		})
	}
}

// TestTableDrivenStorageOperations tests storage operations with table-driven tests
func TestTableDrivenStorageOperations(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(storage *MockAppsStorage)
		operation func(storage *MockAppsStorage) ([]UTXOReference, error)
		wantCount int
		wantError bool
	}{
		{
			name: "find by domain - single match",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 2",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "key2",
					ReleaseDate: "2025-01-02",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindByDomain(context.Background(), "app1.com", 0, 0, "")
			},
			wantCount: 1,
		},
		{
			name: "find by publisher - multiple matches",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "same_key",
					ReleaseDate: "2025-01-01",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 2",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "same_key",
					ReleaseDate: "2025-01-02",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 3",
					Description: "App 3",
					Icon:        "icon3.png",
					HTTPURL:     "https://app3.com",
					Domain:      "app3.com",
					Publisher:   "diff_key",
					ReleaseDate: "2025-01-03",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindByPublisher(context.Background(), "same_key", 0, 0, "")
			},
			wantCount: 2,
		},
		{
			name: "find by outpoint - exact match",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
				})
				_ = storage.StoreRecord(context.Background(), "txid1", 1, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 2",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "key2",
					ReleaseDate: "2025-01-02",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindByOutpoint(context.Background(), "txid1.0")
			},
			wantCount: 1,
		},
		{
			name: "find by name fuzzy - partial match",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "Calculator Pro",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "Simple Calculator",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "key2",
					ReleaseDate: "2025-01-02",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "Notes",
					Description: "App 3",
					Icon:        "icon3.png",
					HTTPURL:     "https://app3.com",
					Domain:      "app3.com",
					Publisher:   "key3",
					ReleaseDate: "2025-01-03",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindByNameFuzzy(context.Background(), "calc", 0, 0, "")
			},
			wantCount: 2,
		},
		{
			name: "find by tags - any tag matches",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
					Tags:        []string{"productivity", "work"},
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 2",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "key2",
					ReleaseDate: "2025-01-02",
					Tags:        []string{"finance", "money"},
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindByTags(context.Background(), []string{"productivity"}, 0, 0, "")
			},
			wantCount: 1,
		},
		{
			name: "find by category - single match",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
					Category:    "Games",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 2",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "key2",
					ReleaseDate: "2025-01-02",
					Category:    "Utilities",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindByCategory(context.Background(), "Games", 0, 0, "")
			},
			wantCount: 1,
		},
		{
			name: "find all apps",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 2",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "key2",
					ReleaseDate: "2025-01-02",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 3",
					Description: "App 3",
					Icon:        "icon3.png",
					HTTPURL:     "https://app3.com",
					Domain:      "app3.com",
					Publisher:   "key3",
					ReleaseDate: "2025-01-03",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindAllApps(context.Background(), 0, 0, "")
			},
			wantCount: 3,
		},
		{
			name: "pagination with limit",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 2",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "key2",
					ReleaseDate: "2025-01-02",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 3",
					Description: "App 3",
					Icon:        "icon3.png",
					HTTPURL:     "https://app3.com",
					Domain:      "app3.com",
					Publisher:   "key3",
					ReleaseDate: "2025-01-03",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindAllApps(context.Background(), 2, 0, "")
			},
			wantCount: 2,
		},
		{
			name: "pagination with skip",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 2",
					Description: "App 2",
					Icon:        "icon2.png",
					HTTPURL:     "https://app2.com",
					Domain:      "app2.com",
					Publisher:   "key2",
					ReleaseDate: "2025-01-02",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 3",
					Description: "App 3",
					Icon:        "icon3.png",
					HTTPURL:     "https://app3.com",
					Domain:      "app3.com",
					Publisher:   "key3",
					ReleaseDate: "2025-01-03",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindAllApps(context.Background(), 0, 1, "")
			},
			wantCount: 2,
		},
		{
			name: "no matches",
			setup: func(storage *MockAppsStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &PublishedAppMetadata{
					Version:     "1.0.0",
					Name:        "App 1",
					Description: "App 1",
					Icon:        "icon1.png",
					HTTPURL:     "https://app1.com",
					Domain:      "app1.com",
					Publisher:   "key1",
					ReleaseDate: "2025-01-01",
				})
			},
			operation: func(storage *MockAppsStorage) ([]UTXOReference, error) {
				return storage.FindByDomain(context.Background(), "nonexistent.com", 0, 0, "")
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockAppsStorage()
			tt.setup(storage)

			results, err := tt.operation(storage)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tt.wantCount)
			}
		})
	}
}
