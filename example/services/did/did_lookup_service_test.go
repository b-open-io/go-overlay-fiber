package did

import (
	"context"
	"fmt"
	"strings"
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

// MockDIDStorage is a mock implementation of DIDStorageEngine for testing
type MockDIDStorage struct {
	records     map[string]DIDRecord
	storeError  error
	deleteError error
	findError   error
}

func NewMockDIDStorage() *MockDIDStorage {
	return &MockDIDStorage{
		records: make(map[string]DIDRecord),
	}
}

func (m *MockDIDStorage) makeKey(txid string, outputIndex int) string {
	return fmt.Sprintf("%s:%d", txid, outputIndex)
}

func (m *MockDIDStorage) StoreRecord(txid string, outputIndex int, serialNumber string) error {
	if m.storeError != nil {
		return m.storeError
	}
	key := m.makeKey(txid, outputIndex)
	m.records[key] = DIDRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		SerialNumber: serialNumber,
		CreatedAt:    time.Now(),
	}
	return nil
}

func (m *MockDIDStorage) DeleteRecord(txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockDIDStorage) FindByCertificateSerialNumber(serialNumber string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.SerialNumber == serialNumber {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}

	return results, nil
}

func (m *MockDIDStorage) FindByOutpoint(outpoint string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	// Parse txid and outputIndex from the outpoint string (format: "txid.outputIndex")
	parts := strings.Split(outpoint, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid outpoint format, expected txid.outputIndex")
	}

	txid := parts[0]
	var outputIndex int
	_, err := fmt.Sscanf(parts[1], "%d", &outputIndex)
	if err != nil {
		return nil, fmt.Errorf("invalid output index in outpoint: %w", err)
	}

	key := m.makeKey(txid, outputIndex)
	if record, exists := m.records[key]; exists {
		return []UTXOReference{
			{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			},
		}, nil
	}

	return []UTXOReference{}, nil
}

func TestDIDLookupService_NewInstance(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestDIDLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "DID Lookup Service")
	assert.Contains(t, docs, "ls_did")
}

func TestDIDLookupService_GetMetaData(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "DID Lookup Service", meta.Name)
	assert.Equal(t, "DID resolution made easy.", meta.Description)
}

func TestDIDLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestDIDLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "unsupported lookup service")
}

func TestDIDLookupService_Lookup_EmptyQuery(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_did",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "no valid query parameters")
}

func TestDIDLookupService_Lookup_BySerialNumber(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Store a record first
	testSerialNumber := "abc123serialnumber"
	err := storage.StoreRecord("txid123", 0, testSerialNumber)
	require.NoError(t, err)

	// Lookup by serial number
	question := &lookup.LookupQuestion{
		Service: "ls_did",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": testSerialNumber}),
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

func TestDIDLookupService_Lookup_ByOutpoint(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Store a record first
	err := storage.StoreRecord("txid456", 2, "serial456")
	require.NoError(t, err)

	// Lookup by outpoint
	question := &lookup.LookupQuestion{
		Service: "ls_did",
		Query:   testutil.MakeQuery(map[string]interface{}{"outpoint": "txid456.2"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid456", results[0].Txid)
	assert.Equal(t, 2, results[0].OutputIndex)
}

func TestDIDLookupService_Lookup_NoResults(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Lookup non-existent serial number
	question := &lookup.LookupQuestion{
		Service: "ls_did",
		Query:   testutil.MakeQuery(map[string]interface{}{"serialNumber": "nonexistent"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestDIDLookupService_OutputAdmittedByTopic(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Create a valid DID transaction
	tx, err := createValidDIDTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_did",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify the record was stored
	txid := tx.TxID().String()
	results, err := storage.FindByOutpoint(txid + ".0")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, txid, results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestDIDLookupService_OutputAdmittedByTopic_WrongTopic(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Create a valid DID transaction
	tx, err := createValidDIDTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_other",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify nothing was stored
	txid := tx.TxID().String()
	results, err := storage.FindByOutpoint(txid + ".0")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDIDLookupService_OutputAdmittedByTopic_InvalidBEEF(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_did",
		OutputIndex: 0,
		AtomicBEEF:  []byte("invalid beef"),
	}

	err := ls.OutputAdmittedByTopic(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
}

func TestDIDLookupService_OutputAdmittedByTopic_InvalidPushDrop(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Create a transaction with non-PushDrop output using the helper
	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: &script.Script{},
	}
	tx, err := createDIDTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_did",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid DID token")
}

func TestDIDLookupService_OutputSpent(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := storage.StoreRecord(txidHex, 1, "serial123")
	require.NoError(t, err)

	// Verify it exists
	results, err := storage.FindByOutpoint(txidHex + ".1")
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_did",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = storage.FindByOutpoint(txidHex + ".1")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDIDLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := storage.StoreRecord(txidHex, 0, "serial456")
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
	results, err := storage.FindByOutpoint(txidHex + ".0")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestDIDLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := storage.StoreRecord(txidHex, 0, "serial789")
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
	results, err := storage.FindByOutpoint(txidHex + ".0")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDIDLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// This is a no-op for DID, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err := ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_did")
	require.NoError(t, err)
}

func TestDIDLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockDIDStorage()
	ls := NewDIDLookupServiceWithStorage(storage)

	// This is a no-op for DID, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
