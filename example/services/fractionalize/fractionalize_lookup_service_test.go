package fractionalize

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/script"
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
	return client.Database("fractionalize_test_" + t.Name())
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

func TestFractionalizeLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestFractionalizeLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Fractionalize Lookup Service")
	assert.Contains(t, docs, "ls_fractionalize")
	assert.Contains(t, docs, "txid")
	assert.Contains(t, docs, "limit")
	assert.Contains(t, docs, "skip")
}

func TestFractionalizeLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Fractionalize Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestFractionalizeLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestFractionalizeLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// The Lookup method doesn't validate service name, so this should work but return empty results
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"txid": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)
}

func TestFractionalizeLookupService_Lookup_InvalidQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)
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
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{"limit": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "limit must be a non-negative number")
}

func TestFractionalizeLookupService_Lookup_NegativeSkip(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{"skip": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "skip must be a non-negative number")
}

func TestFractionalizeLookupService_Lookup_InvalidDateFormat(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{"startDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "invalid startDate format")
}

func TestFractionalizeLookupService_Lookup_ByTxid(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store a record first
	testTxid := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := ls.storage.StoreRecord(context.Background(), testTxid, 0)
	require.NoError(t, err)

	// Lookup by txid
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{"txid": testTxid}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, testTxid, results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestFractionalizeLookupService_Lookup_ByTxid_NotFound(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Lookup non-existent txid
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{"txid": "nonexistent"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestFractionalizeLookupService_Lookup_FindAll(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store multiple records
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0)
	require.NoError(t, err)
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0)
	require.NoError(t, err)
	err = ls.storage.StoreRecord(context.Background(), "txid3", 0)
	require.NoError(t, err)

	// Lookup all records
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 3)
}

func TestFractionalizeLookupService_Lookup_WithLimit(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store multiple records
	for i := 0; i < 10; i++ {
		err := ls.storage.StoreRecord(context.Background(), "txid"+string(rune('0'+i)), 0)
		require.NoError(t, err)
	}

	// Lookup with limit
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{"limit": 5}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 5)
}

func TestFractionalizeLookupService_Lookup_WithSkip(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store multiple records
	for i := 0; i < 10; i++ {
		err := ls.storage.StoreRecord(context.Background(), "txid"+string(rune('0'+i)), 0)
		require.NoError(t, err)
	}

	// Lookup with skip
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{"skip": 5, "limit": 100}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 5)
}

func TestFractionalizeLookupService_Lookup_WithDateRange(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store a record
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0)
	require.NoError(t, err)

	// Lookup with date range
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query: makeQuery(map[string]interface{}{
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
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store multiple records with slight delays to ensure different timestamps
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0)
	require.NoError(t, err)

	// Lookup with ascending sort order
	question := &lookup.LookupQuestion{
		Service: "ls_fractionalize",
		Query:   makeQuery(map[string]interface{}{"sortOrder": "asc"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
	assert.Equal(t, "txid1", results[0].Txid)
}

func TestFractionalizeLookupService_OutputAdmittedByTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

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
	result, err := ls.storage.FindByTxid(context.Background(), txid)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, txid, result.Txid)
	assert.Equal(t, 0, result.OutputIndex)
}

func TestFractionalizeLookupService_OutputAdmittedByTopic_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

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
	result, err := ls.storage.FindByTxid(context.Background(), txid)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestFractionalizeLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := ls.storage.StoreRecord(context.Background(), txidHex, 1)
	require.NoError(t, err)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)
	spendingTxidHash := makeHashFromHex("fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321")
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
	result, err := ls.storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFractionalizeLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0)
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

	// Verify it still exists unchanged
	result, err := ls.storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFractionalizeLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0)
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
	result, err := ls.storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestFractionalizeLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store a record first
	txidHex := "1111111111111111111111111111111111111111111111111111111111111111"
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_fractionalize")
	require.NoError(t, err)

	// Verify it's deleted
	result, err := ls.storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestFractionalizeLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// Store a record first
	txidHex := "2222222222222222222222222222222222222222222222222222222222222222"
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0)
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

	// Verify it still exists
	result, err := ls.storage.FindByTxid(context.Background(), txidHex)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFractionalizeLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewFractionalizeLookupService(db)

	// This is a no-op for Fractionalize, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
