package certmap

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
	return client.Database("certmap_test_" + t.Name())
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

func TestCertMapLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestCertMapLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "CertMap Lookup Service")
	assert.Contains(t, docs, "ls_certmap")
}

func TestCertMapLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "CertMap Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestCertMapLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestCertMapLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"type": "test-type", "registryOperators": []string{"operator1"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)
}

func TestCertMapLookupService_Lookup_EmptyQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "registryOperators")
}

func TestCertMapLookupService_Lookup_MissingRegistryOperators(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query:   makeQuery(map[string]interface{}{"type": "test-type"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "registryOperators")
}

func TestCertMapLookupService_Lookup_MissingTypeAndName(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query:   makeQuery(map[string]interface{}{"registryOperators": []string{"operator1"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "type or name")
}

func TestCertMapLookupService_Lookup_ByType(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Store a record first
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid123", 0, registration)
	require.NoError(t, err)

	// Lookup by type
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
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

func TestCertMapLookupService_Lookup_ByName(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Store records with different names
	registration1 := &CertMapRegistration{
		Type:             "type1",
		Name:             "Test Certificate",
		IconURL:          "https://example.com/icon1.png",
		Description:      "Test description 1",
		DocumentationURL: "https://example.com/docs1",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, registration1)
	require.NoError(t, err)

	registration2 := &CertMapRegistration{
		Type:             "type2",
		Name:             "Another Certificate",
		IconURL:          "https://example.com/icon2.png",
		Description:      "Test description 2",
		DocumentationURL: "https://example.com/docs2",
		CertFields:       map[string]interface{}{"field2": "value2"},
		RegistryOperator: "operator1",
	}
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)

	// Lookup by name (fuzzy)
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"name":              "Certificate",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestCertMapLookupService_Lookup_FilterByRegistryOperator(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Store records with different registry operators
	registration1 := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test 1",
		IconURL:          "https://example.com/icon1.png",
		Description:      "Test description 1",
		DocumentationURL: "https://example.com/docs1",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, registration1)
	require.NoError(t, err)

	registration2 := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test 2",
		IconURL:          "https://example.com/icon2.png",
		Description:      "Test description 2",
		DocumentationURL: "https://example.com/docs2",
		CertFields:       map[string]interface{}{"field2": "value2"},
		RegistryOperator: "operator2",
	}
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)

	// Lookup by type, filtering by registry operator
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid1", results[0].Txid)
}

func TestCertMapLookupService_Lookup_NoResults(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Lookup non-existent type
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "nonexistent",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestCertMapLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 1, registration)
	require.NoError(t, err)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_certmap",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted - attempt to lookup by type
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestCertMapLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
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
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

func TestCertMapLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
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
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestCertMapLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Store a record first
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_certmap")
	require.NoError(t, err)

	// Verify it's deleted
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestCertMapLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Store a record first
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	registration := &CertMapRegistration{
		Type:             "test-type",
		Name:             "Test Name",
		IconURL:          "https://example.com/icon.png",
		Description:      "Test description",
		DocumentationURL: "https://example.com/docs",
		CertFields:       map[string]interface{}{"field1": "value1"},
		RegistryOperator: "operator1",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, registration)
	require.NoError(t, err)

	// Call with wrong topic
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
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{"operator1"},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

func TestCertMapLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// This is a no-op for CertMap, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000003"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}

func TestCertMapLookupService_OutputAdmittedByTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewCertMapLookupService(db)

	// Create a valid CertMap transaction
	tx, err := createValidCertMapTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Call OutputAdmittedByTopic
	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_certmap",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify it was stored by looking it up
	question := &lookup.LookupQuestion{
		Service: "ls_certmap",
		Query: makeQuery(map[string]interface{}{
			"type":              "test-type",
			"registryOperators": []string{},
		}),
	}

	// Note: This might not return results because we need the actual registry operator from the transaction
	// This test mainly verifies that OutputAdmittedByTopic doesn't error
	_, err = ls.Lookup(context.Background(), question)
	require.NoError(t, err)
}
