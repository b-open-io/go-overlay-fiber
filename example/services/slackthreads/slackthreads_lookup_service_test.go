package slackthreads

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

// MockSlackThreadsStorage is a mock implementation of SlackThreadsStorageEngine for testing.
// It embeds MockStorageBase for common functionality and adds SlackThreads-specific lookup logic.
type MockSlackThreadsStorage struct {
	*testutil.MockStorageBase[SlackThreadRecord]
}

func NewMockSlackThreadsStorage() *MockSlackThreadsStorage {
	return &MockSlackThreadsStorage{
		MockStorageBase: testutil.NewMockStorageBase[SlackThreadRecord](),
	}
}

func (m *MockSlackThreadsStorage) StoreRecord(txid string, outputIndex int, threadHash string) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Store(key, SlackThreadRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		ThreadHash:  threadHash,
		CreatedAt:   time.Now(),
	})
}

func (m *MockSlackThreadsStorage) DeleteRecord(txid string, outputIndex int) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Delete(key)
}

func (m *MockSlackThreadsStorage) FindByThreadHash(threadHash string, limit int, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.LookupError != nil {
		return nil, m.LookupError
	}
	if threadHash == "" {
		return []UTXOReference{}, nil
	}

	// Use Filter from base to find matching records
	matches := m.Filter(func(record SlackThreadRecord) bool {
		return record.ThreadHash == threadHash
	})

	// Apply skip and limit
	if skip >= len(matches) {
		return []UTXOReference{}, nil
	}
	matches = matches[skip:]
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}

	// Convert to UTXOReference slice
	results := make([]UTXOReference, len(matches))
	for i, record := range matches {
		results[i] = UTXOReference{Txid: record.Txid, OutputIndex: record.OutputIndex}
	}
	return results, nil
}

func (m *MockSlackThreadsStorage) FindByTxid(txid string, limit int, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.LookupError != nil {
		return nil, m.LookupError
	}
	if txid == "" {
		return []UTXOReference{}, nil
	}

	// Use Filter from base to find matching records
	matches := m.Filter(func(record SlackThreadRecord) bool {
		return record.Txid == txid
	})

	// Apply skip and limit
	if skip >= len(matches) {
		return []UTXOReference{}, nil
	}
	matches = matches[skip:]
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}

	// Convert to UTXOReference slice
	results := make([]UTXOReference, len(matches))
	for i, record := range matches {
		results[i] = UTXOReference{Txid: record.Txid, OutputIndex: record.OutputIndex}
	}
	return results, nil
}

func (m *MockSlackThreadsStorage) FindAll(limit int, skip int, startDate *time.Time, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
	if m.LookupError != nil {
		return nil, m.LookupError
	}

	// Use Filter from base to find matching records
	matches := m.Filter(func(record SlackThreadRecord) bool {
		// Apply date filters
		if startDate != nil && record.CreatedAt.Before(*startDate) {
			return false
		}
		if endDate != nil && record.CreatedAt.After(*endDate) {
			return false
		}
		return true
	})

	// Apply skip and limit
	if skip >= len(matches) {
		return []UTXOReference{}, nil
	}
	matches = matches[skip:]
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}

	// Convert to UTXOReference slice
	results := make([]UTXOReference, len(matches))
	for i, record := range matches {
		results[i] = UTXOReference{Txid: record.Txid, OutputIndex: record.OutputIndex}
	}
	return results, nil
}

func TestSlackThreadsLookupService_NewInstance(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestSlackThreadsLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "SlackThread Lookup Service")
	assert.Contains(t, docs, "ls_slackthread")
}

func TestSlackThreadsLookupService_GetMetaData(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "SlackThread Lookup Service", meta.Name)
	assert.Equal(t, "Find threads on-chain.", meta.Description)
}

func TestSlackThreadsLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestSlackThreadsLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   testutil.MakeQuery(map[string]interface{}{"threadHash": "abcd"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "lookup service not supported")
}

func TestSlackThreadsLookupService_Lookup_ByThreadHash(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store a record first
	testThreadHash := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := storage.StoreRecord("txid123", 0, testThreadHash)
	require.NoError(t, err)

	// Lookup by thread hash
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"threadHash": testThreadHash}),
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

