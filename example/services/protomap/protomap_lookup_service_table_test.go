package protomap

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
	storage := NewMockProtoMapStorage()
	service := NewProtoMapLookupServiceWithStorage(storage)

	// Store some test data
	registration := ProtoMapRegistration{
		RegistryOperator: "02pubkey",
		ProtocolID: ProtocolID{
			SecurityLevel: 1,
			Protocol:      "test-protocol",
		},
		Name: "Test Protocol",
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
			name:        "empty query",
			query:       map[string]interface{}{},
			expectError: true,
			errorMsg:    "query parameters must include",
		},
		{
			name: "valid name query",
			query: map[string]interface{}{
				"name":              "Test Protocol",
				"registryOperators": []string{"02pubkey"},
			},
			expectError: false,
		},
		{
			name: "valid protocolID query",
			query: map[string]interface{}{
				"protocolID": map[string]interface{}{
					"securityLevel": 1,
					"protocol":      "test-protocol",
				},
				"registryOperators": []string{"02pubkey"},
			},
			expectError: false,
		},
		{
			name: "name without registryOperators",
			query: map[string]interface{}{
				"name": "Test Protocol",
			},
			expectError: true,
			errorMsg:    "query parameters must include",
		},
		{
			name: "protocolID without registryOperators",
			query: map[string]interface{}{
				"protocolID": map[string]interface{}{
					"securityLevel": 1,
					"protocol":      "test-protocol",
				},
			},
			expectError: true,
			errorMsg:    "query parameters must include",
		},
		{
			name: "empty registryOperators with name",
			query: map[string]interface{}{
				"name":              "Test Protocol",
				"registryOperators": []string{},
			},
			expectError: true,
			errorMsg:    "query parameters must include",
		},
		{
			name: "multiple registryOperators",
			query: map[string]interface{}{
				"name":              "Test Protocol",
				"registryOperators": []string{"operator1", "operator2"},
			},
			expectError: false,
		},
		{
			name: "non-existent protocol name",
			query: map[string]interface{}{
				"name":              "Nonexistent Protocol",
				"registryOperators": []string{"02pubkey"},
			},
			expectError: false,
		},
		{
			name: "non-existent protocolID",
			query: map[string]interface{}{
				"protocolID": map[string]interface{}{
					"securityLevel": 2,
					"protocol":      "nonexistent",
				},
				"registryOperators": []string{"02pubkey"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_protomap",
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
		setup     func(storage *MockProtoMapStorage)
		queryName string
		queryID   *ProtocolID
		operators []string
		wantCount int
		wantError bool
	}{
		{
			name: "find by name - single match",
			setup: func(storage *MockProtoMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, ProtoMapRegistration{
					RegistryOperator: "operator1",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-a"},
					Name:             "Protocol A",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, ProtoMapRegistration{
					RegistryOperator: "operator2",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-b"},
					Name:             "Protocol B",
				})
			},
			queryName: "Protocol A",
			operators: []string{"operator1"},
			wantCount: 1,
		},
		{
			name: "find by name - multiple matches with same operator",
			setup: func(storage *MockProtoMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, ProtoMapRegistration{
					RegistryOperator: "same_operator",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-a"},
					Name:             "Protocol X",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, ProtoMapRegistration{
					RegistryOperator: "same_operator",
					ProtocolID:       ProtocolID{SecurityLevel: 2, Protocol: "protocol-b"},
					Name:             "Protocol X",
				})
			},
			queryName: "Protocol X",
			operators: []string{"same_operator"},
			wantCount: 2,
		},
		{
			name: "find by name - filter by operator",
			setup: func(storage *MockProtoMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, ProtoMapRegistration{
					RegistryOperator: "operator1",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-a"},
					Name:             "Protocol Y",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, ProtoMapRegistration{
					RegistryOperator: "operator2",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-b"},
					Name:             "Protocol Y",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, ProtoMapRegistration{
					RegistryOperator: "operator3",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-c"},
					Name:             "Protocol Y",
				})
			},
			queryName: "Protocol Y",
			operators: []string{"operator1", "operator2"},
			wantCount: 2,
		},
		{
			name: "find by protocolID - single match",
			setup: func(storage *MockProtoMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, ProtoMapRegistration{
					RegistryOperator: "operator1",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "unique-protocol"},
					Name:             "Unique",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, ProtoMapRegistration{
					RegistryOperator: "operator2",
					ProtocolID:       ProtocolID{SecurityLevel: 2, Protocol: "other-protocol"},
					Name:             "Other",
				})
			},
			queryID:   &ProtocolID{SecurityLevel: 1, Protocol: "unique-protocol"},
			operators: []string{"operator1"},
			wantCount: 1,
		},
		{
			name: "find by protocolID - multiple operators",
			setup: func(storage *MockProtoMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, ProtoMapRegistration{
					RegistryOperator: "operator1",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "shared-protocol"},
					Name:             "Shared 1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, ProtoMapRegistration{
					RegistryOperator: "operator2",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "shared-protocol"},
					Name:             "Shared 2",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, ProtoMapRegistration{
					RegistryOperator: "operator3",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "shared-protocol"},
					Name:             "Shared 3",
				})
			},
			queryID:   &ProtocolID{SecurityLevel: 1, Protocol: "shared-protocol"},
			operators: []string{"operator1", "operator3"},
			wantCount: 2,
		},
		{
			name: "find by protocolID - security level mismatch",
			setup: func(storage *MockProtoMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, ProtoMapRegistration{
					RegistryOperator: "operator1",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol"},
					Name:             "Level 1",
				})
			},
			queryID:   &ProtocolID{SecurityLevel: 2, Protocol: "protocol"},
			operators: []string{"operator1"},
			wantCount: 0,
		},
		{
			name: "no matches - wrong name",
			setup: func(storage *MockProtoMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, ProtoMapRegistration{
					RegistryOperator: "operator1",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol"},
					Name:             "Existing",
				})
			},
			queryName: "Nonexistent",
			operators: []string{"operator1"},
			wantCount: 0,
		},
		{
			name: "no matches - wrong operator",
			setup: func(storage *MockProtoMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, ProtoMapRegistration{
					RegistryOperator: "operator1",
					ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol"},
					Name:             "Test",
				})
			},
			queryName: "Test",
			operators: []string{"different_operator"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockProtoMapStorage()
			tt.setup(storage)

			var results []UTXOReference
			var err error

			if tt.queryName != "" {
				results, err = storage.FindByName(context.Background(), tt.queryName, tt.operators)
			} else if tt.queryID != nil {
				results, err = storage.FindByProtocolID(context.Background(), *tt.queryID, tt.operators)
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
