package fractionalize

import (
	"context"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockFractionalizeStorage is a mock implementation of FractionalizeStorageEngine for testing
type MockFractionalizeStorage struct {
	records      map[string]FractionalizeRecord
	storeError   error
	spendError   error
	deleteError  error
	findError    error
	findAllError error
}

func NewMockFractionalizeStorage() *MockFractionalizeStorage {
	return &MockFractionalizeStorage{
		records: make(map[string]FractionalizeRecord),
	}
}

func (m *MockFractionalizeStorage) makeKey(txid string, outputIndex int) string {
	return txid + ":" + string(rune(outputIndex))
}

func (m *MockFractionalizeStorage) StoreRecord(ctx context.Context, txid string, outputIndex int) error {
	if m.storeError != nil {
		return m.storeError
	}
	key := m.makeKey(txid, outputIndex)
	m.records[key] = FractionalizeRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		CreatedAt:   time.Now(),
	}
	return nil
}

func (m *MockFractionalizeStorage) SpendRecord(ctx context.Context, txid string, outputIndex int, spendingTxid string) error {
	if m.spendError != nil {
		return m.spendError
	}
	key := m.makeKey(txid, outputIndex)
	if record, exists := m.records[key]; exists {
		record.SpendingTxid = spendingTxid
		m.records[key] = record
	}
	return nil
}

func (m *MockFractionalizeStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockFractionalizeStorage) FindByTxid(ctx context.Context, txid string) (*UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if txid == "" {
		return nil, nil
	}

	for _, record := range m.records {
		if record.Txid == txid {
			return &UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			}, nil
		}
	}
	return nil, nil
}

func (m *MockFractionalizeStorage) FindAll(ctx context.Context, limit, skip int, startDate, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
	if m.findAllError != nil {
		return nil, m.findAllError
	}

	if limit <= 0 {
		limit = 50
	}
	if skip < 0 {
		skip = 0
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

func TestFractionalizeLookupService_NewInstance(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestFractionalizeLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Fractionalize Lookup Service")
	assert.Contains(t, docs, "ls_fractionalize")
	assert.Contains(t, docs, "txid")
	assert.Contains(t, docs, "limit")
	assert.Contains(t, docs, "skip")
}

func TestFractionalizeLookupService_GetMetaData(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Fractionalize Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestFractionalizeLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestFractionalizeLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// The Lookup method doesn't validate service name, so this should work but return empty results
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   testutil.MakeQuery(map[string]interface{}{"txid": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerType("output-list"), answer.Type)
}

func TestFractionalizeLookupService_Lookup_InvalidQuery(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   []byte("invalid json"),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "invalid query format")
}

func TestFractionalizeLookupService_Lookup_NegativeLimit(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{"limit": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "limit must be a non-negative number")
}

func TestFractionalizeLookupService_Lookup_NegativeSkip(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{"skip": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "skip must be a non-negative number")
}

func TestFractionalizeLookupService_Lookup_InvalidDateFormat(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{"startDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "invalid startDate format")
}

func TestFractionalizeLookupService_Lookup_ByTxid(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store a record first
	testTxid := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := storage.StoreRecord(context.Background(), testTxid, 0)
	require.NoError(t, err)

	// Lookup by txid
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{"txid": testTxid}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerType("output-list"), answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, testTxid, results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestFractionalizeLookupService_Lookup_ByTxid_NotFound(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Lookup non-existent txid
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{"txid": "nonexistent"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerType("output-list"), answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestFractionalizeLookupService_Lookup_FindAll(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store multiple records
	err := storage.StoreRecord(context.Background(), "txid1", 0)
	require.NoError(t, err)
	err = storage.StoreRecord(context.Background(), "txid2", 0)
	require.NoError(t, err)
	err = storage.StoreRecord(context.Background(), "txid3", 0)
	require.NoError(t, err)

	// Lookup all records
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerType("output-list"), answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 3)
}

func TestFractionalizeLookupService_Lookup_WithLimit(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store multiple records
	for i := 0; i < 10; i++ {
		err := storage.StoreRecord(context.Background(), "txid"+string(rune('0'+i)), 0)
		require.NoError(t, err)
	}

	// Lookup with limit
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{"limit": 5}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 5)
}

func TestFractionalizeLookupService_Lookup_WithSkip(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store multiple records
	for i := 0; i < 10; i++ {
		err := storage.StoreRecord(context.Background(), "txid"+string(rune('0'+i)), 0)
		require.NoError(t, err)
	}

	// Lookup with skip
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{"skip": 5, "limit": 100}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 5)
}

func TestFractionalizeLookupService_Lookup_WithDateRange(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store a record
	err := storage.StoreRecord(context.Background(), "txid1", 0)
	require.NoError(t, err)

	// Lookup with date range
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query: testutil.MakeQuery(map[string]interface{}{
			"startDate": yesterday.Format(time.RFC3339),
			"endDate":   tomorrow.Format(time.RFC3339),
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

func TestFractionalizeLookupService_Lookup_WithSortOrder(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store multiple records with slight delays to ensure different timestamps
	err := storage.StoreRecord(context.Background(), "txid1", 0)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	err = storage.StoreRecord(context.Background(), "txid2", 0)
	require.NoError(t, err)

	// Lookup with ascending sort order
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   testutil.MakeQuery(map[string]interface{}{"sortOrder": "asc"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestFractionalizeLookupService_OutputAdmittedByTopic(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Create a transaction
	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: &script.Script{},
	})
	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Call OutputAdmittedByTopic
	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_fractionalize",
		AtomicBEEF:  beef,
		OutputIndex: 0,
	}
	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify it was stored
	txid := tx.TxID().String()
	result, err := storage.FindByTxid(context.Background(), txid)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, txid, result.Txid)
	assert.Equal(t, 0, result.OutputIndex)
}

func TestFractionalizeLookupService_OutputAdmittedByTopic_WrongTopic(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Create a transaction
	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: &script.Script{},
	})
	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Call OutputAdmittedByTopic with wrong topic
	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_other",
		AtomicBEEF:  beef,
		OutputIndex: 0,
	}
	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify nothing was stored
	txid := tx.TxID().String()
	result, err := storage.FindByTxid(context.Background(), txid)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestFractionalizeLookupService_OutputSpent(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := storage.StoreRecord(context.Background(), txidHex, 1)
	require.NoError(t, err)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)
	spendingTxidHash := testutil.MakeHashFromHex("fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321")
	require.NotNil(t, spendingTxidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_fractionalize",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
		SpendingTxid: spendingTxidHash,
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it was updated (not deleted for fractionalize - it just marks spending txid)
	result, err := storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFractionalizeLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := storage.StoreRecord(context.Background(), txidHex, 0)
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

	// Verify it still exists unchanged
	result, err := storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFractionalizeLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := storage.StoreRecord(context.Background(), txidHex, 0)
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
	result, err := storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestFractionalizeLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1111111111111111111111111111111111111111111111111111111111111111"
	err := storage.StoreRecord(context.Background(), txidHex, 0)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_fractionalize")
	require.NoError(t, err)

	// Verify it's deleted
	result, err := storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestFractionalizeLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "2222222222222222222222222222222222222222222222222222222222222222"
	err := storage.StoreRecord(context.Background(), txidHex, 0)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory with wrong topic
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_other")
	require.NoError(t, err)

	// Verify it still exists
	result, err := storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFractionalizeLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	ls := NewFractionalizeLookupServiceWithStorage(storage)

	// This is a no-op for Fractionalize, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
