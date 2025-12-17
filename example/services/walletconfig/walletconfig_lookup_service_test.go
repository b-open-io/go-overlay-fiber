package walletconfig

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// getTestMongoDB returns a MongoDB database for testing, or nil if MongoDB is not available
func getTestMongoDB(t *testing.T) *mongo.Database {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Use a short timeout for connection attempts
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(mongoURI).SetConnectTimeout(2 * time.Second).SetServerSelectionTimeout(2 * time.Second)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
		return nil
	}

	// Ping to verify connection with timeout
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		t.Skipf("MongoDB not available: %v", err)
		return nil
	}

	// Use a test-specific database
	return client.Database("walletconfig_test_" + t.Name())
}

func cleanupTestDB(t *testing.T, db *mongo.Database) {
	if db != nil {
		_ = db.Drop(context.Background())
	}
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
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestWalletConfigLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "WalletConfig Lookup Service")
	assert.Contains(t, docs, "ls_walletconfig")
}

func TestWalletConfigLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "WalletConfig Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestWalletConfigLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

	// WalletConfig doesn't check for nil question, so this will panic
	// We test that the panic occurs as expected
	assert.Panics(t, func() {
		_, _ = ls.Lookup(context.Background(), nil)
	})
}

func TestWalletConfigLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"configID": "test", "registryOperators": []string{"operator1"}}),
	}

	// WalletConfig doesn't validate the service field, so this should work fine
	// with a valid query structure
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	// Should return empty results since no records exist
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestWalletConfigLookupService_Lookup_MissingRegistryOperators(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)
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
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), "txid123", 0, registration)
	require.NoError(t, err)

	// Lookup by configID
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"configID": "config123", "registryOperators": []string{"operator1"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid123", results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestWalletConfigLookupService_Lookup_ByName(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), "txid456", 0, registration)
	require.NoError(t, err)

	// Lookup by name (fuzzy search)
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"name": "Test", "registryOperators": []string{"operator2"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid456", results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestWalletConfigLookupService_Lookup_ByWAB(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), "txid789", 0, registration)
	require.NoError(t, err)

	// Lookup by WAB
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"wab": "https://wab.test.com", "registryOperators": []string{"operator3"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid789", results[0].Txid)
}

func TestWalletConfigLookupService_Lookup_ListAll(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, registration1)
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
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)

	// List all configs from operator4
	question := &lookup.LookupQuestion{
		Service: "ls_walletconfig",
		Query:   makeQuery(map[string]interface{}{"registryOperators": []string{"operator4"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestWalletConfigLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), txidHex, 1, registration)
	require.NoError(t, err)

	// Verify it exists
	results, err := ls.storage.FindByConfigID(context.Background(), "config-spent", []string{"operator5"})
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
	results, err = ls.storage.FindByConfigID(context.Background(), "config-spent", []string{"operator5"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestWalletConfigLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, registration)
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
	results, err := ls.storage.FindByConfigID(context.Background(), "config-persist", []string{"operator6"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestWalletConfigLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, registration)
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
	results, err := ls.storage.FindByConfigID(context.Background(), "config-evict", []string{"operator7"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestWalletConfigLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, registration)
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
	results, err := ls.storage.FindByConfigID(context.Background(), "config-retain", []string{"operator8"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestWalletConfigLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

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
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, registration)
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
	results, err := ls.storage.FindByConfigID(context.Background(), "config-retain2", []string{"operator9"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestWalletConfigLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewWalletConfigLookupService(db)

	// OutputBlockHeightUpdated should not do anything for WalletConfig
	txidHash := makeHashFromHex("3333333333333333333333333333333333333333333333333333333333333333")
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 1)
	require.NoError(t, err)
}
