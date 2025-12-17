package ump

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
	return client.Database("ump_test_" + t.Name())
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

func TestUMPLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestUMPLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "User Management Protocol")
	assert.Contains(t, docs, "presentationHash")
	assert.Contains(t, docs, "recoveryHash")
	assert.Contains(t, docs, "outpoint")
}

func TestUMPLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "UMP Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestUMPLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestUMPLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)
	// UMP lookup service doesn't validate service name, so just test with empty query
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "query parameters")
}

func TestUMPLookupService_Lookup_EmptyQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_ump",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "query parameters")
}

func TestUMPLookupService_Lookup_ByPresentationHash(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Store a record first
	testPresentationHash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	record := &UMPRecord{
		Txid:             "txid123",
		OutputIndex:      0,
		PresentationHash: testPresentationHash,
		RecoveryHash:     "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
	}
	err := ls.storage.InsertRecord(context.Background(), record)
	require.NoError(t, err)

	// Lookup by presentationHash
	question := &lookup.LookupQuestion{
		Service: "ls_ump",
		Query:   makeQuery(map[string]interface{}{"presentationHash": testPresentationHash}),
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

func TestUMPLookupService_Lookup_ByRecoveryHash(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Store a record first
	testRecoveryHash := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	record := &UMPRecord{
		Txid:             "txid456",
		OutputIndex:      1,
		PresentationHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		RecoveryHash:     testRecoveryHash,
	}
	err := ls.storage.InsertRecord(context.Background(), record)
	require.NoError(t, err)

	// Lookup by recoveryHash
	question := &lookup.LookupQuestion{
		Service: "ls_ump",
		Query:   makeQuery(map[string]interface{}{"recoveryHash": testRecoveryHash}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid456", results[0].Txid)
	assert.Equal(t, 1, results[0].OutputIndex)
}

func TestUMPLookupService_Lookup_ByOutpoint(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Store a record first
	record := &UMPRecord{
		Txid:             "txid789",
		OutputIndex:      2,
		PresentationHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		RecoveryHash:     "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
	}
	err := ls.storage.InsertRecord(context.Background(), record)
	require.NoError(t, err)

	// Lookup by outpoint
	question := &lookup.LookupQuestion{
		Service: "ls_ump",
		Query:   makeQuery(map[string]interface{}{"outpoint": "txid789.2"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid789", results[0].Txid)
	assert.Equal(t, 2, results[0].OutputIndex)
}

func TestUMPLookupService_Lookup_NoResults(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Lookup non-existent presentationHash
	question := &lookup.LookupQuestion{
		Service: "ls_ump",
		Query:   makeQuery(map[string]interface{}{"presentationHash": "nonexistent1234567890abcdef1234567890abcdef1234567890abcdef1234"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestUMPLookupService_OutputAdmittedByTopic_ValidToken(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Create a valid UMP transaction
	tx, err := createValidUMPTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Simulate output admission
	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_users",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify record was inserted - lookup by outpoint
	txid := tx.TxID().String()
	record, err := ls.storage.FindByOutpoint(context.Background(), txid+".0")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, txid, record.Txid)
	assert.Equal(t, 0, record.OutputIndex)
}

func TestUMPLookupService_OutputAdmittedByTopic_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	tx, err := createValidUMPTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Try with wrong topic
	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_other",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify no record was inserted
	txid := tx.TxID().String()
	record, err := ls.storage.FindByOutpoint(context.Background(), txid+".0")
	require.NoError(t, err)
	assert.Nil(t, record)
}

func TestUMPLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	record := &UMPRecord{
		Txid:             txidHex,
		OutputIndex:      1,
		PresentationHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		RecoveryHash:     "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
	}
	err := ls.storage.InsertRecord(context.Background(), record)
	require.NoError(t, err)

	// Verify it exists
	result, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".1")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_users",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	result, err = ls.storage.FindByOutpoint(context.Background(), txidHex+".1")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestUMPLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	record := &UMPRecord{
		Txid:             txidHex,
		OutputIndex:      0,
		PresentationHash: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		RecoveryHash:     "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
	}
	err := ls.storage.InsertRecord(context.Background(), record)
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
	result, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestUMPLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	record := &UMPRecord{
		Txid:             txidHex,
		OutputIndex:      0,
		PresentationHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		RecoveryHash:     "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}
	err := ls.storage.InsertRecord(context.Background(), record)
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
	result, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestUMPLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "1111111111111111111111111111111111111111111111111111111111111111"
	record := &UMPRecord{
		Txid:             txidHex,
		OutputIndex:      0,
		PresentationHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		RecoveryHash:     "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
	}
	err := ls.storage.InsertRecord(context.Background(), record)
	require.NoError(t, err)

	// Remove from history
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_users")
	require.NoError(t, err)

	// Verify it's deleted
	result, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestUMPLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "2222222222222222222222222222222222222222222222222222222222222222"
	record := &UMPRecord{
		Txid:             txidHex,
		OutputIndex:      0,
		PresentationHash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		RecoveryHash:     "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
	}
	err := ls.storage.InsertRecord(context.Background(), record)
	require.NoError(t, err)

	// Try to remove from history with wrong topic
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_other")
	require.NoError(t, err)

	// Verify it still exists (was not deleted)
	result, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestUMPLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUMPLookupService(db)

	// This is a no-op for UMP, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
