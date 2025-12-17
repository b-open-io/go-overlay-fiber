package any

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAnyStorage is a mock implementation of AnyStorageEngine for testing
type MockAnyStorage struct {
	records      map[string]AnyRecord
	storeError   error
	spendError   error
	deleteError  error
	findError    error
	findAllError error
}

func NewMockAnyStorage() *MockAnyStorage {
	return &MockAnyStorage{
		records: make(map[string]AnyRecord),
	}
}

func (m *MockAnyStorage) makeKey(txid string, outputIndex int) string {
	return txid + ":" + strconv.Itoa(outputIndex)
}

func (m *MockAnyStorage) StoreRecord(txid string, outputIndex int) error {
	if m.storeError != nil {
		return m.storeError
	}
	key := m.makeKey(txid, outputIndex)
	m.records[key] = AnyRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		CreatedAt:   time.Now(),
	}
	return nil
}

func (m *MockAnyStorage) SpendRecord(txid string, outputIndex int, spendingTxid string) error {
	if m.spendError != nil {
		return m.spendError
	}
	key := m.makeKey(txid, outputIndex)
	if record, ok := m.records[key]; ok {
		record.SpendingTxid = &spendingTxid
		m.records[key] = record
	}
	return nil
}

func (m *MockAnyStorage) DeleteRecord(txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockAnyStorage) FindByTxid(txid string) (*UTXOReference, error) {
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

func (m *MockAnyStorage) FindAll(limit int, skip int, startDate *time.Time, endDate *time.Time, sortOrder string) ([]UTXOReference, error) {
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

// makeQuery creates a json.RawMessage from a map
func makeQuery(m map[string]interface{}) json.RawMessage {
	data, _ := json.Marshal(m)
	return data
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
		Query:   makeQuery(map[string]interface{}{"txid": "test"}),
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
		Query:   makeQuery(map[string]interface{}{"txid": "txid123"}),
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

func TestAnyLookupService_Lookup_FindAll(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)

	// Store multiple records
	_ = storage.StoreRecord("txid1", 0)
	_ = storage.StoreRecord("txid2", 0)

	// Lookup without txid (findAll)
	question := &lookup.LookupQuestion{
		Service: "ls_anytx",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerType("output-list"), answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestAnyLookupService_Lookup_InvalidLimit(t *testing.T) {
	storage := NewMockAnyStorage()
	ls := NewAnyLookupServiceWithStorage(storage)

	question := &lookup.LookupQuestion{
		Service: "ls_anytx",
		Query:   makeQuery(map[string]interface{}{"limit": -1}),
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
		Query:   makeQuery(map[string]interface{}{"skip": -1}),
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
		Query:   makeQuery(map[string]interface{}{"startDate": "invalid-date"}),
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
		Query:   makeQuery(map[string]interface{}{"endDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "endDate")
}
