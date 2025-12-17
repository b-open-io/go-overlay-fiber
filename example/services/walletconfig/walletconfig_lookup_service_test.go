package walletconfig

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWalletConfigStorage is a mock implementation of WalletConfigStorageEngine for testing
type MockWalletConfigStorage struct {
	records     map[string]*WalletConfigRecord
	storeError  error
	deleteError error
	findError   error
}

func NewMockWalletConfigStorage() *MockWalletConfigStorage {
	return &MockWalletConfigStorage{
		records: make(map[string]*WalletConfigRecord),
	}
}

func (m *MockWalletConfigStorage) makeKey(txid string, outputIndex int) string {
	return txid + ":" + string(rune(outputIndex))
}

func (m *MockWalletConfigStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, registration *WalletConfigRegistration) error {
	if m.storeError != nil {
		return m.storeError
	}

	// Check for duplicates (excluding txid/outputIndex)
	for _, record := range m.records {
		if record.Registration.ConfigID == registration.ConfigID &&
			record.Registration.Name == registration.Name &&
			record.Registration.Icon == registration.Icon &&
			record.Registration.WAB == registration.WAB &&
			record.Registration.Storage == registration.Storage &&
			record.Registration.Messagebox == registration.Messagebox &&
			record.Registration.Legal == registration.Legal &&
			record.Registration.RegistryOperator == registration.RegistryOperator {
			// Duplicate found, don't insert
			return nil
		}
	}

	key := m.makeKey(txid, outputIndex)
	m.records[key] = &WalletConfigRecord{
		Txid:         txid,
		OutputIndex:  outputIndex,
		Registration: registration,
	}
	return nil
}

func (m *MockWalletConfigStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	key := m.makeKey(txid, outputIndex)
	delete(m.records, key)
	return nil
}

