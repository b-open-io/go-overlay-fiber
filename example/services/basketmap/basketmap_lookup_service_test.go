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

// MockBasketMapStorage is a mock implementation of BasketMapStorageEngine for testing.
// It embeds MockStorageBase for common functionality and adds BasketMap-specific lookup logic.
type MockBasketMapStorage struct {
	*testutil.MockStorageBase[BasketMapRecord]
	FindError error
}

func NewMockBasketMapStorage() *MockBasketMapStorage {
	return &MockBasketMapStorage{
		MockStorageBase: testutil.NewMockStorageBase[BasketMapRecord](),
	}
}

func (m *MockBasketMapStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration BasketMapRegistration) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Store(key, BasketMapRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		Registration: registration,
		CreatedAt:    time.Now(),
	})
}

func (m *MockBasketMapStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Delete(key)
}

func (m *MockBasketMapStorage) FindByID(ctx context.Context, basketID string, registryOperators []string) ([]UTXOReference, error) {
	if m.FindError != nil {
		return nil, m.FindError
	}

	// Use Filter from base to find matching records
	matches := m.Filter(func(record BasketMapRecord) bool {
		return record.Registration.BasketID == basketID &&
			testutil.Contains(registryOperators, record.Registration.RegistryOperator)
	})

	// Convert to UTXOReference slice
	results := make([]UTXOReference, len(matches))
	for i, record := range matches {
		results[i] = UTXOReference{Txid: record.Txid, OutputIndex: record.OutputIndex}
	}
	return results, nil
}

func (m *MockBasketMapStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
	if m.FindError != nil {
		return nil, m.FindError
	}

	// Use Filter from base to find matching records
	matches := m.Filter(func(record BasketMapRecord) bool {
		return fuzzyMatch(record.Registration.Name, name) &&
			testutil.Contains(registryOperators, record.Registration.RegistryOperator)
	})

	// Convert to UTXOReference slice
	results := make([]UTXOReference, len(matches))
	for i, record := range matches {
		results[i] = UTXOReference{Txid: record.Txid, OutputIndex: record.OutputIndex}
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

// TestTableDrivenQueryValidation tests various query scenarios using table-driven tests
func TestTableDrivenQueryValidation(t *testing.T) {
	storage := NewMockBasketMapStorage()
	service := NewBasketMapLookupServiceWithStorage(storage)

	// Store some test data
	registryOp := "02operator1"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: registryOp,
	}
	err := storage.StoreRecord(context.Background(), "txid_test", 0, registration)
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty query",
			query:       map[string]interface{}{},
			expectError: true,
			errorMsg:    "query parameters",
		},
		{
			name:        "valid basketID query",
			query:       map[string]interface{}{"basketID": "test-basket", "registryOperators": []string{registryOp}},
			expectError: false,
		},
		{
			name:        "valid name query",
			query:       map[string]interface{}{"name": "Test", "registryOperators": []string{registryOp}},
			expectError: false,
		},
		{
			name:        "basketID without registryOperators",
			query:       map[string]interface{}{"basketID": "test-basket"},
			expectError: true,
			errorMsg:    "query parameters",
		},
		{
			name:        "name without registryOperators",
			query:       map[string]interface{}{"name": "Test"},
			expectError: true,
			errorMsg:    "query parameters",
		},
		{
			name:        "empty registryOperators array",
			query:       map[string]interface{}{"basketID": "test-basket", "registryOperators": []string{}},
			expectError: true,
			errorMsg:    "query parameters",
		},
		{
			name:        "non-existent basketID",
			query:       map[string]interface{}{"basketID": "nonexistent", "registryOperators": []string{registryOp}},
			expectError: false,
		},
		{
			name:        "non-existent name",
			query:       map[string]interface{}{"name": "Nonexistent", "registryOperators": []string{registryOp}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryJSON := testutil.MakeQuery(tt.query)

			question := &lookup.LookupQuestion{
				Service: "ls_basketmap",
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
		name       string
		setup      func(storage *MockBasketMapStorage)
		basketID   string
		basketName string
		operators  []string
		wantCount  int
		wantError  bool
		searchBy   string // "id" or "name"
	}{
		{
			name: "find by basketID - single match",
			setup: func(storage *MockBasketMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "First Basket",
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, BasketMapRegistration{
					BasketID:         "basket2",
					Name:             "Second Basket",
					RegistryOperator: "operator1",
				})
			},
			basketID:  "basket1",
			operators: []string{"operator1"},
			wantCount: 1,
			searchBy:  "id",
		},
		{
			name: "find by basketID - multiple operators",
			setup: func(storage *MockBasketMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "First Basket",
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "First Basket Alt",
					RegistryOperator: "operator2",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "First Basket Third",
					RegistryOperator: "operator3",
				})
			},
			basketID:  "basket1",
			operators: []string{"operator1", "operator2"},
			wantCount: 2,
			searchBy:  "id",
		},
		{
			name: "find by name - fuzzy match",
			setup: func(storage *MockBasketMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "Payment Basket",
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid2", 0, BasketMapRegistration{
					BasketID:         "basket2",
					Name:             "Payments",
					RegistryOperator: "operator1",
				})
				_ = storage.StoreRecord(context.Background(), "txid3", 0, BasketMapRegistration{
					BasketID:         "basket3",
					Name:             "Other",
					RegistryOperator: "operator1",
				})
			},
			basketName: "pay",
			operators:  []string{"operator1"},
			wantCount:  2,
			searchBy:   "name",
		},
		{
			name: "find by name - case insensitive",
			setup: func(storage *MockBasketMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "Test Basket",
					RegistryOperator: "operator1",
				})
			},
			basketName: "TEST",
			operators:  []string{"operator1"},
			wantCount:  1,
			searchBy:   "name",
		},
		{
			name: "no matches - wrong operator",
			setup: func(storage *MockBasketMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "Test Basket",
					RegistryOperator: "operator1",
				})
			},
			basketID:  "basket1",
			operators: []string{"operator2"},
			wantCount: 0,
			searchBy:  "id",
		},
		{
			name: "no matches - nonexistent basketID",
			setup: func(storage *MockBasketMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "Test Basket",
					RegistryOperator: "operator1",
				})
			},
			basketID:  "nonexistent",
			operators: []string{"operator1"},
			wantCount: 0,
			searchBy:  "id",
		},
		{
			name: "no matches - nonexistent name",
			setup: func(storage *MockBasketMapStorage) {
				_ = storage.StoreRecord(context.Background(), "txid1", 0, BasketMapRegistration{
					BasketID:         "basket1",
					Name:             "Test Basket",
					RegistryOperator: "operator1",
				})
			},
			basketName: "nonexistent",
			operators:  []string{"operator1"},
			wantCount:  0,
			searchBy:   "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockBasketMapStorage()
			tt.setup(storage)

			var results []UTXOReference
			var err error

			if tt.searchBy == "id" {
				results, err = storage.FindByID(context.Background(), tt.basketID, tt.operators)
			} else {
				results, err = storage.FindByName(context.Background(), tt.basketName, tt.operators)
			}

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tt.wantCount)
			}
		})
	}
}
