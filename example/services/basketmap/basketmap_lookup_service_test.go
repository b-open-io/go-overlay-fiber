package basketmap

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
	return client.Database("basketmap_test_" + t.Name())
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

func TestBasketMapLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestBasketMapLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "BasketMap Lookup Service")
	assert.Contains(t, docs, "ls_basketmap")
}

func TestBasketMapLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "BasketMap Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestBasketMapLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "invalid query")
}

func TestBasketMapLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"basketID": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.NoError(t, err)
	assert.NotNil(t, answer)
	// BasketMapLookupService doesn't check service name, just validates query parameters
}

func TestBasketMapLookupService_Lookup_EmptyQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_basketmap",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "query parameters")
}

func TestBasketMapLookupService_Lookup_ByBasketID(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// Store a record first
	basketID := "test-basket"
	registryOp := "02operator1"
	registration := BasketMapRegistration{
		BasketID:         basketID,
		Name:             "Test Basket",
		RegistryOperator: registryOp,
	}
	err := ls.storage.StoreRecord(context.Background(), "txid123", 0, registration)
	require.NoError(t, err)

	// Lookup by basket ID
	question := &lookup.LookupQuestion{
		Service: "ls_basketmap",
		Query:   makeQuery(map[string]interface{}{"basketID": basketID, "registryOperators": []string{registryOp}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid123", results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestBasketMapLookupService_Lookup_ByName(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// Store multiple records with similar names
	registryOp := "02operator1"
	registration1 := BasketMapRegistration{
		BasketID:         "basket-1",
		Name:             "Payment Basket",
		RegistryOperator: registryOp,
	}
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, registration1)
	require.NoError(t, err)

	registration2 := BasketMapRegistration{
		BasketID:         "basket-2",
		Name:             "Payments",
		RegistryOperator: registryOp,
	}
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)

	registration3 := BasketMapRegistration{
		BasketID:         "basket-3",
		Name:             "Other",
		RegistryOperator: registryOp,
	}
	err = ls.storage.StoreRecord(context.Background(), "txid3", 0, registration3)
	require.NoError(t, err)

	// Lookup by name (fuzzy search)
	question := &lookup.LookupQuestion{
		Service: "ls_basketmap",
		Query:   makeQuery(map[string]interface{}{"name": "pay", "registryOperators": []string{registryOp}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestBasketMapLookupService_Lookup_NoResults(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// Lookup non-existent basket
	question := &lookup.LookupQuestion{
		Service: "ls_basketmap",
		Query:   makeQuery(map[string]interface{}{"basketID": "nonexistent", "registryOperators": []string{"operator1"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestBasketMapLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 1, registration)
	require.NoError(t, err)

	// Verify it exists
	results, err := ls.storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
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
	results, err = ls.storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBasketMapLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
	}
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
	results, err := ls.storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestBasketMapLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
	}
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
	results, err := ls.storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBasketMapLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Mark as no longer retained
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_basketmap")
	require.NoError(t, err)

	// Verify it's deleted
	results, err := ls.storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBasketMapLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	registration := BasketMapRegistration{
		BasketID:         "test-basket",
		Name:             "Test Basket",
		RegistryOperator: "02operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Try with wrong topic
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_other")
	require.NoError(t, err)

	// Verify it still exists (was not deleted)
	results, err := ls.storage.FindByID(context.Background(), "test-basket", []string{"02operator1"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestBasketMapLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewBasketMapLookupService(db)

	// This is a no-op for BasketMap, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000003"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
