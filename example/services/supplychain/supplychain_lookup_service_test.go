package supplychain

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

	// Use a short timeout for connection attempts (2 seconds)
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
	return client.Database("supplychain_test_" + t.Name())
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

func TestSupplyChainLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestSupplyChainLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "SupplyChain Lookup Service")
	assert.Contains(t, docs, "ls_supplychain")
	assert.Contains(t, docs, "chainId")
}

func TestSupplyChainLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "SupplyChain Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestSupplyChainLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestSupplyChainLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"chainId": "test"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	// Service name mismatch doesn't error in this implementation, just returns results
}

func TestSupplyChainLookupService_Lookup_ByChainID(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store a record first
	chainID := "supply-chain-123"
	offChainValues := map[string]interface{}{
		"chainId": chainID,
		"data":    "test data",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid123", 0, offChainValues)
	require.NoError(t, err)

	// Lookup by chainId
	question := &lookup.LookupQuestion{
		Service: "ls_supplychain",
		Query:   makeQuery(map[string]interface{}{"chainId": chainID}),
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

func TestSupplyChainLookupService_Lookup_ByTxid(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store multiple records with the same txid
	offChainValues1 := map[string]interface{}{
		"chainId": "chain1",
		"data":    "test data 1",
	}
	offChainValues2 := map[string]interface{}{
		"chainId": "chain2",
		"data":    "test data 2",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid456", 0, offChainValues1)
	require.NoError(t, err)
	err = ls.storage.StoreRecord(context.Background(), "txid456", 1, offChainValues2)
	require.NoError(t, err)

	// Lookup by txid
	question := &lookup.LookupQuestion{
		Service: "ls_supplychain",
		Query:   makeQuery(map[string]interface{}{"txid": "txid456"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestSupplyChainLookupService_Lookup_WithPagination(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store multiple records
	for i := 0; i < 10; i++ {
		offChainValues := map[string]interface{}{
			"chainId": "test-chain",
			"index":   i,
		}
		err := ls.storage.StoreRecord(context.Background(), "txid", i, offChainValues)
		require.NoError(t, err)
	}

	// Lookup with limit and skip
	question := &lookup.LookupQuestion{
		Service: "ls_supplychain",
		Query:   makeQuery(map[string]interface{}{"chainId": "test-chain", "limit": 5, "skip": 2}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 5)
}

func TestSupplyChainLookupService_Lookup_WithDateRange(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store a record
	offChainValues := map[string]interface{}{
		"chainId": "date-test",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid789", 0, offChainValues)
	require.NoError(t, err)

	// Lookup with date range (today's date range)
	startDate := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	endDate := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	question := &lookup.LookupQuestion{
		Service: "ls_supplychain",
		Query:   makeQuery(map[string]interface{}{"startDate": startDate, "endDate": endDate}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.NotEmpty(t, results)
}

func TestSupplyChainLookupService_Lookup_EmptyResults(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Lookup non-existent chainId
	question := &lookup.LookupQuestion{
		Service: "ls_supplychain",
		Query:   makeQuery(map[string]interface{}{"chainId": "nonexistent"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestSupplyChainLookupService_Lookup_InvalidQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	tests := []struct {
		name    string
		query   map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "negative_limit",
			query:   map[string]interface{}{"chainId": "test", "limit": -5},
			wantErr: true,
			errMsg:  "limit must be a non-negative number",
		},
		{
			name:    "negative_skip",
			query:   map[string]interface{}{"chainId": "test", "skip": -1},
			wantErr: true,
			errMsg:  "skip must be a non-negative number",
		},
		{
			name:    "invalid_startDate",
			query:   map[string]interface{}{"startDate": "invalid-date"},
			wantErr: true,
			errMsg:  "invalid startDate format",
		},
		{
			name:    "invalid_endDate",
			query:   map[string]interface{}{"endDate": "invalid-date"},
			wantErr: true,
			errMsg:  "invalid endDate format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			question := &lookup.LookupQuestion{
				Service: "ls_supplychain",
				Query:   makeQuery(tt.query),
			}
			answer, err := ls.Lookup(context.Background(), question)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, answer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, answer)
			}
		})
	}
}

func TestSupplyChainLookupService_OutputAdmittedByTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Create a valid transaction
	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis: 1000,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Test with off-chain values
	offChainValuesJSON := []byte(`{"chainId":"test-chain","data":"test"}`)
	output := &engine.OutputAdmittedByTopic{
		Topic:          "tm_supplychain",
		OutputIndex:    0,
		AtomicBEEF:     beef,
		OffChainValues: offChainValuesJSON,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), output)
	require.NoError(t, err)

	// Verify the record was stored
	results, err := ls.storage.FindByChainID(context.Background(), "test-chain", 10, 0)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSupplyChainLookupService_OutputAdmittedByTopic_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis: 1000,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Test with wrong topic
	offChainValuesJSON := []byte(`{"chainId":"test-chain"}`)
	output := &engine.OutputAdmittedByTopic{
		Topic:          "tm_other",
		OutputIndex:    0,
		AtomicBEEF:     beef,
		OffChainValues: offChainValuesJSON,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), output)
	require.NoError(t, err)

	// Verify no record was stored
	results, err := ls.storage.FindByChainID(context.Background(), "test-chain", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSupplyChainLookupService_OutputAdmittedByTopic_NoOffChainValues(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis: 1000,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Test with no off-chain values (should use txid as chainId)
	output := &engine.OutputAdmittedByTopic{
		Topic:          "tm_supplychain",
		OutputIndex:    0,
		AtomicBEEF:     beef,
		OffChainValues: nil,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), output)
	require.NoError(t, err)

	// Verify the record was stored with txid as chainId
	txid := tx.TxID().String()
	results, err := ls.storage.FindByChainID(context.Background(), txid, 10, 0)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSupplyChainLookupService_OutputAdmittedByTopic_MissingChainID(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis: 1000,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Test with off-chain values but no chainId field
	offChainValuesJSON := []byte(`{"data":"test"}`)
	output := &engine.OutputAdmittedByTopic{
		Topic:          "tm_supplychain",
		OutputIndex:    0,
		AtomicBEEF:     beef,
		OffChainValues: offChainValuesJSON,
	}

	err = ls.OutputAdmittedByTopic(context.Background(), output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing chainId")
}

func TestSupplyChainLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store a record first
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	offChainValues := map[string]interface{}{
		"chainId": "test-chain",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 1, offChainValues)
	require.NoError(t, err)

	// Verify it exists
	results, err := ls.storage.FindByTxid(context.Background(), txidHex, 10, 0, "desc")
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)
	spendingTxidHash := makeHashFromHex("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	require.NotNil(t, spendingTxidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_supplychain",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
		SpendingTxid: spendingTxidHash,
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Note: SupplyChain's OutputSpent marks as spent, but doesn't delete
}

func TestSupplyChainLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store a record first
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	offChainValues := map[string]interface{}{
		"chainId": "test-chain",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, offChainValues)
	require.NoError(t, err)

	// Try to mark as spent with wrong topic
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)
	spendingTxidHash := makeHashFromHex("1111111111111111111111111111111111111111111111111111111111111111")

	payload := &engine.OutputSpent{
		Topic: "tm_other",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 0,
		},
		SpendingTxid: spendingTxidHash,
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it still exists (was not affected)
	results, err := ls.storage.FindByTxid(context.Background(), txidHex, 10, 0, "desc")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSupplyChainLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store a record first
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	offChainValues := map[string]interface{}{
		"chainId": "test-chain",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, offChainValues)
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
	results, err := ls.storage.FindByTxid(context.Background(), txidHex, 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSupplyChainLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store a record first
	txidHex := "1111111122222222333333334444444455555555666666667777777788888888"
	offChainValues := map[string]interface{}{
		"chainId": "test-chain",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, offChainValues)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_supplychain")
	require.NoError(t, err)

	// Verify it's deleted
	results, err := ls.storage.FindByTxid(context.Background(), txidHex, 10, 0, "desc")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSupplyChainLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// Store a record first
	txidHex := "2222222233333333444444445555555566666666777777778888888899999999"
	offChainValues := map[string]interface{}{
		"chainId": "test-chain",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, offChainValues)
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

	// Verify it still exists (was not affected)
	results, err := ls.storage.FindByTxid(context.Background(), txidHex, 10, 0, "desc")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSupplyChainLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSupplyChainLookupService(db)

	// This is a no-op for SupplyChain, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
