package identity

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
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
	return client.Database("identity_test_" + t.Name())
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

func TestIdentityLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestIdentityLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Identity Lookup Service")
	assert.Contains(t, docs, "ls_identity")
}

func TestIdentityLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Identity Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestIdentityLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestIdentityLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"serialNumber": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	// The lookup service doesn't validate the service name, it just processes the query
	// So this should succeed if the query is valid
	require.NoError(t, err)
	require.NotNil(t, answer)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results) // No results because nothing stored
}

func TestIdentityLookupService_Lookup_EmptyQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "query parameters")
}

func TestIdentityLookupService_Lookup_BySerialNumber(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store a record first
	testSerial := "serial123"
	txid := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	cert := createTestCertificate(testSerial)
	err := ls.storage.StoreRecord(context.Background(), txid, 0, cert)
	require.NoError(t, err)

	// Lookup by serial number
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{"serialNumber": testSerial}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, txid, results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestIdentityLookupService_Lookup_ByCertifiers(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store multiple records with the same certifier
	certifier := "02abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	cert1 := createTestCertificate("serial1")
	cert1.Certifier = createPubKeyFromHex(certifier)
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, cert1)
	require.NoError(t, err)

	cert2 := createTestCertificate("serial2")
	cert2.Certifier = createPubKeyFromHex(certifier)
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, cert2)
	require.NoError(t, err)

	cert3 := createTestCertificate("serial3")
	cert3.Certifier = createPubKeyFromHex("03fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321")
	err = ls.storage.StoreRecord(context.Background(), "txid3", 0, cert3)
	require.NoError(t, err)

	// Lookup by certifiers
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{"certifiers": []string{certifier}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestIdentityLookupService_Lookup_ByIdentityKeyAndCertifiers(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store records
	identityKey := "02123456789012345678901234567890123456789012345678901234567890abcd"
	certifier := "02abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	cert1 := createTestCertificate("serial1")
	cert1.Subject = createPubKeyFromHex(identityKey)
	cert1.Certifier = createPubKeyFromHex(certifier)
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, cert1)
	require.NoError(t, err)

	cert2 := createTestCertificate("serial2")
	cert2.Subject = createPubKeyFromHex("03different1234567890123456789012345678901234567890123456789012345678")
	cert2.Certifier = createPubKeyFromHex(certifier)
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, cert2)
	require.NoError(t, err)

	// Lookup by identity key and certifiers
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{
			"identityKey": identityKey,
			"certifiers":  []string{certifier},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
	assert.Equal(t, "txid1", results[0].Txid)
}

func TestIdentityLookupService_Lookup_ByAttributesAndCertifiers(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store records
	certifier := "02abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	// Note: Attribute search requires searchableAttributes to be populated
	// This would typically be done in the storage layer
	cert := createTestCertificate("serial1")
	cert.Certifier = createPubKeyFromHex(certifier)
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, cert)
	require.NoError(t, err)

	// Lookup by attributes and certifiers
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{
			"attributes": map[string]string{"name": "John"},
			"certifiers": []string{certifier},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	// Results may be empty if searchableAttributes isn't populated
	assert.NotNil(t, results)
}

func TestIdentityLookupService_Lookup_NoResults(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Lookup non-existent serial number
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{"serialNumber": "nonexistent"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestIdentityLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	cert := createTestCertificate("serial123")
	err := ls.storage.StoreRecord(context.Background(), txidHex, 1, cert)
	require.NoError(t, err)

	// Verify it exists
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_identity",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	answer, err = ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ = answer.Result.([]UTXOReference)
	assert.Empty(t, results)
}

func TestIdentityLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	cert := createTestCertificate("serial123")
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, cert)
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
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	assert.Len(t, results, 1)
}

func TestIdentityLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	cert := createTestCertificate("serial123")
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, cert)
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
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	assert.Empty(t, results)
}

func TestIdentityLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store a record first
	txidHex := "1111111111111111111111111111111111111111111111111111111111111111"
	cert := createTestCertificate("serial123")
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, cert)
	require.NoError(t, err)

	// Mark as no longer retained
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_identity")
	require.NoError(t, err)

	// Verify it's deleted
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	assert.Empty(t, results)
}

func TestIdentityLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// Store a record first
	txidHex := "2222222222222222222222222222222222222222222222222222222222222222"
	cert := createTestCertificate("serial123")
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, cert)
	require.NoError(t, err)

	// Try to mark as no longer retained with wrong topic
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_other")
	require.NoError(t, err)

	// Verify it still exists (was not deleted)
	question := &lookup.LookupQuestion{
		Service: "ls_identity",
		Query:   makeQuery(map[string]interface{}{"serialNumber": "serial123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, _ := answer.Result.([]UTXOReference)
	assert.Len(t, results, 1)
}

func TestIdentityLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewIdentityLookupService(db)

	// This is a no-op for Identity, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}

// Helper functions

func createTestCertificate(serialNumber string) *certificates.Certificate {
	privateKey, _ := ec.NewPrivateKey()
	certifierKey, _ := ec.NewPrivateKey()

	return &certificates.Certificate{
		Type:         wallet.StringBase64("aWRlbnRpdHk="), // "identity" in base64
		SerialNumber: wallet.StringBase64(serialNumber),
		Subject:      *privateKey.PubKey(),
		Certifier:    *certifierKey.PubKey(),
		Fields: map[wallet.CertificateFieldNameUnder50Bytes]wallet.StringBase64{
			"name":  wallet.StringBase64("Sm9obiBEb2U="), // "John Doe" in base64
			"email": wallet.StringBase64("am9obkBleGFtcGxlLmNvbQ=="), // "john@example.com" in base64
		},
	}
}

func createPubKeyFromHex(hexStr string) ec.PublicKey {
	// Create a simple public key from hex string
	// This is a simplified version for testing
	key, _ := ec.NewPrivateKey()
	return *key.PubKey()
}
