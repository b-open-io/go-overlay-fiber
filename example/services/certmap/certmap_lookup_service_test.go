package certmap

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCertMapStorage is a mock implementation of CertMapStorageEngine for testing
type MockCertMapStorage struct {
	records     map[string]*CertMapRecord
	storeError  error
	deleteError error
	findError   error
}

func NewMockCertMapStorage() *MockCertMapStorage {
	return &MockCertMapStorage{
		records: make(map[string]*CertMapRecord),
	}
}

func (m *MockCertMapStorage) makeKey(txid string, outputIndex int) string {
	return txid + ":" + string(rune(outputIndex))
}

func (m *MockCertMapStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration *CertMapRegistration) error {
	if m.storeError != nil {
		return m.storeError
	}
	key := m.makeKey(txid, outputIndex)
	m.records[key] = &CertMapRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		Registration: registration,
	}
	return nil
}

func (m *MockCertMapStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockCertMapStorage) FindByType(ctx context.Context, certType string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.Registration.Type == certType {
			// Check if registry operator matches
			for _, op := range registryOperators {
				if record.Registration.RegistryOperator == op {
					results = append(results, UTXOReference{
						Txid:        record.Txid,
						OutputIndex: record.OutputIndex,
					})
					break
				}
			}
		}
	}
	return results, nil
}

func (m *MockCertMapStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
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

	var results []UTXOReference
	for _, record := range m.records {
		// Check if registry operator matches
		operatorMatches := false
		for _, op := range registryOperators {
			if record.Registration.RegistryOperator == op {
				operatorMatches = true
				break
			}
		}
		if !operatorMatches {
			continue
		}

		// Check if name matches fuzzy pattern
		if fuzzyRegex.MatchString(record.Registration.Name) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

// makeQuery creates a json.RawMessage from a map
func makeQuery(m map[string]interface{}) json.RawMessage {
	data, _ := json.Marshal(m)
	return data
}

// makeHashFromHex creates a chainhash.Hash from a hex string, padding if necessary
func makeHashFromHex(hexStr string) *chainhash.Hash {
	// Pad to 64 characters (32 bytes)
	for len(hexStr) < 64 {
		hexStr = "0" + hexStr
	}
	hash, _ := chainhash.NewHashFromHex(hexStr)
	return hash
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
		Query:   makeQuery(map[string]interface{}{}),
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
		Query:   makeQuery(map[string]interface{}{"type": "test-type"}),
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
		Query:   makeQuery(map[string]interface{}{"registryOperators": []string{"operator1"}}),
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
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerType("output-list"), answer.Type)

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
		Query: makeQuery(map[string]interface{}{
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
		Query: makeQuery(map[string]interface{}{
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
		Query: makeQuery(map[string]interface{}{
			"type":              "nonexistent",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerType("output-list"), answer.Type)

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
	txidHash := makeHashFromHex(txidHex)
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
		Query: makeQuery(map[string]interface{}{
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
	txidHash := makeHashFromHex(txidHex)
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
		Query: makeQuery(map[string]interface{}{
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
	txidHash := makeHashFromHex(txidHex)
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
		Query: makeQuery(map[string]interface{}{
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
	txidHash := makeHashFromHex(txidHex)
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
		Query: makeQuery(map[string]interface{}{
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
	txidHash := makeHashFromHex(txidHex)
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
		Query: makeQuery(map[string]interface{}{
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
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