func TestSlackThreadsLookupService_Lookup_ByTxid(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store a record first
	testTxid := "abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	testThreadHash := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	err := storage.StoreRecord(testTxid, 1, testThreadHash)
	require.NoError(t, err)

	// Lookup by txid
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"txid": testTxid}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, testTxid, results[0].Txid)
	assert.Equal(t, 1, results[0].OutputIndex)
}

func TestSlackThreadsLookupService_Lookup_FindAll(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store multiple records
	testHash1 := "1111111111111111111111111111111111111111111111111111111111111111"
	testHash2 := "2222222222222222222222222222222222222222222222222222222222222222"
	err := storage.StoreRecord("txid1", 0, testHash1)
	require.NoError(t, err)
	err = storage.StoreRecord("txid2", 1, testHash2)
	require.NoError(t, err)

	// Find all
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 2)
}

func TestSlackThreadsLookupService_Lookup_WithLimitAndSkip(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store multiple records
	for i := 0; i < 5; i++ {
		threadHash := make([]byte, 32)
		for j := range threadHash {
			threadHash[j] = byte(i)
		}
		err := storage.StoreRecord("txid"+string(rune('0'+i)), i, string(threadHash))
		require.NoError(t, err)
	}

	// Test with limit
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"limit": 2}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)

	// Test with skip
	question2 := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"limit": 10, "skip": 3}),
	}
	answer2, err := ls.Lookup(context.Background(), question2)
	require.NoError(t, err)
	results2, ok := answer2.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results2, 2)
}

func TestSlackThreadsLookupService_Lookup_InvalidLimit(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"limit": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "limit")
}

func TestSlackThreadsLookupService_Lookup_InvalidSkip(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"skip": -5}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "skip")
}

func TestSlackThreadsLookupService_Lookup_WithDateRange(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store records
	testHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err := storage.StoreRecord("txid1", 0, testHash)
	require.NoError(t, err)

	// Query with date range
	startDate := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	endDate := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query: testutil.MakeQuery(map[string]interface{}{
			"startDate": startDate,
			"endDate":   endDate,
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

func TestSlackThreadsLookupService_Lookup_InvalidDateFormat(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"startDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "startDate")
}

func TestSlackThreadsLookupService_Lookup_SortOrder(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store records with slight delay to ensure different timestamps
	hash1 := "1111111111111111111111111111111111111111111111111111111111111111"
	err := storage.StoreRecord("txid1", 0, hash1)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	hash2 := "2222222222222222222222222222222222222222222222222222222222222222"
	err = storage.StoreRecord("txid2", 0, hash2)
	require.NoError(t, err)

	// Test descending order (newest first) - mock doesn't actually sort, just test query works
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"sortOrder": "desc"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 2)

	// Test ascending order (oldest first)
	question2 := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   testutil.MakeQuery(map[string]interface{}{"sortOrder": "asc"}),
	}
	answer2, err := ls.Lookup(context.Background(), question2)
	require.NoError(t, err)
	results2, ok := answer2.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results2, 2)
}

func TestSlackThreadsLookupService_OutputSpent(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	testHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err := storage.StoreRecord(txidHex, 1, testHash)
	require.NoError(t, err)

	// Verify it exists
	results, err := storage.FindByThreadHash(testHash, 10, 0, "desc")
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_slackthread",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = storage.FindByThreadHash(testHash, 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSlackThreadsLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	testHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	err := storage.StoreRecord(txidHex, 0, testHash)
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
	results, err := storage.FindByThreadHash(testHash, 10, 0, "desc")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSlackThreadsLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	testHash := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	err := storage.StoreRecord(txidHex, 0, testHash)
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
	results, err := storage.FindByThreadHash(testHash, 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSlackThreadsLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// This should just return nil without doing anything
	txidHash := testutil.MakeHashFromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err := ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_slackthread")
	assert.NoError(t, err)
}

func TestSlackThreadsLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockSlackThreadsStorage()
	ls := NewSlackThreadsLookupServiceWithStorage(storage)

	// This should just return nil without doing anything
	txidHash := testutil.MakeHashFromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 100, 0)
	assert.NoError(t, err)
}
