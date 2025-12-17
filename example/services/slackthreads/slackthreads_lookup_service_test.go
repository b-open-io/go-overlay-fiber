package slackthreads

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
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
	return client.Database("slackthreads_test_" + t.Name())
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

func TestSlackThreadsLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestSlackThreadsLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "SlackThread Lookup Service")
	assert.Contains(t, docs, "ls_slackthread")
}

func TestSlackThreadsLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "SlackThread Lookup Service", meta.Name)
	assert.Equal(t, "Find threads on-chain.", meta.Description)
}

func TestSlackThreadsLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "valid query")
}

func TestSlackThreadsLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"threadHash": "abcd"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "lookup service not supported")
}

func TestSlackThreadsLookupService_Lookup_ByThreadHash(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	// Store a record first
	testThreadHash := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	err := ls.storage.StoreRecord("txid123", 0, testThreadHash)
	require.NoError(t, err)

	// Lookup by thread hash
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"threadHash": testThreadHash}),
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

func TestSlackThreadsLookupService_Lookup_ByTxid(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	// Store a record first
	testTxid := "abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	testThreadHash := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	err := ls.storage.StoreRecord(testTxid, 1, testThreadHash)
	require.NoError(t, err)

	// Lookup by txid
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"txid": testTxid}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, testTxid, results[0].Txid)
	assert.Equal(t, 1, results[0].OutputIndex)
}

func TestSlackThreadsLookupService_Lookup_FindAll(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	// Store multiple records
	testHash1 := "1111111111111111111111111111111111111111111111111111111111111111"
	testHash2 := "2222222222222222222222222222222222222222222222222222222222222222"
	err := ls.storage.StoreRecord("txid1", 0, testHash1)
	require.NoError(t, err)
	err = ls.storage.StoreRecord("txid2", 1, testHash2)
	require.NoError(t, err)

	// Find all
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, "output-list", answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 2)
}

func TestSlackThreadsLookupService_Lookup_WithLimitAndSkip(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	// Store multiple records
	for i := 0; i < 5; i++ {
		threadHash := make([]byte, 32)
		for j := range threadHash {
			threadHash[j] = byte(i)
		}
		err := ls.storage.StoreRecord("txid"+string(rune('0'+i)), i, string(threadHash))
		require.NoError(t, err)
	}

	// Test with limit
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"limit": 2}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)

	// Test with skip
	question2 := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"limit": 10, "skip": 3}),
	}
	answer2, err := ls.Lookup(context.Background(), question2)
	require.NoError(t, err)
	results2, ok := answer2.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results2, 2)
}

func TestSlackThreadsLookupService_Lookup_InvalidLimit(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"limit": -1}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "limit")
}

func TestSlackThreadsLookupService_Lookup_InvalidSkip(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"skip": -5}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "skip")
}

func TestSlackThreadsLookupService_Lookup_WithDateRange(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	// Store records
	testHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err := ls.storage.StoreRecord("txid1", 0, testHash)
	require.NoError(t, err)

	// Query with date range
	startDate := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	endDate := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query: makeQuery(map[string]interface{}{
			"startDate": startDate,
			"endDate":   endDate,
		}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

func TestSlackThreadsLookupService_Lookup_InvalidDateFormat(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"startDate": "invalid-date"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
	assert.Contains(t, err.Error(), "startDate")
}

func TestSlackThreadsLookupService_Lookup_SortOrder(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewSlackThreadLookupService(db)

	// Store records with slight delay to ensure different timestamps
	hash1 := "1111111111111111111111111111111111111111111111111111111111111111"
	err := ls.storage.StoreRecord("txid1", 0, hash1)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	hash2 := "2222222222222222222222222222222222222222222222222222222222222222"
	err = ls.storage.StoreRecord("txid2", 0, hash2)
	require.NoError(t, err)

	// Test descending order (newest first)
	question := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"sortOrder": "desc"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 2)
	assert.Equal(t, "txid2", results[0].Txid)

	// Test ascending order (oldest first)
	question2 := &lookup.LookupQuestion{
		Service: "ls_slackthread",
		Query:   makeQuery(map[string]interface{}{"sortOrder": "asc"}),
	}
	answer2, err := ls.Lookup(context.Background(), question2)
	require.NoError(t, err)
	results2, ok := answer2.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results2, 2)
	assert.Equal(t, "txid1", results2[0].Txid)
}
