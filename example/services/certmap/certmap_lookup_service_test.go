package certmap

import (
	"context"
	"regexp"
	"testing"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCertMapStorage is a mock implementation of CertMapStorageEngine for testing.
// It embeds MockStorageBase for common functionality and adds CertMap-specific lookup logic.
type MockCertMapStorage struct {
	*testutil.MockStorageBase[CertMapRecord]
}

func NewMockCertMapStorage() *MockCertMapStorage {
	return &MockCertMapStorage{
		MockStorageBase: testutil.NewMockStorageBase[CertMapRecord](),
	}
}

func (m *MockCertMapStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration *CertMapRegistration) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Store(key, CertMapRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		Registration: registration,
	})
}

func (m *MockCertMapStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Delete(key)
}

func (m *MockCertMapStorage) FindByType(ctx context.Context, certType string, registryOperators []string) ([]UTXOReference, error) {
	if m.LookupError != nil {
		return nil, m.LookupError
	}

	// Use Filter from base to find matching records
	matches := m.Filter(func(record CertMapRecord) bool {
		if record.Registration.Type != certType {
			return false
		}
		// Check if registry operator matches
		for _, op := range registryOperators {
			if record.Registration.RegistryOperator == op {
				return true
			}
		}
		return false
	})

	// Convert to UTXOReference slice
	results := make([]UTXOReference, len(matches))
	for i, record := range matches {
		results[i] = UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		}
	}
	return results, nil
}

func (m *MockCertMapStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
	if m.LookupError != nil {
		return nil, m.LookupError
	}

	// Create fuzzy pattern like the real implementation
	escaped := regexp.QuoteMeta(name)
	fuzzyPattern := ""
	for i, char := range escaped {
		if i > 0 {
			fuzzyPattern += ".*"
		}
		fuzzyPattern += string(char)
	}
	fuzzyRegex, err := regexp.Compile("(?i)" + fuzzyPattern)
	if err != nil {
		return nil, err
	}

	// Use Filter from base to find matching records
	matches := m.Filter(func(record CertMapRecord) bool {
		// Check if registry operator matches
		operatorMatches := false
		for _, op := range registryOperators {
			if record.Registration.RegistryOperator == op {
				operatorMatches = true
				break
			}
		}
		if !operatorMatches {
			return false
		}

		// Check if name matches fuzzy pattern
		return fuzzyRegex.MatchString(record.Registration.Name)
	})

	// Convert to UTXOReference slice
	results := make([]UTXOReference, len(matches))
	for i, record := range matches {
		results[i] = UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		}
	}
	return results, nil
}

func TestCertMapLookupService_NewInstance(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestCertMapLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "CertMap Lookup Service")
	assert.Contains(t, docs, "ls_certmap")
}

func TestCertMapLookupService_GetMetaData(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "CertMap Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestCertMapLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestCertMapLookupService_Lookup_EmptyQuery(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "registryOperators")
}

func TestCertMapLookupService_Lookup_MissingRegistryOperators(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query:   testutil.MakeQuery(map[string]interface{}{"type": "test-type"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "registryOperators")
}

func TestCertMapLookupService_Lookup_MissingTypeAndName(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query:   testutil.MakeQuery(map[string]interface{}{"registryOperators": []string{"operator1"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "type or name")
}

func TestCertMapLookupService_Lookup_ByType(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Store a record first
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), "txid123", 0, registration)
	require.NoError(t, err)

	// Lookup by type
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid123", results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestCertMapLookupService_Lookup_ByName(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Store records with different names
	registration1 := &CertMapRegistration{
		Type:             "type1",
		Name:             "Test Certificate",
		IconURL:          "https://example.com/icon1.png",
		Description:      "Test description 1",
		DocumentationURL: "https://example.com/docs1",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), "txid1", 0, registration1)
	require.NoError(t, err)

	registration2 := &CertMapRegistration{
		Type:             "type2",
		Name:             "Another Certificate",
		IconURL:          "https://example.com/icon2.png",
		Description:      "Test description 2",
		DocumentationURL: "https://example.com/docs2",
		CertFields:       map[string]interface{}{"field2": "value2"},
		RegistryOperator: "operator1",
	}
	err = storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)

	// Lookup by name (fuzzy)
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"name":              "Certificate",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestCertMapLookupService_Lookup_FilterByRegistryOperator(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Store records with different registry operators
	registration1 := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test 1",
		IconURL:          "https://example.com/icon1.png",
		Description:      "Test description 1",
		DocumentationURL: "https://example.com/docs1",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), "txid1", 0, registration1)
	require.NoError(t, err)

	registration2 := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test 2",
		IconURL:          "https://example.com/icon2.png",
		Description:      "Test description 2",
		DocumentationURL: "https://example.com/docs2",
		CertFields:       map[string]interface{}{"field2": "value2"},
		RegistryOperator: "operator2",
	}
	err = storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)

	// Lookup by type, filtering by registry operator
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid1", results[0].Txid)
}

func TestCertMapLookupService_Lookup_NoResults(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Lookup non-existent type
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"type":              "nonexistent",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestCertMapLookupService_OutputSpent(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), txidHex, 1, registration)
	require.NoError(t, err)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_certmap",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted - attempt to lookup by type
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestCertMapLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
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
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

func TestCertMapLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
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
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestCertMapLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_certmap")
	require.NoError(t, err)

	// Verify it's deleted
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestCertMapLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Call with wrong topic
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
		Service: "ls_certmap",
		Query: testutil.MakeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

func TestCertMapLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockCertMapStorage()
	ls := NewCertMapLookupServiceWithStorage(storage)

	// This is a no-op for CertMap, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000003"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
