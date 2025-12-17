package uhrp

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
	return client.Database("uhrp_test_" + t.Name())
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

func TestUHRPLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestUHRPLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Universal Hash Resolution Protocol")
	assert.Contains(t, docs, "ls_uhrp")
}

func TestUHRPLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "UHRP Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestUHRPLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestUHRPLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"uhrpUrl": "uhrp://test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "unsupported lookup service")
}

func TestUHRPLookupService_Lookup_EmptyQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "lookup must specify")
}

func TestUHRPLookupService_Lookup_ByUHRPUrl(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// Store a record first
	testURL := "uhrp://abc123def456"
	err := ls.storage.StoreRecord(testURL, "txid123", 0, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
	require.NoError(t, err)

	// Lookup by UHRP URL
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   makeQuery(map[string]interface{}{"uhrpUrl": testURL}),
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

func TestUHRPLookupService_Lookup_ByOutpoint(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// Store a record first
	err := ls.storage.StoreRecord("uhrp://test", "txid456", 2, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
	require.NoError(t, err)

	// Lookup by outpoint
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   makeQuery(map[string]interface{}{"outpoint": "txid456.2"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid456", results[0].Txid)
	assert.Equal(t, 2, results[0].OutputIndex)
}

func TestUHRPLookupService_Lookup_ByHostIdentityKey(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// Store multiple records with the same host identity key
	hostKey := "02abcdef1234567890"
	err := ls.storage.StoreRecord("uhrp://url1", "txid1", 0, hostKey, "https://example.com/file1.dat", 1735689600, 1024)
	require.NoError(t, err)
	err = ls.storage.StoreRecord("uhrp://url2", "txid2", 0, hostKey, "https://example.com/file2.dat", 1735689600, 2048)
	require.NoError(t, err)
	err = ls.storage.StoreRecord("uhrp://url3", "txid3", 0, "different_key", "https://example.com/file3.dat", 1735689600, 512)
	require.NoError(t, err)

	// Lookup by host identity key
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   makeQuery(map[string]interface{}{"hostIdentityKey": hostKey}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestUHRPLookupService_Lookup_NoResults(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// Lookup non-existent URL
	question := &lookup.LookupQuestion{
		Service: "ls_uhrp",
		Query:   makeQuery(map[string]interface{}{"uhrpUrl": "uhrp://nonexistent"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestUHRPLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := ls.storage.StoreRecord("uhrp://test", txidHex, 1, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
	require.NoError(t, err)

	// Verify it exists
	results, err := ls.storage.Lookup(&UHRPQuery{Outpoint: txidHex + ".1"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
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
	results, err = ls.storage.Lookup(&UHRPQuery{Outpoint: txidHex + ".1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUHRPLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := ls.storage.StoreRecord("uhrp://test", txidHex, 0, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
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
	results, err := ls.storage.Lookup(&UHRPQuery{Outpoint: txidHex + ".0"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestUHRPLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := ls.storage.StoreRecord("uhrp://test", txidHex, 0, "02pubkey", "https://example.com/file.dat", 1735689600, 1024)
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
	results, err := ls.storage.Lookup(&UHRPQuery{Outpoint: txidHex + ".0"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUHRPLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// This is a no-op for UHRP, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err := ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_uhrp")
	require.NoError(t, err)
}

func TestUHRPLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewUHRPLookupService(db)

	// This is a no-op for UHRP, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
