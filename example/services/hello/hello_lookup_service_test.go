package hello

import (
	"context"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHelloWorldStorage is a mock implementation of HelloWorldStorageEngine for testing
type MockHelloWorldStorage struct {
	records      map[string]HelloWorldRecord
	storeError   error
	deleteError  error
	findError    error
	findAllError error
}

func NewMockHelloWorldStorage() *MockHelloWorldStorage {
	return &MockHelloWorldStorage{
		records: make(map[string]HelloWorldRecord),
	}
}

func (m *MockHelloWorldStorage) makeKey(txid string, outputIndex int) string {
	return txid + ":" + string(rune(outputIndex))
}

func (m *MockHelloWorldStorage) StoreRecord(txid string, outputIndex int, message string) error {
	if m.storeError != nil {
		return m.storeError
	}
	key := m.makeKey(txid, outputIndex)
	m.records[key] = HelloWorldRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		Message:     message,
		CreatedAt:   time.Now(),
	}
	return nil
}

func (m *MockHelloWorldStorage) DeleteRecord(txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockHelloWorldStorage) FindByMessage(message string, limit int, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if message == "" {
		return []UTXOReference{}, nil
	}

	var results []UTXOReference
	for _, record := range m.records {
		// Simple substring match for testing (real implementation uses full-text search)
		if contains(record.Message, message) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}

	// Apply skip and limit
	if skip >= len(results) {
		return []UTXOReference{}, nil
	}
	results = results[skip:]
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (m *MockHelloWorldStorage) FindAll(limit int, skip int, startDate *time.Time, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
	if m.findAllError != nil {
		return nil, m.findAllError
	}

	var results []UTXOReference
	for _, record := range m.records {
		// Apply date filters
		if startDate != nil && record.CreatedAt.Before(*startDate) {
			continue
		}
		if endDate != nil && record.CreatedAt.After(*endDate) {
			continue
		}
		results = append(results, UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		})
	}

	// Apply skip and limit
	if skip >= len(results) {
		return []UTXOReference{}, nil
	}
	results = results[skip:]
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Helper function for simple substring matching
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHelloWorldLookupService_NewInstance(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestHelloWorldLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "HelloWorld Lookup Service")
	assert.Contains(t, docs, "ls_helloworld")
}

func TestHelloWorldLookupService_GetMetaData(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "HelloWorld Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestHelloWorldLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestHelloWorldLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   testutil.MakeQuery(map[string]interface{}{"message": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "lookup service")
}

func TestHelloWorldLookupService_Lookup_ByMessage(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	// Store a record first
	testMessage := "Hello Overlay"
	err := storage.StoreRecord("txid123", 0, testMessage)
	require.NoError(t, err)

	// Lookup by message
	question := &lookup.LookupQuestion{
		Service: "ls_helloworld",
		Query:   testutil.MakeQuery(map[string]interface{}{"message": testMessage}),
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

func TestHelloWorldLookupService_Lookup_EmptyMessage(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	// Store a record
	err := storage.StoreRecord("txid123", 0, "Hello World")
	require.NoError(t, err)

	// Lookup with empty message should return all via FindAll
	question := &lookup.LookupQuestion{
		Service: "ls_helloworld",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)
}

func TestHelloWorldLookupService_Lookup_WithLimit(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	// Store multiple records
	_ = storage.StoreRecord("txid1", 0, "Hello One")
	_ = storage.StoreRecord("txid2", 0, "Hello Two")
	_ = storage.StoreRecord("txid3", 0, "Hello Three")

	// Lookup with limit
	question := &lookup.LookupQuestion{
		Service: "ls_helloworld",
		Query:   testutil.MakeQuery(map[string]interface{}{"message": "Hello", "limit": 2}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.LessOrEqual(t, len(results), 2)
}

func TestHelloWorldLookupService_Lookup_InvalidLimit(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_helloworld",
		Query:   testutil.MakeQuery(map[string]interface{}{"limit": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "limit")
}

func TestHelloWorldLookupService_Lookup_InvalidSkip(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_helloworld",
		Query:   testutil.MakeQuery(map[string]interface{}{"skip": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "skip")
}

func TestHelloWorldLookupService_Lookup_InvalidStartDate(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_helloworld",
		Query:   testutil.MakeQuery(map[string]interface{}{"startDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "startDate")
}

func TestHelloWorldLookupService_Lookup_InvalidEndDate(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_helloworld",
		Query:   testutil.MakeQuery(map[string]interface{}{"endDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "endDate")
}

func TestHelloWorldLookupService_OutputSpent(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := storage.StoreRecord(txidHex, 1, "Hello World")
	require.NoError(t, err)

	// Verify it exists
	results, err := storage.FindByMessage("Hello", 10, 0, "desc")
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_helloworld",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = storage.FindByMessage("Hello", 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestHelloWorldLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := storage.StoreRecord(txidHex, 0, "Test Message")
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

	// Verify it still exists (was not deleted because wrong topic)
	results, err := storage.FindByMessage("Test", 10, 0, "desc")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestHelloWorldLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := storage.StoreRecord(txidHex, 0, "Evict Me")
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
	results, err := storage.FindByMessage("Evict", 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestHelloWorldLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	// This should just return nil without doing anything
	txidHash := testutil.MakeHashFromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err := ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_helloworld")
	assert.NoError(t, err)
}

func TestHelloWorldLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockHelloWorldStorage()
	ls := NewHelloWorldLookupServiceWithStorage(storage)

	// This should just return nil without doing anything
	txidHash := testutil.MakeHashFromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 100, 0)
	assert.NoError(t, err)
}
