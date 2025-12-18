package uhrp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockUHRPStorage is a mock implementation of UHRPStorageEngine for testing
type MockUHRPStorage struct {
	records     map[string]UHRPRecord
	storeError  error
	deleteError error
	lookupError error
}

func NewMockUHRPStorage() *MockUHRPStorage {
	return &MockUHRPStorage{
		records: make(map[string]UHRPRecord),
	}
}

func (m *MockUHRPStorage) makeKey(txid string, outputIndex int) string {
	return txid + ":" + strconv.Itoa(outputIndex)
}

func (m *MockUHRPStorage) StoreRecord(uhrpUrl string, txid string, outputIndex int, hostIdentityKey string, hostedFileLocation string, expiryTime uint64, fileSize uint64) error {
	if m.storeError != nil {
		return m.storeError
	}
	key := m.makeKey(txid, outputIndex)
	m.records[key] = UHRPRecord{
		Txid:               txid,
		OutputIndex:        outputIndex,
		UHRPUrl:            uhrpUrl,
		HostIdentityKey:    hostIdentityKey,
		HostedFileLocation: hostedFileLocation,
		ExpiryTime:         expiryTime,
		FileSize:           fileSize,
	}
	return nil
}

func (m *MockUHRPStorage) DeleteRecord(txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockUHRPStorage) Lookup(query *UHRPQuery) ([]UTXOReference, error) {
	if m.lookupError != nil {
		return nil, m.lookupError
	}

	// Handle outpoint query (exact match by txid.outputIndex)
	if query.Outpoint != "" {
		parts := strings.Split(query.Outpoint, ".")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid outpoint format, expected txid.outputIndex")
		}
		txid := parts[0]
		outputIndex, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid output index in outpoint: %w", err)
		}

		key := m.makeKey(txid, outputIndex)
		if record, ok := m.records[key]; ok {
			return []UTXOReference{{Txid: record.Txid, OutputIndex: record.OutputIndex}}, nil
		}
		return []UTXOReference{}, nil
	}

	// Build results based on query parameters
	var results []UTXOReference
	for _, record := range m.records {
		matches := true

		if query.UHRPUrl != "" && record.UHRPUrl != query.UHRPUrl {
			matches = false
		}
		if query.ExpiryTime != 0 && record.ExpiryTime != query.ExpiryTime {
			matches = false
		}
		if query.HostIdentityKey != "" && record.HostIdentityKey != query.HostIdentityKey {
			matches = false
		}
		if query.FileSize != 0 && record.FileSize != query.FileSize {
			matches = false
		}

		if matches {
			results = append(results, UTXOReference{Txid: record.Txid, OutputIndex: record.OutputIndex})
		}
	}

	// Must have at least one filter criterion
	if query.UHRPUrl == "" && query.ExpiryTime == 0 && query.HostIdentityKey == "" && query.FileSize == 0 {
		return nil, fmt.Errorf("lookup must specify either outpoint, or at least one of (uhrpUrl, expiryTime, hostIdentityKey, fileSize)")
	}

	return results, nil
}

func TestUHRPLookupService_NewInstance(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestUHRPLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Universal Hash Resolution Protocol")
	assert.Contains(t, docs, "ls_uhrp")
}

func TestUHRPLookupService_GetMetaData(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "UHRP Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestUHRPLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestUHRPLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   testutil.MakeQuery(map[string]interface{}{"uhrpUrl": "uhrp://test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "unsupported lookup service")
}

func TestUHRPLookupService_Lookup_EmptyQuery(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "lookup must specify")
}

func TestUHRPLookupService_Lookup_ByUHRPUrl(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// Store a record first
	testURL := "uhrp://abc123def456"
	err := storage.StoreRecord(testURL, "txid123", 0, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
	require.NoError(t, err)

	// Lookup by UHRP URL
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   testutil.MakeQuery(map[string]interface{}{"uhrpUrl": testURL}),
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

func TestUHRPLookupService_Lookup_ByOutpoint(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// Store a record first
	err := storage.StoreRecord("uhrp://test", "txid456", 2, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
	require.NoError(t, err)

	// Lookup by outpoint
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
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

func TestUHRPLookupService_Lookup_ByHostIdentityKey(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// Store multiple records with the same host identity key
	hostKey := "02abcdef1234567890"
	err := storage.StoreRecord("uhrp://url1", "txid1", 0, hostKey, "https://example.com/file1.dat", 1735689600, 1024)
	require.NoError(t, err)
	err = storage.StoreRecord("uhrp://url2", "txid2", 0, hostKey, "https://example.com/file2.dat", 1735689600, 2048)
	require.NoError(t, err)
	err = storage.StoreRecord("uhrp://url3", "txid3", 0, "different_key", "https://example.com/file3.dat", 1735689600, 512)
	require.NoError(t, err)

	// Lookup by host identity key
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   testutil.MakeQuery(map[string]interface{}{"hostIdentityKey": hostKey}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestUHRPLookupService_Lookup_NoResults(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// Lookup non-existent URL
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   testutil.MakeQuery(map[string]interface{}{"uhrpUrl": "uhrp://nonexistent"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestUHRPLookupService_OutputSpent(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := storage.StoreRecord("uhrp://test", txidHex, 1, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
	require.NoError(t, err)

	// Verify it exists
	results, err := storage.Lookup(&UHRPQuery{Outpoint: txidHex + ".1"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_uhrp",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = storage.Lookup(&UHRPQuery{Outpoint: txidHex + ".1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUHRPLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := storage.StoreRecord("uhrp://test", txidHex, 0, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
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
	results, err := storage.Lookup(&UHRPQuery{Outpoint: txidHex + ".0"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestUHRPLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := storage.StoreRecord("uhrp://test", txidHex, 0, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
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
	results, err := storage.Lookup(&UHRPQuery{Outpoint: txidHex + ".0"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUHRPLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// This is a no-op for UHRP, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err := ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_uhrp")
	require.NoError(t, err)
}

func TestUHRPLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockUHRPStorage()
	ls := NewUHRPLookupServiceWithStorage(storage)

	// This is a no-op for UHRP, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
