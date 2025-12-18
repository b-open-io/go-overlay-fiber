package basketmap

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockBasketMapStorage is a mock implementation of BasketMapStorageEngine for testing
type MockBasketMapStorage struct {
	records     map[string]BasketMapRecord
	storeError  error
	deleteError error
	findError   error
}

func NewMockBasketMapStorage() *MockBasketMapStorage {
	return &MockBasketMapStorage{
		records: make(map[string]BasketMapRecord),
	}
}

func (m *MockBasketMapStorage) makeKey(txid string, outputIndex int) string {
	return txid + ":" + string(rune(outputIndex))
}

func (m *MockBasketMapStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration BasketMapRegistration) error {
	if m.storeError != nil {
		return m.storeError
	}
	key := m.makeKey(txid, outputIndex)
	m.records[key] = BasketMapRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		Registration: registration,
		CreatedAt:    time.Now(),
	}
	return nil
}

func (m *MockBasketMapStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockBasketMapStorage) FindByID(ctx context.Context, basketID string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.Registration.BasketID == basketID && testutil.Contains(registryOperators, record.Registration.RegistryOperator) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

func (m *MockBasketMapStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		// Simple fuzzy matching: case-insensitive substring match
		if fuzzyMatch(record.Registration.Name, name) && testutil.Contains(registryOperators, record.Registration.RegistryOperator) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

// Helper function for fuzzy matching (simplified version)
func fuzzyMatch(haystack, needle string) bool {
	haystack = strings.ToLower(haystack)
	needle = strings.ToLower(needle)

	// Simple fuzzy match: all characters of needle must appear in order in haystack
	needleIdx := 0
	for i := 0; i < len(haystack) && needleIdx < len(needle); i++ {
		if haystack[i] == needle[needleIdx] {
			needleIdx++
		}
	}
	return needleIdx == len(needle)
}

func TestBasketMapLookupService_NewInstance(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestBasketMapLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "BasketMap Lookup Service")
	assert.Contains(t, docs, "ls_basketmap")
}

func TestBasketMapLookupService_GetMetaData(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "BasketMap Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestBasketMapLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "invalid query")
}

func TestBasketMapLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   testutil.MakeQuery(map[string]interface{}{"basketID": "test", "registryOperators": []string{"operator1"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.NoError(t, err)
	assert.NotNil(t, answer)
	// BasketMapLookupService doesn't check service name, just validates query parameters
}

func TestBasketMapLookupService_Lookup_EmptyQuery(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_basketmap",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "query parameters")
}

func TestBasketMapLookupService_Lookup_ByBasketID(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// Store a record first
	basketID := "test-basket"
	registryOp := "02operator1"
	registration := BasketMapRegistration{
		BasketID:         basketID,
		Name:             "Test Basket",
		RegistryOperator: registryOp,
	}
	err := storage.StoreRecord(context.Background(), "txid123", 0, registration)
	require.NoError(t, err)

	// Lookup by basket ID
	question := &lookup.LookupQuestion{
		Service: "ls_basketmap",
		Query:   testutil.MakeQuery(map[string]interface{}{"basketID": basketID, "registryOperators": []string{registryOp}}),
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

func TestBasketMapLookupService_Lookup_ByName(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// Store multiple records with similar names
	registryOp := "02operator1"
	registration1 := BasketMapRegistration{
		BasketID:         "basket-1",
		Name:             "Payment Basket",
		RegistryOperator: registryOp,
	}
	err := storage.StoreRecord(context.Background(), "txid1", 0, registration1)
	require.NoError(t, err)

	registration2 := BasketMapRegistration{
		BasketID:         "basket-2",
		Name:             "Payments",
		RegistryOperator: registryOp,
	}
	err = storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)

	registration3 := BasketMapRegistration{
		BasketID:         "basket-3",
		Name:             "Other",
		RegistryOperator: registryOp,
	}
	err = storage.StoreRecord(context.Background(), "txid3", 0, registration3)
	require.NoError(t, err)

	// Lookup by name (fuzzy search)
	question := &lookup.LookupQuestion{
		Service: "ls_basketmap",
		Query:   testutil.MakeQuery(map[string]interface{}{"name": "pay", "registryOperators": []string{registryOp}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestBasketMapLookupService_Lookup_NoResults(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// Lookup non-existent basket
	question := &lookup.LookupQuestion{
		Service: "ls_basketmap",
		Query:   testutil.MakeQuery(map[string]interface{}{"basketID": "nonexistent", "registryOperators": []string{"operator1"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestBasketMapLookupService_OutputSpent(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
	}
	err := storage.StoreRecord(context.Background(), txidHex, 1, registration)
	require.NoError(t, err)

	// Verify it exists
	results, err := storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_basketmap",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBasketMapLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
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
	results, err := storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestBasketMapLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
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
	results, err := storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBasketMapLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
	}
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Mark as no longer retained
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_basketmap")
	require.NoError(t, err)

	// Verify it's deleted
	results, err := storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBasketMapLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
	}
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Try with wrong topic
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_other")
	require.NoError(t, err)

	// Verify it still exists (was not deleted)
	results, err := storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestBasketMapLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockBasketMapStorage()
	ls := NewBasketMapLookupServiceWithStorage(storage)

	// This is a no-op for BasketMap, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000003"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
