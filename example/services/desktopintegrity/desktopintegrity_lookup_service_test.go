package desktopintegrity

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
	return client.Database("desktopintegrity_test_" + t.Name())
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

func TestDesktopIntegrityLookupService_NewInstance(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewDesktopIntegrityLookupService(db)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestDesktopIntegrityLookupService_GetDocumentation(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewDesktopIntegrityLookupService(db)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "DesktopIntegrity Lookup Service")
	assert.Contains(t, docs, "ls_desktopintegrity")
	assert.Contains(t, docs, "fileHash")
}

func TestDesktopIntegrityLookupService_GetMetaData(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewDesktopIntegrityLookupService(db)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "DesktopIntegrity Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestDesktopIntegrityLookupService_Lookup_NilQuestion(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewDesktopIntegrityLookupService(db)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestDesktopIntegrityLookupService_Lookup_WrongService(t *testing.T) {
	db := getTestMongoDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	ls := NewDesktopIntegrityLookupService(db)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   makeQuery(map[string]interface{}{"fileHash": "abc123"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	assert.Error(t, err)
	assert.Nil(t, answer)
}