func (m *MockWalletConfigStorage) FindByConfigID(ctx context.Context, configID string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.Registration.ConfigID == configID && contains(registryOperators, record.Registration.RegistryOperator) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

func (m *MockWalletConfigStorage) FindByName(ctx context.Context, name string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		// Simple case-insensitive substring match for testing
		if strings.Contains(strings.ToLower(record.Registration.Name), strings.ToLower(name)) &&
			contains(registryOperators, record.Registration.RegistryOperator) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

func (m *MockWalletConfigStorage) FindByWAB(ctx context.Context, wab string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.Registration.WAB == wab && contains(registryOperators, record.Registration.RegistryOperator) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

func (m *MockWalletConfigStorage) FindByStorage(ctx context.Context, storage string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.Registration.Storage == storage && contains(registryOperators, record.Registration.RegistryOperator) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

func (m *MockWalletConfigStorage) FindByMessagebox(ctx context.Context, messagebox string, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		if record.Registration.Messagebox == messagebox && contains(registryOperators, record.Registration.RegistryOperator) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

func (m *MockWalletConfigStorage) ListAll(ctx context.Context, registryOperators []string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}

	var results []UTXOReference
	for _, record := range m.records {
		if contains(registryOperators, record.Registration.RegistryOperator) {
			results = append(results, UTXOReference{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			})
		}
	}
	return results, nil
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// makeQuery creates a json.RawMessage from a map
func makeQuery(m map[string]interface{}) json.RawMessage {
	data, _ := json.Marshal(m)
	return data
}

// makeHashFromHex creates a chainhash.Hash from a hex string, padding if necessary
func makeHashFromHex(hexStr string) *chainhash.Hash {
	// Pad to 64 characters (32 bytes)
	for len(hexStr) < 64 {
		hexStr = "0" + hexStr
	}
	hash, _ := chainhash.NewHashFromHex(hexStr)
	return hash
}

func TestWalletConfigLookupService_NewInstance(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestWalletConfigLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "WalletConfig Lookup Service")
	assert.Contains(t, docs, "ls_walletconfig")
}

func TestWalletConfigLookupService_GetMetaData(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "WalletConfig Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestWalletConfigLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestWalletConfigLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"configID": "test", "registryOperators": []string{"operator1"}}),
	}

	// WalletConfig doesn't validate the service field, so this should work fine
	// with a valid query structure
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	// Should return empty results since no records exist
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestWalletConfigLookupService_Lookup_MissingRegistryOperators(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"configID": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "registryOperators")
}

func TestWalletConfigLookupService_Lookup_ByConfigID(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store a record first
	registration := &WalletConfigRegistration{
		ConfigID:         "config123",
		Name:             "My Wallet",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.example.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator1",
	}
	err := storage.StoreRecord(context.Background(), "txid123", 0, registration)
	require.NoError(t, err)

	// Lookup by configID
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"configID": "config123", "registryOperators": []string{"operator1"}}),
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

func TestWalletConfigLookupService_Lookup_ByName(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store a record first
	registration := &WalletConfigRegistration{
		ConfigID:         "config456",
		Name:             "Test Wallet",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.example.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator2",
	}
	err := storage.StoreRecord(context.Background(), "txid456", 0, registration)
	require.NoError(t, err)

	// Lookup by name (fuzzy search)
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"name": "Test", "registryOperators": []string{"operator2"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid456", results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestWalletConfigLookupService_Lookup_ByWAB(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store a record first
	registration := &WalletConfigRegistration{
		ConfigID:         "config789",
		Name:             "WAB Test Wallet",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.test.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator3",
	}
	err := storage.StoreRecord(context.Background(), "txid789", 0, registration)
	require.NoError(t, err)

	// Lookup by WAB
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"wab": "https://wab.test.com", "registryOperators": []string{"operator3"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid789", results[0].Txid)
}

func TestWalletConfigLookupService_Lookup_ListAll(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store multiple records
	registration1 := &WalletConfigRegistration{
		ConfigID:         "config1",
		Name:             "Wallet 1",
		Icon:             "https://example.com/icon1.png",
		WAB:              "https://wab1.example.com",
		Storage:          "https://storage1.example.com",
		Messagebox:       "https://messagebox1.example.com",
		Legal:            "https://legal1.example.com",
		RegistryOperator: "operator4",
	}
	err := storage.StoreRecord(context.Background(), "txid1", 0, registration1)
	require.NoError(t, err)

	registration2 := &WalletConfigRegistration{
		ConfigID:         "config2",
		Name:             "Wallet 2",
		Icon:             "https://example.com/icon2.png",
		WAB:              "https://wab2.example.com",
		Storage:          "https://storage2.example.com",
		Messagebox:       "https://messagebox2.example.com",
		Legal:            "https://legal2.example.com",
		RegistryOperator: "operator4",
	}
	err = storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)

	// List all configs from operator4
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"registryOperators": []string{"operator4"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestWalletConfigLookupService_OutputSpent(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store a record first
	registration := &WalletConfigRegistration{
		ConfigID:         "config-spent",
		Name:             "Spent Wallet",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.example.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator5",
	}
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := storage.StoreRecord(context.Background(), txidHex, 1, registration)
	require.NoError(t, err)

	// Verify it exists
	results, err := storage.FindByConfigID(context.Background(), "config-spent", []string{"operator5"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_walletconfig",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = storage.FindByConfigID(context.Background(), "config-spent", []string{"operator5"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestWalletConfigLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store a record first
	registration := &WalletConfigRegistration{
		ConfigID:         "config-persist",
		Name:             "Persistent Wallet",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.example.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator6",
	}
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Try to mark as spent with wrong topic
	txidHash := makeHashFromHex(txidHex)
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
	results, err := storage.FindByConfigID(context.Background(), "config-persist", []string{"operator6"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestWalletConfigLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store a record first
	registration := &WalletConfigRegistration{
		ConfigID:         "config-evict",
		Name:             "Evicted Wallet",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.example.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator7",
	}
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Evict the output
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputEvicted(context.Background(), outpoint)
	require.NoError(t, err)

	// Verify it's deleted
	results, err := storage.FindByConfigID(context.Background(), "config-evict", []string{"operator7"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestWalletConfigLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store a record first
	registration := &WalletConfigRegistration{
		ConfigID:         "config-retain",
		Name:             "Retained Wallet",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.example.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator8",
	}
	txidHex := "1111111111111111111111111111111111111111111111111111111111111111"
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_walletconfig")
	require.NoError(t, err)

	// Verify it's deleted
	results, err := storage.FindByConfigID(context.Background(), "config-retain", []string{"operator8"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestWalletConfigLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// Store a record first
	registration := &WalletConfigRegistration{
		ConfigID:         "config-retain2",
		Name:             "Retained Wallet 2",
		Icon:             "https://example.com/icon.png",
		WAB:              "https://wab.example.com",
		Storage:          "https://storage.example.com",
		Messagebox:       "https://messagebox.example.com",
		Legal:            "https://legal.example.com",
		RegistryOperator: "operator9",
	}
	txidHex := "2222222222222222222222222222222222222222222222222222222222222222"
	err := storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory with wrong topic
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_other")
	require.NoError(t, err)

	// Verify it still exists (was not deleted)
	results, err := storage.FindByConfigID(context.Background(), "config-retain2", []string{"operator9"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestWalletConfigLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockWalletConfigStorage()
	ls := NewWalletConfigLookupServiceWithStorage(storage)

	// OutputBlockHeightUpdated should not do anything for WalletConfig
	txidHash := makeHashFromHex("3333333333333333333333333333333333333333333333333333333333333333")
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 1)
	require.NoError(t, err)
}
