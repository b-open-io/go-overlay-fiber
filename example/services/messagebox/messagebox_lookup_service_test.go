package messagebox

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
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
	return client.Database("messagebox_test_" + t.Name())
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

func TestMessageBoxLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestMessageBoxLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "MessageBox Lookup Service")
	assert.Contains(t, docs, "ls_messagebox")
	assert.Contains(t, docs, "tm_messagebox")
}

func TestMessageBoxLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "MessageBox Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestMessageBoxLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestMessageBoxLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"identityKey": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "unsupported lookup service")
}

func TestMessageBoxLookupService_Lookup_MissingIdentityKey(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_messagebox",
		Query:   makeQuery(map[string]interface{}{"host": "https://example.com"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "identityKey")
}

func TestMessageBoxLookupService_Lookup_ByIdentityKey(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// Create a key and store a record
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)
	identityKeyHex := hex.EncodeToString(privateKey.PubKey().Compressed())

	testHost := "https://alice-messagebox.example.com"
	err = ls.storage.StoreRecord(identityKeyHex, testHost, "txid123", 0)
	require.NoError(t, err)

	// Allow time for MongoDB to update
	time.Sleep(100 * time.Millisecond)

	// Lookup by identity key
	question := &lookup.LookupQuestion{
		Service: "ls_messagebox",
		Query:   makeQuery(map[string]interface{}{"identityKey": identityKeyHex}),
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

func TestMessageBoxLookupService_Lookup_ByIdentityKeyAndHost(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// Create a key and store multiple records
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)
	identityKeyHex := hex.EncodeToString(privateKey.PubKey().Compressed())

	host1 := "https://alice-messagebox.example.com"
	host2 := "https://backup-messagebox.example.com"

	err = ls.storage.StoreRecord(identityKeyHex, host1, "txid123", 0)
	require.NoError(t, err)
	err = ls.storage.StoreRecord(identityKeyHex, host2, "txid456", 0)
	require.NoError(t, err)

	// Allow time for MongoDB to update
	time.Sleep(100 * time.Millisecond)

	// Lookup by identity key and specific host
	question := &lookup.LookupQuestion{
		Service: "ls_messagebox",
		Query: makeQuery(map[string]interface{}{
			"identityKey": identityKeyHex,
			"host":        host1,
		}),
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

func TestMessageBoxLookupService_OutputAdmittedByTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// Create a valid MessageBox transaction
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	identityKey := privateKey.PubKey().Compressed()
	host := []byte("https://example-messagebox.com")

	// Create data to sign: identityKey + host
	dataToSign := append(identityKey, host...)
	signature, err := privateKey.Sign(dataToSign)
	require.NoError(t, err)

	fields := [][]byte{
		identityKey,
		host,
		signature.Serialize(),
	}

	lockingScript, err := createMessageBoxPushDropScript(privateKey, fields)
	require.NoError(t, err)

	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Call OutputAdmittedByTopic
	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_messagebox",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify record was stored
	identityKeyHex := hex.EncodeToString(identityKey)
	results, err := ls.storage.FindAdvertisements(identityKeyHex, "")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, tx.TxID().String(), results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestMessageBoxLookupService_OutputAdmittedByTopic_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// Create a transaction
	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis: 1,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Call OutputAdmittedByTopic with wrong topic
	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_other",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify no records were stored
	results, err := ls.storage.FindAll()
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestMessageBoxLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// Store a record first
	identityKeyHex := "02" + "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := ls.storage.StoreRecord(identityKeyHex, "https://example.com", txidHex, 1)
	require.NoError(t, err)

	// Verify it exists
	results, err := ls.storage.FindAdvertisements(identityKeyHex, "")
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_messagebox",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = ls.storage.FindAdvertisements(identityKeyHex, "")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestMessageBoxLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// Store a record first
	identityKeyHex := "02" + "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	err := ls.storage.StoreRecord(identityKeyHex, "https://example.com", txidHex, 0)
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
	results, err := ls.storage.FindAdvertisements(identityKeyHex, "")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMessageBoxLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// Store a record first
	identityKeyHex := "02" + "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	err := ls.storage.StoreRecord(identityKeyHex, "https://example.com", txidHex, 0)
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
	results, err := ls.storage.FindAdvertisements(identityKeyHex, "")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestMessageBoxLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// This function should not error
	txidHash := makeHashFromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err := ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_messagebox")
	require.NoError(t, err)
}

func TestMessageBoxLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewMessageBoxLookupService(db)

	// This function should not error
	txidHash := makeHashFromHex("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
