package apps

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
	return client.Database("apps_test_" + t.Name())
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

func TestAppsLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestAppsLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Apps Lookup Service")
	assert.Contains(t, docs, "ls_apps")
}

func TestAppsLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Apps Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestAppsLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestAppsLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"domain": "example.com"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	// The service doesn't check service name, it just processes the query
}

func TestAppsLookupService_Lookup_EmptyQuery(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   makeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestAppsLookupService_Lookup_ByDomain(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store a record first
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid123", 0, metadata)
	require.NoError(t, err)

	// Lookup by domain
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   makeQuery(map[string]interface{}{"domain": "example.com"}),
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

func TestAppsLookupService_Lookup_ByPublisher(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store multiple records with the same publisher
	publisherKey := "02abcdef1234567890"
	metadata1 := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "App One",
		Description: "First app",
		Icon:        "https://example.com/icon1.png",
		HTTPURL:     "https://app1.com",
		Domain:      "app1.com",
		Publisher:   publisherKey,
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, metadata1)
	require.NoError(t, err)

	metadata2 := &PublishedAppMetadata{
		Version:     "2.0.0",
		Name:        "App Two",
		Description: "Second app",
		Icon:        "https://example.com/icon2.png",
		HTTPURL:     "https://app2.com",
		Domain:      "app2.com",
		Publisher:   publisherKey,
		ReleaseDate: "2025-01-02",
	}
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, metadata2)
	require.NoError(t, err)

	metadata3 := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "App Three",
		Description: "Third app",
		Icon:        "https://example.com/icon3.png",
		HTTPURL:     "https://app3.com",
		Domain:      "app3.com",
		Publisher:   "different_key",
		ReleaseDate: "2025-01-03",
	}
	err = ls.storage.StoreRecord(context.Background(), "txid3", 0, metadata3)
	require.NoError(t, err)

	// Lookup by publisher
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   makeQuery(map[string]interface{}{"publisher": publisherKey}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestAppsLookupService_Lookup_ByName(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store a record
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Calculator",
		Description: "A calculator app",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://calc.com",
		Domain:      "calc.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid456", 0, metadata)
	require.NoError(t, err)

	// Lookup by name (fuzzy)
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   makeQuery(map[string]interface{}{"name": "calc"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid456", results[0].Txid)
}

func TestAppsLookupService_Lookup_ByOutpoint(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store a record
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid789", 2, metadata)
	require.NoError(t, err)

	// Lookup by outpoint
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   makeQuery(map[string]interface{}{"outpoint": "txid789.2"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid789", results[0].Txid)
	assert.Equal(t, 2, results[0].OutputIndex)
}

func TestAppsLookupService_Lookup_ByTags(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store records with different tags
	metadata1 := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Productivity App",
		Description: "A productivity tool",
		Icon:        "https://example.com/icon1.png",
		HTTPURL:     "https://productivity.com",
		Domain:      "productivity.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
		Tags:        []string{"productivity", "work"},
	}
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, metadata1)
	require.NoError(t, err)

	metadata2 := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Finance App",
		Description: "A finance tool",
		Icon:        "https://example.com/icon2.png",
		HTTPURL:     "https://finance.com",
		Domain:      "finance.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-02",
		Tags:        []string{"finance", "money"},
	}
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, metadata2)
	require.NoError(t, err)

	// Lookup by tags
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   makeQuery(map[string]interface{}{"tags": []string{"productivity"}}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid1", results[0].Txid)
}

func TestAppsLookupService_Lookup_ByCategory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store records with different categories
	metadata1 := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Game App",
		Description: "A game",
		Icon:        "https://example.com/icon1.png",
		HTTPURL:     "https://game.com",
		Domain:      "game.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
		Category:    "Games",
	}
	err := ls.storage.StoreRecord(context.Background(), "txid1", 0, metadata1)
	require.NoError(t, err)

	metadata2 := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Tool App",
		Description: "A tool",
		Icon:        "https://example.com/icon2.png",
		HTTPURL:     "https://tool.com",
		Domain:      "tool.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-02",
		Category:    "Utilities",
	}
	err = ls.storage.StoreRecord(context.Background(), "txid2", 0, metadata2)
	require.NoError(t, err)

	// Lookup by category
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   makeQuery(map[string]interface{}{"category": "Games"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid1", results[0].Txid)
}

func TestAppsLookupService_Lookup_NoResults(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Lookup non-existent domain
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   makeQuery(map[string]interface{}{"domain": "nonexistent.com"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeFreeform, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestAppsLookupService_OutputSpent(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 1, metadata)
	require.NoError(t, err)

	// Verify it exists
	results, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".1")
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	payload := &engine.OutputSpent{
		Topic: "tm_apps",
		Outpoint: &transaction.Outpoint{
			Txid:  *txidHash,
			Index: 1,
		},
	}
	err = ls.OutputSpent(context.Background(), payload)
	require.NoError(t, err)

	// Verify it's deleted
	results, err = ls.storage.FindByOutpoint(context.Background(), txidHex+".1")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAppsLookupService_OutputSpent_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, metadata)
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
	results, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestAppsLookupService_OutputEvicted(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, metadata)
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
	results, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAppsLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "1111111111111111111111111111111111111111111111111111111111111111"
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, metadata)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_apps")
	require.NoError(t, err)

	// Verify it's deleted
	results, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAppsLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// Store a record first - use a valid hex txid
	txidHex := "2222222222222222222222222222222222222222222222222222222222222222"
	metadata := &PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   "02pubkey",
		ReleaseDate: "2025-01-01",
	}
	err := ls.storage.StoreRecord(context.Background(), txidHex, 0, metadata)
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

	// Verify it still exists (was not deleted)
	results, err := ls.storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestAppsLookupService_OutputBlockHeightUpdated(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewAppsLookupService(db)

	// This is a no-op for Apps, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := makeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
