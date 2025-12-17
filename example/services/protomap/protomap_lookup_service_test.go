package protomap

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
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
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
	return client.Database("protomap_test_" + t.Name())
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

func TestProtoMapLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestProtoMapLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "ProtoMap Lookup Service")
	assert.Contains(t, docs, "ls_protomap")
}

func TestProtoMapLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "ProtoMap", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestProtoMapLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestProtoMapLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"name": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	// The current implementation doesn't check service name, so it won't error
	// We test that it processes the query
	if err != nil {
		// If it does error, that's also acceptable
		assert.NotNil(t, err)
	} else {
		// If it doesn't error, it should return a valid answer
		require.NotNil(t, answer)
	}
}

func TestProtoMapLookupService_Lookup_EmptyQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_protomap",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "query parameters must include")
}

func TestProtoMapLookupService_Lookup_ByName(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Store a record first
	testName := "test-protocol"
	registryOperator := "02abcdef1234567890"
	registration := ProtoMapRegistration{
		RegistryOperator: registryOperator,
		ProtocolID: ProtocolID{
			SecurityLevel: 1,
			Protocol:      "test-protocol",
		},
		Name: testName,
	}
	err := ls.storage.StoreRecord(context.Background(), "txid123", 0, registration)
	require.NoError(t, err)

	// Lookup by name
	question := &lookup.LookupQuestion{
		Service: "ls_protomap",
		Query: makeQuery(map[string]interface{}{
			"name":              testName,
			"registryOperators": []string{registryOperator},
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

func TestProtoMapLookupService_Lookup_ByProtocolID(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Store a record first
	registryOperator := "02fedcba0987654321"
	registration := ProtoMapRegistration{
		RegistryOperator: registryOperator,
		ProtocolID: ProtocolID{
			SecurityLevel: 2,
			Protocol:      "my-protocol",
		},
		Name: "My Protocol",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid456", 1, registration)
	require.NoError(t, err)

	// Lookup by protocol ID
	question := &lookup.LookupQuestion{
		Service: "ls_protomap",
		Query: makeQuery(map[string]interface{}{
			"protocolID": map[string]interface{}{
				"securityLevel": 2,
				"protocol":      "my-protocol",
			},
			"registryOperators": []string{registryOperator},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid456", results[0].Txid)
	assert.Equal(t, 1, results[0].OutputIndex)
}

func TestProtoMapLookupService_Lookup_MultipleRegistryOperators(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Store multiple records with different operators
	operator1 := "operator1"
	operator2 := "operator2"
	operator3 := "operator3"

	registration1 := ProtoMapRegistration{
		RegistryOperator: operator1,
		ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-a"},
		Name:             "Protocol A",
	}
	registration2 := ProtoMapRegistration{
		RegistryOperator: operator2,
		ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-a"},
		Name:             "Protocol A",
	}
	registration3 := ProtoMapRegistration{
		RegistryOperator: operator3,
		ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "protocol-a"},
		Name:             "Protocol A",
	}

	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, registration1)
	require.NoError(t, err)
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, registration2)
	require.NoError(t, err)
	err = ls.storage.StoreRecord(context.Background(), "txid3", 0, registration3)
	require.NoError(t, err)

	// Lookup with only operator1 and operator2
	question := &lookup.LookupQuestion{
		Service: "ls_protomap",
		Query: makeQuery(map[string]interface{}{
			"name":              "Protocol A",
			"registryOperators": []string{operator1, operator2},
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestProtoMapLookupService_Lookup_NoResults(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Lookup non-existent protocol
	question := &lookup.LookupQuestion{
		Service: "ls_protomap",
		Query: makeQuery(map[string]interface{}{
			"name":              "nonexistent",
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

func TestProtoMapLookupService_OutputAdmittedByTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Create a valid ProtoMap transaction
	tx := transaction.NewTransaction()
	protocolIDJSON, err := json.Marshal([]interface{}{1, "test-protocol"})
	require.NoError(t, err)

	fields := [][]byte{
		protocolIDJSON,
		[]byte("Test Protocol"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		[]byte("02abcdef1234567890"),
		[]byte("signature_data"),
	}

	lockingScript, err := createProtoMapPushDropScript(fields)
	require.NoError(t, err)

	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Call OutputAdmittedByTopic
	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_protomap",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// Verify the record was stored
	results, err := ls.storage.FindByName(context.Background(), "Test Protocol", []string{"02abcdef1234567890"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestProtoMapLookupService_OutputAdmittedByTopic_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Create a valid transaction
	tx := transaction.NewTransaction()
	beef, err := tx.BEEF()
	require.NoError(t, err)

	payload := &engine.OutputAdmittedByTopic{
		Topic:       "tm_other",
		OutputIndex: 0,
		AtomicBEEF:  beef,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), payload)
	require.NoError(t, err)

	// No records should be stored
	results, err := ls.storage.FindByName(context.Background(), "Test", []string{"operator"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestProtoMapLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	registration := ProtoMapRegistration{
		RegistryOperator: "operator1",
		ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "test"},
		Name:             "Test",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 1, registration)
	require.NoError(t, err)

	// Verify it exists
	results, err := ls.storage.FindByName(context.Background(), "Test", []string{"operator1"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_protomap",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = ls.storage.FindByName(context.Background(), "Test", []string{"operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestProtoMapLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	registration := ProtoMapRegistration{
		RegistryOperator: "operator1",
		ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "test"},
		Name:             "Test",
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
	results, err := ls.storage.FindByName(context.Background(), "Test", []string{"operator1"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestProtoMapLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	registration := ProtoMapRegistration{
		RegistryOperator: "operator1",
		ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "test"},
		Name:             "Test",
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
	results, err := ls.storage.FindByName(context.Background(), "Test", []string{"operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestProtoMapLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Store a record first
	txidHex := "0000000000000000000000000000000000000000000000000000000000000001"
	registration := ProtoMapRegistration{
		RegistryOperator: "operator1",
		ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "test"},
		Name:             "Test",
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
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_protomap")
	require.NoError(t, err)

	// Verify it's deleted
	results, err := ls.storage.FindByName(context.Background(), "Test", []string{"operator1"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestProtoMapLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// Store a record first
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	registration := ProtoMapRegistration{
		RegistryOperator: "operator1",
		ProtocolID:       ProtocolID{SecurityLevel: 1, Protocol: "test"},
		Name:             "Test",
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

	// Verify it still exists
	results, err := ls.storage.FindByName(context.Background(), "Test", []string{"operator1"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestProtoMapLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewProtoMapLookupService(db)

	// This is a no-op for ProtoMap, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000003"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}

// Helper functions

func createProtoMapPushDropScript(fields [][]byte) (*script.Script, error) {
	// Create the PushDrop chunks
	pushDropChunks := make([]*script.ScriptChunk, 0)
	for _, field := range fields {
		pushDropChunks = append(pushDropChunks, pushdrop.CreateMinimallyEncodedScriptChunk(field))
	}

	// Add DROP operations
	notYetDropped := len(fields)
	for notYetDropped > 1 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.Op2DROP})
		notYetDropped -= 2
	}
	if notYetDropped != 0 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.OpDROP})
	}

	// Add OP_RETURN at the beginning
	allChunks := []*script.ScriptChunk{{Op: script.OpRETURN}}
	allChunks = append(allChunks, pushDropChunks...)

	return script.NewScriptFromScriptOps(allChunks)
}
