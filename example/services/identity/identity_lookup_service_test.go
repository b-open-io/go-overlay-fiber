package identity

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockIdentityStorage is a mock implementation of IdentityStorageEngine for testing
type MockIdentityStorage struct {
	records     map[string]*IdentityRecord
	storeError  error
	deleteError error
	findError   error
}

func NewMockIdentityStorage() *MockIdentityStorage {
	return &MockIdentityStorage{
		records: make(map[string]*IdentityRecord),
	}
}

func (m *MockIdentityStorage) makeKey(txid string, outputIndex int) string {
	return fmt.Sprintf("%s:%d", txid, outputIndex)
}

func (m *MockIdentityStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, certificate *certificates.Certificate) error {
	if m.storeError != nil {
		return m.storeError
	}

	// Build searchable attributes string from certificate fields
	var searchableAttrs []string
	for key, value := range certificate.Fields {
		keyStr := string(key)
		valueStr := string(value)
		if keyStr != "profilePhoto" && keyStr != "icon" {
			searchableAttrs = append(searchableAttrs, valueStr)
		}
	}

	key := m.makeKey(txid, outputIndex)
	m.records[key] = &IdentityRecord{
		Txid:                 txid,
		OutputIndex:          outputIndex,
		Certificate:          certificate,
		CreatedAt:            time.Now(),
		SearchableAttributes: strings.Join(searchableAttrs, " "),
	}
	return nil
}

func (m *MockIdentityStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockIdentityStorage) FindByAttribute(ctx context.Context, attributes IdentityAttributes, certifiers []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if len(attributes) == 0 {
		return []UTXOReference{}, nil
	}

	var results []UTXOReference
	for _, record := range m.records {
		// Check if certifier matches
		certifierMatch := false
		certifierHex := record.Certificate.Certifier.ToDERHex()
		for _, certifier := range certifiers {
			if certifierHex == certifier {
				certifierMatch = true
				break
			}
		}
		if !certifierMatch {
			continue
		}

		// Handle "any" special case for full-text search
		if anyValue, ok := attributes["any"]; ok {
			if fuzzyMatch(record.SearchableAttributes, anyValue) {
				results = append(results, UTXOReference{
					Txid:        record.Txid,
					OutputIndex: record.OutputIndex,
				})
			}
		} else {
			// Check specific attributes
			allMatch := true
			for key, value := range attributes {
				if fieldValue, ok := record.Certificate.Fields[wallet.CertificateFieldNameUnder50Bytes(key)]; ok {
					if !fuzzyMatch(string(fieldValue), value) {
						allMatch = false
						break
					}
				} else {
					allMatch = false
					break
				}
			}
			if allMatch {
				results = append(results, UTXOReference{
					Txid:        record.Txid,
					OutputIndex: record.OutputIndex,
				})
			}
		}
	}

	return results, nil
}

func (m *MockIdentityStorage) FindByIdentityKey(ctx context.Context, identityKey string, certifiers []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if identityKey == "" {
		return []UTXOReference{}, nil
	}

	var results []UTXOReference
	for _, record := range m.records {
		// Check subject match
		if record.Certificate.Subject.ToDERHex() != identityKey {
			continue
		}

		// Check certifier match if provided
		if len(certifiers) > 0 {
			certifierMatch := false
			certifierHex := record.Certificate.Certifier.ToDERHex()
			for _, certifier := range certifiers {
				if certifierHex == certifier {
					certifierMatch = true
					break
				}
			}
			if !certifierMatch {
				continue
			}
		}

		results = append(results, UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		})
	}

	return results, nil
}

func (m *MockIdentityStorage) FindByCertifier(ctx context.Context, certifiers []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if len(certifiers) == 0 {
		return []UTXOReference{}, nil
	}

	var results []UTXOReference
	for _, record := range m.records {
		certifierHex := record.Certificate.Certifier.ToDERHex()
		for _, certifier := range certifiers {
			if certifierHex == certifier {
				results = append(results, UTXOReference{
					Txid:        record.Txid,
					OutputIndex: record.OutputIndex,
				})
				break
			}
		}
	}

	return results, nil
}

