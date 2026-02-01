package fractionalize

import (
	"context"
	"fmt"
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

// MockFractionalizeStorage is a mock implementation of FractionalizeStorageEngine for testing.
// It embeds MockStorageBase for common functionality and adds Fractionalize-specific lookup logic.
type MockFractionalizeStorage struct {
	*testutil.MockStorageBase[FractionalizeRecord]

	// Error injection for service-specific operations
	findError    error
	findAllError error
}

func NewMockFractionalizeStorage() *MockFractionalizeStorage {
	return &MockFractionalizeStorage{
		MockStorageBase: testutil.NewMockStorageBase[FractionalizeRecord](),
	}
}

func (m *MockFractionalizeStorage) StoreRecord(ctx context.Context, txid string, outputIndex int) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Store(key, FractionalizeRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		CreatedAt:   time.Now(),
	})
}

func (m *MockFractionalizeStorage) SpendRecord(ctx context.Context, txid string, outputIndex int, spendingTxid string) error {
	key := testutil.MakeKey(txid, outputIndex)
	record, ok := m.Get(key)
	if !ok {
		return nil
	}
	record.SpendingTxid = spendingTxid
	return m.Store(key, record)
}

func (m *MockFractionalizeStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Delete(key)
}

func (m *MockFractionalizeStorage) FindByTxid(ctx context.Context, txid string) (*UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	if txid == "" {
		return nil, nil
	}

	matches := m.Filter(func(record FractionalizeRecord) bool {
		return record.Txid == txid
	})

	if len(matches) == 0 {
		return nil, nil
	}

	return &UTXOReference{
		Txid:        matches[0].Txid,
		OutputIndex: matches[0].OutputIndex,
	}, nil
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

	// Use Filter from base to find matching records
	matches := m.Filter(func(record FractionalizeRecord) bool {
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
		results[i] = UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		}
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
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)
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
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

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
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

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
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

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

// TestTableDrivenQueryValidation tests various query scenarios using table-driven tests
func TestTableDrivenQueryValidation(t *testing.T) {
	storage := NewMockFractionalizeStorage()
	service := NewFractionalizeLookupServiceWithStorage(storage)

	// Store some test data
	err := storage.StoreRecord(context.Background(), "txid_test", 0)
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid txid query",
			query:       map[string]interface{}{"txid": "txid_test"},
			expectError: false,
		},
		{
			name:        "valid empty query - find all",
			query:       map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "valid limit query",
			query:       map[string]interface{}{"limit": 10},
			expectError: false,
		},
		{
			name:        "valid skip query",
			query:       map[string]interface{}{"skip": 5},
			expectError: false,
		},
		{
			name:        "valid sortOrder query",
			query:       map[string]interface{}{"sortOrder": "asc"},
			expectError: false,
		},
		{
			name:        "valid date range query",
			query:       map[string]interface{}{"startDate": "2025-01-01T00:00:00Z", "endDate": "2025-12-31T23:59:59Z"},
			expectError: false,
		},
		{
			name:        "negative limit",
			query:       map[string]interface{}{"limit": -1},
			expectError: true,
			errorMsg:    "limit must be a non-negative number",
		},
		{
			name:        "negative skip",
			query:       map[string]interface{}{"skip": -1},
			expectError: true,
			errorMsg:    "skip must be a non-negative number",
		},
		{
			name:        "invalid startDate format",
			query:       map[string]interface{}{"startDate": "invalid-date"},
			expectError: true,
			errorMsg:    "invalid startDate format",
		},
		{
			name:        "invalid endDate format",
			query:       map[string]interface{}{"endDate": "not-a-date"},
			expectError: true,
			errorMsg:    "invalid endDate format",
		},
		{
			name:        "non-existent txid",
			query:       map[string]interface{}{"txid": "nonexistent_txid"},
			expectError: false,
		},
		{
			name:        "combined filters",
			query:       map[string]interface{}{"limit": 10, "skip": 2, "sortOrder": "desc"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON := testutil.MakeQuery(tt.query)

			question := &lookup.LookupQuestion{
				Service: "ls_fractionalize",
				Query:   queryJSON,
			}

			answer, err := service.Lookup(context.Background(), question)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, answer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, answer)
			}
		})
	}
}

