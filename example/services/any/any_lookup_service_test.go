package any

import (
	"context"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAnyStorage is a mock implementation of AnyStorageEngine for testing.
// It embeds MockStorageBase for common functionality and adds Any-specific lookup logic.
type MockAnyStorage struct {
	*testutil.MockStorageBase[AnyRecord]
}

func NewMockAnyStorage() *MockAnyStorage {
	return &MockAnyStorage{
		MockStorageBase: testutil.NewMockStorageBase[AnyRecord](),
	}
}

func (m *MockAnyStorage) StoreRecord(txid string, outputIndex int) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Store(key, AnyRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		CreatedAt:   time.Now(),
	})
}

func (m *MockAnyStorage) SpendRecord(txid string, outputIndex int, spendingTxid string) error {
	key := testutil.MakeKey(txid, outputIndex)
	if record, ok := m.Get(key); ok {
		record.SpendingTxid = &spendingTxid
		return m.Store(key, record)
	}
	return nil
}

func (m *MockAnyStorage) DeleteRecord(txid string, outputIndex int) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Delete(key)
}

func (m *MockAnyStorage) FindByTxid(txid string) (*UTXOReference, error) {
	if m.LookupError != nil {
		return nil, m.LookupError
	}
	if txid == "" {
		return nil, nil
	}

	// Use Filter from base to find matching record
	matches := m.Filter(func(record AnyRecord) bool {
		return record.Txid == txid
	})

	if len(matches) > 0 {
		return &UTXOReference{
			Txid:        matches[0].Txid,
			OutputIndex: matches[0].OutputIndex,
		}, nil
	}
	return nil, nil
}

func (m *MockAnyStorage) FindAll(limit int, skip int, startDate *time.Time, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
	if m.LookupError != nil {
		return nil, m.LookupError
	}

	// Use Filter from base to find matching records
	matches := m.Filter(func(record AnyRecord) bool {
		if startDate != nil && record.CreatedAt.Before(*startDate) {
			return false
		}
		if endDate != nil && record.CreatedAt.After(*endDate) {
			return false
		}
		return true
	})

	// Convert to UTXOReference slice
	var results []UTXOReference
	for _, record := range matches {
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

func TestAnyLookupService_NewInstance(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestAnyLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Any Lookup")
	assert.Contains(t, docs, "Literally any transaction")
}

func TestAnyLookupService_GetMetaData(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Any Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestAnyLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestAnyLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   testutil.MakeQuery(map[string]interface{}{"txid": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "lookup service not supported")
}

func TestAnyLookupService_Lookup_ByTxid(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)

	// Store a record first
	err := storage.StoreRecord("txid123", 0)
	require.NoError(t, err)

	// Lookup by txid
	question := &lookup.LookupQuestion{
		Service: "ls_anytx",
		Query:   testutil.MakeQuery(map[string]interface{}{"txid": "txid123"}),
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

func TestAnyLookupService_Lookup_FindAll(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)

	// Store multiple records
	_ = storage.StoreRecord("txid1", 0)
	_ = storage.StoreRecord("txid2", 0)

	// Lookup without txid (findAll)
	question := &lookup.LookupQuestion{
		Service: "ls_anytx",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestAnyLookupService_Lookup_InvalidLimit(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_anytx",
		Query:   testutil.MakeQuery(map[string]interface{}{"limit": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "limit")
}

func TestAnyLookupService_Lookup_InvalidSkip(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_anytx",
		Query:   testutil.MakeQuery(map[string]interface{}{"skip": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "skip")
}

func TestAnyLookupService_Lookup_InvalidStartDate(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_anytx",
		Query:   testutil.MakeQuery(map[string]interface{}{"startDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "startDate")
}

func TestAnyLookupService_Lookup_InvalidEndDate(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_anytx",
		Query:   testutil.MakeQuery(map[string]interface{}{"endDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "endDate")
}
