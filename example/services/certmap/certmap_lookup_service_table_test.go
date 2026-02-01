package certmap

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
	storage := NewMockCertMapStorage()
	service := NewCertMapLookupServiceWithStorage(storage)

	// Store some test data
	registration := &CertMapRegistration{
		Type:             "test-cert-type",
		Name:             "Test Certificate",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
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
			name:        "empty query",
			query:       map[string]interface{}{},
			expectError: true,
			errorMsg:    "registryOperators",
		},
		{
			name:        "missing registryOperators",
			query:       map[string]interface{}{"type": "test-cert-type"},
			expectError: true,
			errorMsg:    "registryOperators",
		},
		{
			name:        "missing type and name",
			query:       map[string]interface{}{"registryOperators": []string{"operator1"}},
			expectError: true,
			errorMsg:    "type or name",
		},
		{
			name: "valid type query",
			query: map[string]interface{}{
				"type":              "test-cert-type",
				"registryOperators": []string{"operator1"},
			},
			expectError: false,
		},
		{
			name: "valid name query",
			query: map[string]interface{}{
				"name":              "Certificate",
				"registryOperators": []string{"operator1"},
			},
			expectError: false,
		},
		{
			name: "valid type and name query",
			query: map[string]interface{}{
				"type":              "test-cert-type",
				"name":              "Test",
				"registryOperators": []string{"operator1"},
			},
			expectError: false,
		},
		{
			name: "non-existent type",
			query: map[string]interface{}{
				"type":              "nonexistent-type",
				"registryOperators": []string{"operator1"},
			},
			expectError: false,
		},
		{
			name: "non-matching operator",
			query: map[string]interface{}{
				"type":              "test-cert-type",
				"registryOperators": []string{"different-operator"},
			},
			expectError: false,
		},
		{
			name: "empty registryOperators array",
			query: map[string]interface{}{
				"type":              "test-cert-type",
				"registryOperators": []string{},
			},
			expectError: true,
			errorMsg:    "registryOperators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_certmap",
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
		setup     func(storage *MockCertMapStorage)
		queryType string
		queryName string
		operators []string
		wantCount int
		wantError bool
	}{
		{
			name: "find by type - single match",
			setup: func(storage *MockCertMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &CertMapRegistration{
					Type:             "cert-type-1",
					Name:             "Certificate 1",
					IconURL:          "https://example.com/icon1.png",
					Description:      "Description 1",
					DocumentationURL: "https://example.com/docs1",
					CertFields:       map[string]interface{}{"field1": "value1"},
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &CertMapRegistration{
					Type:             "cert-type-2",
					Name:             "Certificate 2",
					IconURL:          "https://example.com/icon2.png",
					Description:      "Description 2",
					DocumentationURL: "https://example.com/docs2",
					CertFields:       map[string]interface{}{"field2": "value2"},
					RegistryOperator: "operator1",
				})
			},
			queryType: "cert-type-1",
			operators: []string{"operator1"},
			wantCount: 1,
		},
		{
			name: "find by type - multiple matches",
			setup: func(storage *MockCertMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &CertMapRegistration{
					Type:             "same-type",
					Name:             "Certificate 1",
					IconURL:          "https://example.com/icon1.png",
					Description:      "Description 1",
					DocumentationURL: "https://example.com/docs1",
					CertFields:       map[string]interface{}{"field1": "value1"},
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &CertMapRegistration{
					Type:             "same-type",
					Name:             "Certificate 2",
					IconURL:          "https://example.com/icon2.png",
					Description:      "Description 2",
					DocumentationURL: "https://example.com/docs2",
					CertFields:       map[string]interface{}{"field2": "value2"},
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &CertMapRegistration{
					Type:             "different-type",
					Name:             "Certificate 3",
					IconURL:          "https://example.com/icon3.png",
					Description:      "Description 3",
					DocumentationURL: "https://example.com/docs3",
					CertFields:       map[string]interface{}{"field3": "value3"},
					RegistryOperator: "operator1",
				})
			},
			queryType: "same-type",
			operators: []string{"operator1"},
			wantCount: 2,
		},
		{
			name: "find by name - fuzzy match",
			setup: func(storage *MockCertMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &CertMapRegistration{
					Type:             "type1",
					Name:             "Test Certificate",
					IconURL:          "https://example.com/icon1.png",
					Description:      "Description 1",
					DocumentationURL: "https://example.com/docs1",
					CertFields:       map[string]interface{}{"field1": "value1"},
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &CertMapRegistration{
					Type:             "type2",
					Name:             "Another Certificate",
					IconURL:          "https://example.com/icon2.png",
					Description:      "Description 2",
					DocumentationURL: "https://example.com/docs2",
					CertFields:       map[string]interface{}{"field2": "value2"},
					RegistryOperator: "operator1",
				})
			},
			queryName: "Certificate",
			operators: []string{"operator1"},
			wantCount: 2,
		},
		{
			name: "filter by registry operator - match",
			setup: func(storage *MockCertMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &CertMapRegistration{
					Type:             "cert-type",
					Name:             "Certificate 1",
					IconURL:          "https://example.com/icon1.png",
					Description:      "Description 1",
					DocumentationURL: "https://example.com/docs1",
					CertFields:       map[string]interface{}{"field1": "value1"},
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &CertMapRegistration{
					Type:             "cert-type",
					Name:             "Certificate 2",
					IconURL:          "https://example.com/icon2.png",
					Description:      "Description 2",
					DocumentationURL: "https://example.com/docs2",
					CertFields:       map[string]interface{}{"field2": "value2"},
					RegistryOperator: "operator2",
				})
			},
			queryType: "cert-type",
			operators: []string{"operator1"},
			wantCount: 1,
		},
		{
			name: "filter by registry operator - multiple operators",
			setup: func(storage *MockCertMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &CertMapRegistration{
					Type:             "cert-type",
					Name:             "Certificate 1",
					IconURL:          "https://example.com/icon1.png",
					Description:      "Description 1",
					DocumentationURL: "https://example.com/docs1",
					CertFields:       map[string]interface{}{"field1": "value1"},
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, &CertMapRegistration{
					Type:             "cert-type",
					Name:             "Certificate 2",
					IconURL:          "https://example.com/icon2.png",
					Description:      "Description 2",
					DocumentationURL: "https://example.com/docs2",
					CertFields:       map[string]interface{}{"field2": "value2"},
					RegistryOperator: "operator2",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, &CertMapRegistration{
					Type:             "cert-type",
					Name:             "Certificate 3",
					IconURL:          "https://example.com/icon3.png",
					Description:      "Description 3",
					DocumentationURL: "https://example.com/docs3",
					CertFields:       map[string]interface{}{"field3": "value3"},
					RegistryOperator: "operator3",
				})
			},
			queryType: "cert-type",
			operators: []string{"operator1", "operator2"},
			wantCount: 2,
		},
		{
			name: "no matches - wrong operator",
			setup: func(storage *MockCertMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &CertMapRegistration{
					Type:             "cert-type",
					Name:             "Certificate 1",
					IconURL:          "https://example.com/icon1.png",
					Description:      "Description 1",
					DocumentationURL: "https://example.com/docs1",
					CertFields:       map[string]interface{}{"field1": "value1"},
					RegistryOperator: "operator1",
				})
			},
			queryType: "cert-type",
			operators: []string{"different-operator"},
			wantCount: 0,
		},
		{
			name: "no matches - wrong type",
			setup: func(storage *MockCertMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &CertMapRegistration{
					Type:             "cert-type",
					Name:             "Certificate 1",
					IconURL:          "https://example.com/icon1.png",
					Description:      "Description 1",
					DocumentationURL: "https://example.com/docs1",
					CertFields:       map[string]interface{}{"field1": "value1"},
					RegistryOperator: "operator1",
				})
			},
			queryType: "nonexistent-type",
			operators: []string{"operator1"},
			wantCount: 0,
		},
		{
			name: "fuzzy name search - partial match",
			setup: func(storage *MockCertMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, &CertMapRegistration{
					Type:             "type1",
					Name:             "My Special Certificate",
					IconURL:          "https://example.com/icon1.png",
					Description:      "Description 1",
					DocumentationURL: "https://example.com/docs1",
					CertFields:       map[string]interface{}{"field1": "value1"},
					RegistryOperator: "operator1",
				})
			},
			queryName: "Special",
			operators: []string{"operator1"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockCertMapStorage()
			tt.setup(storage)

			var results []UTXOReference
			var err error

			if tt.queryType != "" {
				results, err = storage.FindByType(context.Background(), tt.queryType, tt.operators)
			} else if tt.queryName != "" {
				results, err = storage.FindByName(context.Background(), tt.queryName, tt.operators)
			} else {
				t.Fatal("test must specify either queryType or queryName")
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
