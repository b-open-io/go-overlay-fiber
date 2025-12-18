package desktopintegrity

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

// MockDesktopIntegrityStorage is a mock implementation of DesktopIntegrityStorageEngine for testing
type MockDesktopIntegrityStorage struct {
	records      map[string]DesktopIntegrityRecord
	storeError   error
	deleteError  error
	findError    error
	findAllError error
}

func NewMockDesktopIntegrityStorage() *MockDesktopIntegrityStorage {
	return &MockDesktopIntegrityStorage{
		records: make(map[string]DesktopIntegrityRecord),
	}
}

func (m *MockDesktopIntegrityStorage) makeKey(txid string, outputIndex int) string {
	return txid + ":" + string(rune(outputIndex))
}

func (m *MockDesktopIntegrityStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, fileHash string, offChainValues []byte) error {
	if m.storeError != nil {
		return m.storeError
	}
	key := m.makeKey(txid, outputIndex)
	m.records[key] = DesktopIntegrityRecord{
		Txid:           txid,
		OutputIndex:    outputIndex,
		FileHash:       fileHash,
		OffChainValues: offChainValues,
		CreatedAt:      time.Now(),
	}
	return nil
}

func (m *MockDesktopIntegrityStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockDesktopIntegrityStorage) FindByFileHash(ctx context.Context, fileHash string, limit int, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if fileHash == "" {
		return []UTXOReference{}, nil
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.FileHash == fileHash {
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

func (m *MockDesktopIntegrityStorage) FindByTxid(ctx context.Context, txid string, limit int, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if txid == "" {
		return []UTXOReference{}, nil
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.Txid == txid {
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

func (m *MockDesktopIntegrityStorage) FindAll(ctx context.Context, limit int, skip int, startDate *time.Time, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
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

func TestDesktopIntegrityLookupService_NewInstance(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestDesktopIntegrityLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "DesktopIntegrity Lookup Service")
	assert.Contains(t, docs, "ls_desktopintegrity")
	assert.Contains(t, docs, "fileHash")
}

func TestDesktopIntegrityLookupService_GetMetaData(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "DesktopIntegrity Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestDesktopIntegrityLookupService_Lookup_ByFileHash(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store a record first
	testFileHash := "abc123def456"
	err := storage.StoreRecord(context.Background(), "txid123", 0, testFileHash, nil)
	require.NoError(t, err)

	// Lookup by fileHash
	question := &lookup.LookupQuestion{
		Service: "ls_desktopintegrity",
		Query:   testutil.MakeQuery(map[string]interface{}{"fileHash": testFileHash}),
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

func TestDesktopIntegrityLookupService_Lookup_ByTxid(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store a record first
	testTxid := "txid456"
	err := storage.StoreRecord(context.Background(), testTxid, 1, "filehash123", nil)
	require.NoError(t, err)

	// Lookup by txid
	question := &lookup.LookupQuestion{
		Service: "ls_desktopintegrity",
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

func TestDesktopIntegrityLookupService_Lookup_EmptyQuery(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store a record
	err := storage.StoreRecord(context.Background(), "txid789", 0, "hash789", nil)
	require.NoError(t, err)

	// Lookup with empty query should return all via FindAll
	question := &lookup.LookupQuestion{
		Service: "ls_desktopintegrity",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

func TestDesktopIntegrityLookupService_Lookup_WithLimit(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store multiple records with the same fileHash
	testFileHash := "commonhash"
	_ = storage.StoreRecord(context.Background(), "txid1", 0, testFileHash, nil)
	_ = storage.StoreRecord(context.Background(), "txid2", 0, testFileHash, nil)
	_ = storage.StoreRecord(context.Background(), "txid3", 0, testFileHash, nil)

	// Lookup with limit
	question := &lookup.LookupQuestion{
		Service: "ls_desktopintegrity",
		Query:   testutil.MakeQuery(map[string]interface{}{"fileHash": testFileHash, "limit": 2}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.LessOrEqual(t, len(results), 2)
}

func TestDesktopIntegrityLookupService_Lookup_InvalidLimit(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_desktopintegrity",
		Query:   testutil.MakeQuery(map[string]interface{}{"limit": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "limit")
}

func TestDesktopIntegrityLookupService_Lookup_InvalidSkip(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_desktopintegrity",
		Query:   testutil.MakeQuery(map[string]interface{}{"skip": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "skip")
}

func TestDesktopIntegrityLookupService_Lookup_InvalidStartDate(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_desktopintegrity",
		Query:   testutil.MakeQuery(map[string]interface{}{"startDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "startDate")
}

func TestDesktopIntegrityLookupService_Lookup_InvalidEndDate(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_desktopintegrity",
		Query:   testutil.MakeQuery(map[string]interface{}{"endDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "endDate")
}

func TestDesktopIntegrityLookupService_OutputSpent(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := storage.StoreRecord(context.Background(), txidHex, 1, "testhash", nil)
	require.NoError(t, err)

	// Verify it exists
	results, err := storage.FindByFileHash(context.Background(), "testhash", 10, 0, "desc")
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_desktopintegrity",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = storage.FindByFileHash(context.Background(), "testhash", 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDesktopIntegrityLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := storage.StoreRecord(context.Background(), txidHex, 0, "testhash2", nil)
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
	results, err := storage.FindByFileHash(context.Background(), "testhash2", 10, 0, "desc")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestDesktopIntegrityLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := storage.StoreRecord(context.Background(), txidHex, 0, "evicthash", nil)
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
	results, err := storage.FindByFileHash(context.Background(), "evicthash", 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDesktopIntegrityLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// Store a record first
	txidHex := "1111111111111111111111111111111111111111111111111111111111111111"
	err := storage.StoreRecord(context.Background(), txidHex, 0, "retainhash", nil)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := testutil.MakeHashFromHex(txidHex)
	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_desktopintegrity")
	require.NoError(t, err)

	// Verify it's deleted
	results, err := storage.FindByFileHash(context.Background(), "retainhash", 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDesktopIntegrityLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockDesktopIntegrityStorage()
	ls := NewDesktopIntegrityLookupServiceWithStorage(storage)

	// This should just return nil without doing anything
	txidHash := testutil.MakeHashFromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 100, 0)
	assert.NoError(t, err)
}
