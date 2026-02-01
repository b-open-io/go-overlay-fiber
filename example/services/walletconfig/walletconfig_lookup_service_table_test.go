package walletconfig

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
	storage := NewMockWalletConfigStorage()
	service := NewWalletConfigLookupServiceWithStorage(storage)

	// Store some test data
	registration := &WalletConfigRegistration{
		ConfigID:         "test-config-id",
		Name:             "Test Wallet",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.example.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), "txid_test", 0, registration)
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing registryOperators",
			query:       map[string]interface{}{"configID": "test-config-id"},
			expectError: true,
			errorMsg:    "registryOperators",
		},
		{
			name:        "empty registryOperators array",
			query:       map[string]interface{}{"configID": "test-config-id", "registryOperators": []string{}},
			expectError: true,
			errorMsg:    "registryOperators",
		},
		{
			name:        "valid configID query",
			query:       map[string]interface{}{"configID": "test-config-id", "registryOperators": []string{"operator1"}},
			expectError: false,
		},
		{
			name:        "valid name query",
			query:       map[string]interface{}{"name": "Test Wallet", "registryOperators": []string{"operator1"}},
			expectError: false,
		},
		{
			name:        "valid name query - partial match",
			query:       map[string]interface{}{"name": "Test", "registryOperators": []string{"operator1"}},
			expectError: false,
		},
		{
			name:        "valid wab query",
			query:       map[string]interface{}{"wab": "https://wab.example.com", "registryOperators": []string{"operator1"}},
			expectError: false,
		},
		{
			name:        "valid storage query",
			query:       map[string]interface{}{"storage": "https://storage.example.com", "registryOperators": []string{"operator1"}},
			expectError: false,
		},
		{
			name:        "valid messagebox query",
			query:       map[string]interface{}{"messagebox": "https://messagebox.example.com", "registryOperators": []string{"operator1"}},
			expectError: false,
		},
		{
			name:        "valid list all query",
			query:       map[string]interface{}{"registryOperators": []string{"operator1"}},
			expectError: false,
		},
		{
			name:        "non-existent configID",
			query:       map[string]interface{}{"configID": "nonexistent", "registryOperators": []string{"operator1"}},
			expectError: false,
		},
		{
			name:        "wrong operator",
			query:       map[string]interface{}{"configID": "test-config-id", "registryOperators": []string{"wrong_operator"}},
			expectError: false,
		},
		{
			name:        "multiple operators - one matches",
			query:       map[string]interface{}{"configID": "test-config-id", "registryOperators": []string{"wrong_operator", "operator1"}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_walletconfig",
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
		setup     func(storage *MockWalletConfigStorage)
		query     func(storage *MockWalletConfigStorage) ([]UTXOReference, error)
		wantCount int
		wantError bool
	}{
		{
			name: "find by configID - single match",
			setup: func(storage *MockWalletConfigStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &WalletConfigRegistration{
					ConfigID:         "config1",
					Name:             "Wallet 1",
					Icon:             "icon1",
					WAB:              "wab1",
					Storage:          "storage1",
					Messagebox:       "mb1",
					Legal:            "legal1",
					RegistryOperator: "op1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &WalletConfigRegistration{
					ConfigID:         "config2",
					Name:             "Wallet 2",
					Icon:             "icon2",
					WAB:              "wab2",
					Storage:          "storage2",
					Messagebox:       "mb2",
					Legal:            "legal2",
					RegistryOperator: "op1",
				})
			},
			query: func(storage *MockWalletConfigStorage) ([]UTXOReference, error) {
				return storage.FindByConfigID(context.Background(), "config1", []string{"op1"})
			},
			wantCount: 1,
		},
		{
			name: "find by name - partial match",
			setup: func(storage *MockWalletConfigStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &WalletConfigRegistration{
					ConfigID:         "config1",
					Name:             "My Test Wallet",
					Icon:             "icon1",
					WAB:              "wab1",
					Storage:          "storage1",
					Messagebox:       "mb1",
					Legal:            "legal1",
					RegistryOperator: "op1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &WalletConfigRegistration{
					ConfigID:         "config2",
					Name:             "Another Wallet",
					Icon:             "icon2",
					WAB:              "wab2",
					Storage:          "storage2",
					Messagebox:       "mb2",
					Legal:            "legal2",
					RegistryOperator: "op1",
				})
			},
			query: func(storage *MockWalletConfigStorage) ([]UTXOReference, error) {
				return storage.FindByName(context.Background(), "test", []string{"op1"})
			},
			wantCount: 1,
		},
		{
			name: "find by WAB - multiple matches",
			setup: func(storage *MockWalletConfigStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &WalletConfigRegistration{
					ConfigID:         "config1",
					Name:             "Wallet 1",
					Icon:             "icon1",
					WAB:              "https://same-wab.com",
					Storage:          "storage1",
					Messagebox:       "mb1",
					Legal:            "legal1",
					RegistryOperator: "op1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &WalletConfigRegistration{
					ConfigID:         "config2",
					Name:             "Wallet 2",
					Icon:             "icon2",
					WAB:              "https://same-wab.com",
					Storage:          "storage2",
					Messagebox:       "mb2",
					Legal:            "legal2",
					RegistryOperator: "op1",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &WalletConfigRegistration{
					ConfigID:         "config3",
					Name:             "Wallet 3",
					Icon:             "icon3",
					WAB:              "https://different-wab.com",
					Storage:          "storage3",
					Messagebox:       "mb3",
					Legal:            "legal3",
					RegistryOperator: "op1",
				})
			},
			query: func(storage *MockWalletConfigStorage) ([]UTXOReference, error) {
				return storage.FindByWAB(context.Background(), "https://same-wab.com", []string{"op1"})
			},
			wantCount: 2,
		},
		{
			name: "find by storage - single match",
			setup: func(storage *MockWalletConfigStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &WalletConfigRegistration{
					ConfigID:         "config1",
					Name:             "Wallet 1",
					Icon:             "icon1",
					WAB:              "wab1",
					Storage:          "https://storage1.com",
					Messagebox:       "mb1",
					Legal:            "legal1",
					RegistryOperator: "op1",
				})
			},
			query: func(storage *MockWalletConfigStorage) ([]UTXOReference, error) {
				return storage.FindByStorage(context.Background(), "https://storage1.com", []string{"op1"})
			},
			wantCount: 1,
		},
		{
			name: "find by messagebox - single match",
			setup: func(storage *MockWalletConfigStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &WalletConfigRegistration{
					ConfigID:         "config1",
					Name:             "Wallet 1",
					Icon:             "icon1",
					WAB:              "wab1",
					Storage:          "storage1",
					Messagebox:       "https://mb1.com",
					Legal:            "legal1",
					RegistryOperator: "op1",
				})
			},
			query: func(storage *MockWalletConfigStorage) ([]UTXOReference, error) {
				return storage.FindByMessagebox(context.Background(), "https://mb1.com", []string{"op1"})
			},
			wantCount: 1,
		},
		{
			name: "list all - filters by operator",
			setup: func(storage *MockWalletConfigStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &WalletConfigRegistration{
					ConfigID:         "config1",
					Name:             "Wallet 1",
					Icon:             "icon1",
					WAB:              "wab1",
					Storage:          "storage1",
					Messagebox:       "mb1",
					Legal:            "legal1",
					RegistryOperator: "op1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &WalletConfigRegistration{
					ConfigID:         "config2",
					Name:             "Wallet 2",
					Icon:             "icon2",
					WAB:              "wab2",
					Storage:          "storage2",
					Messagebox:       "mb2",
					Legal:            "legal2",
					RegistryOperator: "op1",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &WalletConfigRegistration{
					ConfigID:         "config3",
					Name:             "Wallet 3",
					Icon:             "icon3",
					WAB:              "wab3",
					Storage:          "storage3",
					Messagebox:       "mb3",
					Legal:            "legal3",
					RegistryOperator: "op2",
				})
			},
			query: func(storage *MockWalletConfigStorage) ([]UTXOReference, error) {
				return storage.ListAll(context.Background(), []string{"op1"})
			},
			wantCount: 2,
		},
		{
			name: "no matches - wrong operator",
			setup: func(storage *MockWalletConfigStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &WalletConfigRegistration{
					ConfigID:         "config1",
					Name:             "Wallet 1",
					Icon:             "icon1",
					WAB:              "wab1",
					Storage:          "storage1",
					Messagebox:       "mb1",
					Legal:            "legal1",
					RegistryOperator: "op1",
				})
			},
			query: func(storage *MockWalletConfigStorage) ([]UTXOReference, error) {
				return storage.FindByConfigID(context.Background(), "config1", []string{"wrong_op"})
			},
			wantCount: 0,
		},
		{
			name: "duplicate prevention",
			setup: func(storage *MockWalletConfigStorage) {
				reg := &WalletConfigRegistration{
					ConfigID:         "config1",
					Name:             "Wallet 1",
					Icon:             "icon1",
					WAB:              "wab1",
					Storage:          "storage1",
					Messagebox:       "mb1",
					Legal:            "legal1",
					RegistryOperator: "op1",
				}
				_ = storage.StoreRecord(context.Background(), "txid1", 0, reg)
				// Try to store same config with different txid/output
				_ = storage.StoreRecord(context.Background(), "txid2", 1, reg)
			},
			query: func(storage *MockWalletConfigStorage) ([]UTXOReference, error) {
				return storage.ListAll(context.Background(), []string{"op1"})
			},
			wantCount: 1, // Should only have one record due to duplicate prevention
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockWalletConfigStorage()
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

// TestTableDrivenMultipleOperatorFiltering tests filtering by multiple registry operators
func TestTableDrivenMultipleOperatorFiltering(t *testing.T) {
	tests := []struct {
		name      string
		operators []string
		wantCount int
	}{
		{
			name:      "single operator - op1",
			operators: []string{"op1"},
			wantCount: 2,
		},
		{
			name:      "single operator - op2",
			operators: []string{"op2"},
			wantCount: 1,
		},
		{
			name:      "multiple operators - op1 and op2",
			operators: []string{"op1", "op2"},
			wantCount: 3,
		},
		{
			name:      "multiple operators - op1 and op3",
			operators: []string{"op1", "op3"},
			wantCount: 3,
		},
		{
			name:      "all operators",
			operators: []string{"op1", "op2", "op3"},
			wantCount: 4,
		},
		{
			name:      "non-existent operator",
			operators: []string{"nonexistent"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockWalletConfigStorage()

			// Set up test data with different operators
			_ = storage.StoreRecord(context.Background(), "txid1", 0, &WalletConfigRegistration{
				ConfigID:         "config1",
				Name:             "Wallet 1",
				Icon:             "icon1",
				WAB:              "wab1",
				Storage:          "storage1",
				Messagebox:       "mb1",
				Legal:            "legal1",
				RegistryOperator: "op1",
			})
			_ = storage.StoreRecord(context.Background(), "txid2", 0, &WalletConfigRegistration{
				ConfigID:         "config2",
				Name:             "Wallet 2",
				Icon:             "icon2",
				WAB:              "wab2",
				Storage:          "storage2",
				Messagebox:       "mb2",
				Legal:            "legal2",
				RegistryOperator: "op2",
			})
			_ = storage.StoreRecord(context.Background(), "txid3", 0, &WalletConfigRegistration{
				ConfigID:         "config3",
				Name:             "Wallet 3",
				Icon:             "icon3",
				WAB:              "wab3",
				Storage:          "storage3",
				Messagebox:       "mb3",
				Legal:            "legal3",
				RegistryOperator: "op1",
			})
			_ = storage.StoreRecord(context.Background(), "txid4", 0, &WalletConfigRegistration{
				ConfigID:         "config4",
				Name:             "Wallet 4",
				Icon:             "icon4",
				WAB:              "wab4",
				Storage:          "storage4",
				Messagebox:       "mb4",
				Legal:            "legal4",
				RegistryOperator: "op3",
			})

			results, err := storage.ListAll(context.Background(), tt.operators)
			require.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
		})
	}
}
