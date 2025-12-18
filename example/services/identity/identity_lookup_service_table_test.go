package identity

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTableDrivenQueryValidation tests various query scenarios using table-driven tests
func TestTableDrivenQueryValidation(t *testing.T) {
	storage := NewMockIdentityStorage()
	service := NewIdentityLookupServiceWithStorage(storage)

	// Create test certificate and certifier
	certifierKey, _ := ec.NewPrivateKey()
	certifier := certifierKey.PubKey().ToDERHex()

	cert := createTestCertificate("serial123")
	cert.Certifier = *certifierKey.PubKey()
	err := storage.StoreRecord(context.Background(), "txid_test", 0, cert)
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
			errorMsg:    "query parameters",
		},
		{
			name:        "valid serialNumber query",
			query:       map[string]interface{}{"serialNumber": "serial123"},
			expectError: false,
		},
		{
			name:        "valid certifiers only query",
			query:       map[string]interface{}{"certifiers": []string{certifier}},
			expectError: false,
		},
		{
			name: "valid attributes and certifiers query",
			query: map[string]interface{}{
				"attributes": map[string]string{"name": "John"},
				"certifiers": []string{certifier},
			},
			expectError: false,
		},
		{
			name: "valid identityKey and certifiers query",
			query: map[string]interface{}{
				"identityKey": cert.Subject.ToDERHex(),
				"certifiers":  []string{certifier},
			},
			expectError: false,
		},
		{
			name: "valid identityKey, certificateTypes, and certifiers query",
			query: map[string]interface{}{
				"identityKey":      cert.Subject.ToDERHex(),
				"certificateTypes": []string{string(cert.Type)},
				"certifiers":       []string{certifier},
			},
			expectError: false,
		},
		{
			name:        "attributes without certifiers - should error",
			query:       map[string]interface{}{"attributes": map[string]string{"name": "John"}},
			expectError: true,
			errorMsg:    "query parameters",
		},
		{
			name:        "identityKey without certifiers - should error",
			query:       map[string]interface{}{"identityKey": cert.Subject.ToDERHex()},
			expectError: true,
			errorMsg:    "query parameters",
		},
		{
			name:        "non-existent serialNumber",
			query:       map[string]interface{}{"serialNumber": "nonexistent"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON, err := json.Marshal(tt.query)
			require.NoError(t, err)

			question := &lookup.LookupQuestion{
				Service: "ls_identity",
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
		setup     func(storage *MockIdentityStorage) (certifier string, identityKey string)
		query     func(certifier, identityKey string) interface{}
		wantCount int
		wantError bool
	}{
		{
			name: "find by serialNumber - single match",
			setup: func(storage *MockIdentityStorage) (string, string) {
				cert1 := createTestCertificate("serial1")
				_ = storage.StoreRecord(context.Background(), "txid1", 0, cert1)
				cert2 := createTestCertificate("serial2")
				_ = storage.StoreRecord(context.Background(), "txid2", 0, cert2)
				return "", ""
			},
			query:     func(_, _ string) interface{} { return "serial1" },
			wantCount: 1,
		},
		{
			name: "find by certifiers - multiple matches",
			setup: func(storage *MockIdentityStorage) (string, string) {
				certifierKey, _ := ec.NewPrivateKey()
				certifier := certifierKey.PubKey().ToDERHex()

				cert1 := createTestCertificate("serial1")
				cert1.Certifier = *certifierKey.PubKey()
				_ = storage.StoreRecord(context.Background(), "txid1", 0, cert1)

				cert2 := createTestCertificate("serial2")
				cert2.Certifier = *certifierKey.PubKey()
				_ = storage.StoreRecord(context.Background(), "txid2", 0, cert2)

				otherCertifierKey, _ := ec.NewPrivateKey()
				cert3 := createTestCertificate("serial3")
				cert3.Certifier = *otherCertifierKey.PubKey()
				_ = storage.StoreRecord(context.Background(), "txid3", 0, cert3)

				return certifier, ""
			},
			query:     func(certifier, _ string) interface{} { return []string{certifier} },
			wantCount: 2,
		},
		{
			name: "find by identityKey and certifiers - single match",
			setup: func(storage *MockIdentityStorage) (string, string) {
				identityKeyPriv, _ := ec.NewPrivateKey()
				identityKey := identityKeyPriv.PubKey().ToDERHex()
				certifierKey, _ := ec.NewPrivateKey()
				certifier := certifierKey.PubKey().ToDERHex()

				cert1 := createTestCertificate("serial1")
				cert1.Subject = *identityKeyPriv.PubKey()
				cert1.Certifier = *certifierKey.PubKey()
				_ = storage.StoreRecord(context.Background(), "txid1", 0, cert1)

				otherIdentityKey, _ := ec.NewPrivateKey()
				cert2 := createTestCertificate("serial2")
				cert2.Subject = *otherIdentityKey.PubKey()
				cert2.Certifier = *certifierKey.PubKey()
				_ = storage.StoreRecord(context.Background(), "txid2", 0, cert2)

				return certifier, identityKey
			},
			query: func(certifier, identityKey string) interface{} {
				return map[string]interface{}{
					"identityKey": identityKey,
					"certifiers":  []string{certifier},
				}
			},
			wantCount: 1,
		},
		{
			name: "find by attributes and certifiers - fuzzy match",
			setup: func(storage *MockIdentityStorage) (string, string) {
				certifierKey, _ := ec.NewPrivateKey()
				certifier := certifierKey.PubKey().ToDERHex()

				cert := createTestCertificate("serial1")
				cert.Certifier = *certifierKey.PubKey()
				// The certificate fields are stored as base64, e.g., "Sm9obiBEb2U=" for "John Doe"
				// The fuzzy match works on the base64 values
				_ = storage.StoreRecord(context.Background(), "txid1", 0, cert)

				return certifier, ""
			},
			query: func(certifier, _ string) interface{} {
				return map[string]interface{}{
					// Match against the base64 encoded value "Sm9obiBEb2U=" (John Doe)
					// Fuzzy match "Sm9o" should match "Sm9obiBEb2U="
					"attributes": IdentityAttributes{"name": "Sm9o"},
					"certifiers": []string{certifier},
				}
			},
			wantCount: 1,
		},
		{
			name: "find by certificateType, identityKey and certifiers",
			setup: func(storage *MockIdentityStorage) (string, string) {
				identityKeyPriv, _ := ec.NewPrivateKey()
				identityKey := identityKeyPriv.PubKey().ToDERHex()
				certifierKey, _ := ec.NewPrivateKey()
				certifier := certifierKey.PubKey().ToDERHex()

				cert1 := createTestCertificate("serial1")
				cert1.Subject = *identityKeyPriv.PubKey()
				cert1.Certifier = *certifierKey.PubKey()
				cert1.Type = wallet.StringBase64("aWRlbnRpdHk=") // "identity" in base64
				_ = storage.StoreRecord(context.Background(), "txid1", 0, cert1)

				cert2 := createTestCertificate("serial2")
				cert2.Subject = *identityKeyPriv.PubKey()
				cert2.Certifier = *certifierKey.PubKey()
				cert2.Type = wallet.StringBase64("b3RoZXI=") // "other" in base64
				_ = storage.StoreRecord(context.Background(), "txid2", 0, cert2)

				return certifier, identityKey
			},
			query: func(certifier, identityKey string) interface{} {
				return map[string]interface{}{
					"identityKey":      identityKey,
					"certificateTypes": []string{"aWRlbnRpdHk="},
					"certifiers":       []string{certifier},
				}
			},
			wantCount: 1,
		},
		{
			name: "no matches",
			setup: func(storage *MockIdentityStorage) (string, string) {
				cert := createTestCertificate("serial1")
				_ = storage.StoreRecord(context.Background(), "txid1", 0, cert)
				return "", ""
			},
			query:     func(_, _ string) interface{} { return "nonexistent" },
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockIdentityStorage()
			certifier, identityKey := tt.setup(storage)
			queryParam := tt.query(certifier, identityKey)

			var results []UTXOReference
			var err error

			// Execute the appropriate storage method based on query type
			switch v := queryParam.(type) {
			case string: // serialNumber
				results, err = storage.FindByCertificateSerialNumber(context.Background(), v)
			case []string: // certifiers only
				results, err = storage.FindByCertifier(context.Background(), v)
			case map[string]interface{}:
				// Complex query - check what fields are present
				if attrs, ok := v["attributes"].(IdentityAttributes); ok {
					certifiers := v["certifiers"].([]string)
					results, err = storage.FindByAttribute(context.Background(), attrs, certifiers)
				} else if certTypes, ok := v["certificateTypes"].([]string); ok {
					identityKey := v["identityKey"].(string)
					certifiers := v["certifiers"].([]string)
					results, err = storage.FindByCertificateType(context.Background(), certTypes, identityKey, certifiers)
				} else if identityKey, ok := v["identityKey"].(string); ok {
					certifiers := v["certifiers"].([]string)
					results, err = storage.FindByIdentityKey(context.Background(), identityKey, certifiers)
				}
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