// TestTableDrivenStorageOperations tests storage operations with table-driven tests
func TestTableDrivenStorageOperations(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(storage *MockFractionalizeStorage)
		query     func() (*UTXOReference, []UTXOReference, error)
		wantCount int
		wantError bool
	}{
		{
			name: "find by txid - single match",
			setup: func(storage *MockFractionalizeStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0)
				_ = storage.StoreRecord(context.Background(), "txid2", 0)
			},
			query: func() (*UTXOReference, []UTXOReference, error) {
				storage := NewMockFractionalizeStorage()
				_ = storage.StoreRecord(context.Background(), "txid1", 0)
				_ = storage.StoreRecord(context.Background(), "txid2", 0)
				result, err := storage.FindByTxid(context.Background(), "txid1")
				if result != nil {
					return result, nil, err
				}
				return nil, nil, err
			},
			wantCount: 1,
		},
		{
			name: "find all - multiple matches",
			setup: func(storage *MockFractionalizeStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0)
				_ = storage.StoreRecord(context.Background(), "txid2", 0)
				_ = storage.StoreRecord(context.Background(), "txid3", 0)
			},
			query: func() (*UTXOReference, []UTXOReference, error) {
				storage := NewMockFractionalizeStorage()
				_ = storage.StoreRecord(context.Background(), "txid1", 0)
				_ = storage.StoreRecord(context.Background(), "txid2", 0)
				_ = storage.StoreRecord(context.Background(), "txid3", 0)
				results, err := storage.FindAll(context.Background(), 50, 0, nil, nil, "desc")
				return nil, results, err
			},
			wantCount: 3,
		},
		{
			name: "find all with limit",
			setup: func(storage *MockFractionalizeStorage) {
				for i := 0; i < 10; i++ {
					_ = storage.StoreRecord(context.Background(), fmt.Sprintf("txid%d", i), 0)
				}
			},
			query: func() (*UTXOReference, []UTXOReference, error) {
				storage := NewMockFractionalizeStorage()
				for i := 0; i < 10; i++ {
					_ = storage.StoreRecord(context.Background(), fmt.Sprintf("txid%d", i), 0)
				}
				results, err := storage.FindAll(context.Background(), 5, 0, nil, nil, "desc")
				return nil, results, err
			},
			wantCount: 5,
		},
		{
			name: "find all with skip",
			setup: func(storage *MockFractionalizeStorage) {
				for i := 0; i < 10; i++ {
					_ = storage.StoreRecord(context.Background(), fmt.Sprintf("txid%d", i), 0)
				}
			},
			query: func() (*UTXOReference, []UTXOReference, error) {
				storage := NewMockFractionalizeStorage()
				for i := 0; i < 10; i++ {
					_ = storage.StoreRecord(context.Background(), fmt.Sprintf("txid%d", i), 0)
				}
				results, err := storage.FindAll(context.Background(), 100, 5, nil, nil, "desc")
				return nil, results, err
			},
			wantCount: 5,
		},
		{
			name: "find all with date range",
			setup: func(storage *MockFractionalizeStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0)
			},
			query: func() (*UTXOReference, []UTXOReference, error) {
				storage := NewMockFractionalizeStorage()
				_ = storage.StoreRecord(context.Background(), "txid1", 0)
				now := time.Now()
				yesterday := now.Add(-24 * time.Hour)
				tomorrow := now.Add(24 * time.Hour)
				results, err := storage.FindAll(context.Background(), 50, 0, &yesterday, &tomorrow, "desc")
				return nil, results, err
			},
			wantCount: 1,
		},
		{
			name: "find by txid - no match",
			setup: func(storage *MockFractionalizeStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0)
			},
			query: func() (*UTXOReference, []UTXOReference, error) {
				storage := NewMockFractionalizeStorage()
				_ = storage.StoreRecord(context.Background(), "txid1", 0)
				result, err := storage.FindByTxid(context.Background(), "nonexistent")
				return result, nil, err
			},
			wantCount: 0,
		},
		{
			name:  "empty storage",
			setup: func(storage *MockFractionalizeStorage) {},
			query: func() (*UTXOReference, []UTXOReference, error) {
				storage := NewMockFractionalizeStorage()
				results, err := storage.FindAll(context.Background(), 50, 0, nil, nil, "desc")
				return nil, results, err
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			singleResult, listResults, err := tt.query()

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if singleResult != nil {
					assert.Equal(t, 1, tt.wantCount)
				} else if listResults != nil {
					assert.Len(t, listResults, tt.wantCount)
				} else {
					assert.Equal(t, 0, tt.wantCount)
				}
			}
		})
	}
}