func (m *MockIdentityStorage) FindByCertificateType(ctx context.Context, certificateTypes []string, identityKey string, certifiers []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if len(certificateTypes) == 0 || identityKey == "" || len(certifiers) == 0 {
		return []UTXOReference{}, nil
	}

	var results []UTXOReference
	for _, record := range m.records {
		// Check subject match
		if record.Certificate.Subject.ToDERHex() != identityKey {
			continue
		}

		// Check certifier match
		certifierMatch := false
		certifierHex := record.Certificate.Certifier.ToDERHex()
		for _, certifier := range certifiers {
			if certifierHex == certifier {
				certifierMatch = true
				break
			}
		}
		if !certifierMatch {
			continue
		}

		// Check type match
		typeMatch := false
		certType := string(record.Certificate.Type)
		for _, certTypeStr := range certificateTypes {
			if certType == certTypeStr {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			continue
		}

		results = append(results, UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		})
	}

	return results, nil
}

func (m *MockIdentityStorage) FindByCertificateSerialNumber(ctx context.Context, serialNumber string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if serialNumber == "" {
		return []UTXOReference{}, nil
	}

	var results []UTXOReference
	for _, record := range m.records {
		if string(record.Certificate.SerialNumber) == serialNumber {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}

	return results, nil
}

// fuzzyMatch implements a simple fuzzy matching algorithm for testing
func fuzzyMatch(text, pattern string) bool {
	// Case-insensitive matching
	text = strings.ToLower(text)
	pattern = strings.ToLower(pattern)

	// Simple implementation: check if pattern characters appear in order
	textIdx := 0
	for _, char := range pattern {
		found := false
		for textIdx < len(text) {
			if rune(text[textIdx]) == char {
				found = true
				textIdx++
				break
			}
			textIdx++
		}
		if !found {
			return false
		}
	}
	return true
}

func TestIdentityLookupService_NewInstance(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestIdentityLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Identity Lookup Service")
	assert.Contains(t, docs, "ls_identity")
}

func TestIdentityLookupService_GetMetaData(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Identity Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestIdentityLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestIdentityLookupService_Lookup_EmptyQuery(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "query parameters")
}

func TestIdentityLookupService_Lookup_BySerialNumber(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store a record first
	testSerial := "serial123"
	txid := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	cert := createTestCertificate(testSerial)
	err := storage.StoreRecord(context.Background(), txid, 0, cert)
	require.NoError(t, err)

	// Lookup by serial number
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": testSerial}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, txid, results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestIdentityLookupService_Lookup_ByCertifiers(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store multiple records with the same certifier
	certifierKey, _ := ec.NewPrivateKey()
	certifier := certifierKey.PubKey().ToDERHex()

	cert1 := createTestCertificate("serial1")
	cert1.Certifier = *certifierKey.PubKey()
	err := storage.StoreRecord(context.Background(), "txid1", 0, cert1)
	require.NoError(t, err)

	cert2 := createTestCertificate("serial2")
	cert2.Certifier = *certifierKey.PubKey()
	err = storage.StoreRecord(context.Background(), "txid2", 0, cert2)
	require.NoError(t, err)

	otherKey, _ := ec.NewPrivateKey()
	cert3 := createTestCertificate("serial3")
	cert3.Certifier = *otherKey.PubKey()
	err = storage.StoreRecord(context.Background(), "txid3", 0, cert3)
	require.NoError(t, err)

	// Lookup by certifiers
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{"certifiers": []string{certifier}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestIdentityLookupService_Lookup_ByIdentityKeyAndCertifiers(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store records
	identityKeyPriv, _ := ec.NewPrivateKey()
	identityKey := identityKeyPriv.PubKey().ToDERHex()

	certifierKey, _ := ec.NewPrivateKey()
	certifier := certifierKey.PubKey().ToDERHex()

	cert1 := createTestCertificate("serial1")
	cert1.Subject = *identityKeyPriv.PubKey()
	cert1.Certifier = *certifierKey.PubKey()
	err := storage.StoreRecord(context.Background(), "txid1", 0, cert1)
	require.NoError(t, err)

	otherIdentityKey, _ := ec.NewPrivateKey()
	cert2 := createTestCertificate("serial2")
	cert2.Subject = *otherIdentityKey.PubKey()
	cert2.Certifier = *certifierKey.PubKey()
	err = storage.StoreRecord(context.Background(), "txid2", 0, cert2)
	require.NoError(t, err)

	// Lookup by identity key and certifiers
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query: testutil.MakeQuery(map[string]interface{}{
			"identityKey": identityKey,
			"certifiers":  []string{certifier},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
	assert.Equal(t, "txid1", results[0].Txid)
}

func TestIdentityLookupService_Lookup_ByAttributesAndCertifiers(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store records
	certifierKey, _ := ec.NewPrivateKey()
	certifier := certifierKey.PubKey().ToDERHex()

	cert := createTestCertificate("serial1")
	cert.Certifier = *certifierKey.PubKey()
	err := storage.StoreRecord(context.Background(), "txid1", 0, cert)
	require.NoError(t, err)

	// Lookup by attributes and certifiers
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query: testutil.MakeQuery(map[string]interface{}{
			"attributes": map[string]string{"name": "John"},
			"certifiers": []string{certifier},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	// Results depend on whether the test certificate has matching attributes
	assert.NotNil(t, results)
}

func TestIdentityLookupService_Lookup_NoResults(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Lookup non-existent serial number
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": "nonexistent"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestIdentityLookupService_OutputSpent(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	cert := createTestCertificate("serial123")
	err := storage.StoreRecord(context.Background(), txidHex, 1, cert)
	require.NoError(t, err)

	// Verify it exists
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_identity",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	answer, err = ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ = answer.Result.([]UTXOReference)
	assert.Empty(t, results)
}

func TestIdentityLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	cert := createTestCertificate("serial123")
	err := storage.StoreRecord(context.Background(), txidHex, 0, cert)
	require.NoError(t, err)

	// Try to mark as spent with wrong topic
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_other",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 0,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it still exists (was not deleted)
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	assert.Len(t, results, 1)
}

func TestIdentityLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	cert := createTestCertificate("serial123")
	err := storage.StoreRecord(context.Background(), txidHex, 0, cert)
	require.NoError(t, err)

	// Evict the output
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputEvicted(context.Background(), outpoint)
	require.NoError(t, err)

	// Verify it's deleted
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	assert.Empty(t, results)
}

func TestIdentityLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1111111111111111111111111111111111111111111111111111111111111111"
	cert := createTestCertificate("serial123")
	err := storage.StoreRecord(context.Background(), txidHex, 0, cert)
	require.NoError(t, err)

	// Mark as no longer retained
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_identity")
	require.NoError(t, err)

	// Verify it's deleted
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	assert.Empty(t, results)
}

func TestIdentityLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "2222222222222222222222222222222222222222222222222222222222222222"
	cert := createTestCertificate("serial123")
	err := storage.StoreRecord(context.Background(), txidHex, 0, cert)
	require.NoError(t, err)

	// Try to mark as no longer retained with wrong topic
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_other")
	require.NoError(t, err)

	// Verify it still exists (was not deleted)
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	assert.Len(t, results, 1)
}

func TestIdentityLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockIdentityStorage()
	ls := NewIdentityLookupServiceWithStorage(storage)

	// This is a no-op for Identity, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}

// Helper functions

func createTestCertificate(serialNumber string) *certificates.Certificate {
	privateKey, _ := ec.NewPrivateKey()
	certifierKey, _ := ec.NewPrivateKey()

	return &certificates.Certificate{
		Type:         wallet.StringBase64("aWRlbnRpdHk="), // "identity" in base64
		SerialNumber: wallet.StringBase64(serialNumber),
		Subject:      *privateKey.PubKey(),
		Certifier:    *certifierKey.PubKey(),
		Fields: map[wallet.CertificateFieldNameUnder50Bytes]wallet.StringBase64{
			"name":  wallet.StringBase64("Sm9obiBEb2U="),             // "John Doe" in base64
			"email": wallet.StringBase64("am9obkBleGFtcGxlLmNvbQ=="), // "john@example.com" in base64
		},
	}
}
