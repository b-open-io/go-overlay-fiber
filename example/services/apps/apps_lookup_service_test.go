package apps

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/testutil"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAppsStorage is a mock implementation of AppsStorageEngine for testing.
// It embeds MockStorageBase for common functionality and adds Apps-specific lookup logic.
type MockAppsStorage struct {
	*testutil.MockStorageBase[AppCatalogRecord]
	findError error
}

func NewMockAppsStorage() *MockAppsStorage {
	return &MockAppsStorage{
		MockStorageBase: testutil.NewMockStorageBase[AppCatalogRecord](),
	}
}

func (m *MockAppsStorage) StoreRecord(ctx context.Context, txid string, outputIndex int, metadata *PublishedAppMetadata) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Store(key, AppCatalogRecord{
		Txid:        txid,
		OutputIndex: outputIndex,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	})
}

func (m *MockAppsStorage) DeleteRecord(ctx context.Context, txid string, outputIndex int) error {
	key := testutil.MakeKey(txid, outputIndex)
	return m.Delete(key)
}

func (m *MockAppsStorage) FindByDomain(ctx context.Context, domain string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	matches := m.Filter(func(record AppCatalogRecord) bool {
		return record.Metadata.Domain == domain
	})
	return m.toUTXOReferences(matches, limit, skip), nil
}

func (m *MockAppsStorage) FindByPublisher(ctx context.Context, publisher string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	matches := m.Filter(func(record AppCatalogRecord) bool {
		return record.Metadata.Publisher == publisher
	})
	return m.toUTXOReferences(matches, limit, skip), nil
}

func (m *MockAppsStorage) FindByOutpoint(ctx context.Context, outpoint string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	txid, outputIndex, err := testutil.ParseOutpoint(outpoint)
	if err != nil {
		return nil, err
	}

	key := testutil.MakeKey(txid, outputIndex)
	if record, ok := m.Get(key); ok {
		return []UTXOReference{
			{
				Txid:        record.Txid,
				OutputIndex: record.OutputIndex,
			},
		}, nil
	}
	return []UTXOReference{}, nil
}

func (m *MockAppsStorage) FindByNameFuzzy(ctx context.Context, partialName string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	lowerPartialName := strings.ToLower(partialName)
	matches := m.Filter(func(record AppCatalogRecord) bool {
		return strings.Contains(strings.ToLower(record.Metadata.Name), lowerPartialName)
	})
	return m.toUTXOReferences(matches, limit, skip), nil
}

func (m *MockAppsStorage) FindByTags(ctx context.Context, tags []string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	matches := m.Filter(func(record AppCatalogRecord) bool {
		for _, tag := range tags {
			if testutil.Contains(record.Metadata.Tags, tag) {
				return true
			}
		}
		return false
	})
	return m.toUTXOReferences(matches, limit, skip), nil
}

func (m *MockAppsStorage) FindByCategory(ctx context.Context, category string, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	matches := m.Filter(func(record AppCatalogRecord) bool {
		return record.Metadata.Category == category
	})
	return m.toUTXOReferences(matches, limit, skip), nil
}

func (m *MockAppsStorage) FindAllApps(ctx context.Context, limit, skip int, sortOrder string) ([]UTXOReference, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	matches := m.GetAll()
	return m.toUTXOReferences(matches, limit, skip), nil
}

func (m *MockAppsStorage) toUTXOReferences(records []AppCatalogRecord, limit, skip int) []UTXOReference {
	if skip >= len(records) {
		return []UTXOReference{}
	}
	records = records[skip:]
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	results := make([]UTXOReference, len(records))
	for i, record := range records {
		results[i] = UTXOReference{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		}
	}
	return results
}

func TestAppsLookupService_NewInstance(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)
	require.NotNil(t, ls)
	require.NotNil(t, ls.storage)
}

func TestAppsLookupService_GetDocumentation(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)
	docs := ls.GetDocumentation()
	assert.Contains(t, docs, "Apps Lookup Service")
	assert.Contains(t, docs, "ls_apps")
}

func TestAppsLookupService_GetMetaData(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)
	meta := ls.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Apps Lookup Service", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestAppsLookupService_Lookup_NilQuestion(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)
	answer, err := ls.Lookup(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, answer)
}

func TestAppsLookupService_Lookup_WrongService(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_wrong",
		Query:   testutil.MakeQuery(map[string]interface{}{"domain": "example.com"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	// The service doesn't check service name, it just processes the query
}

func TestAppsLookupService_Lookup_EmptyQuery(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   testutil.MakeQuery(map[string]interface{}{}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestAppsLookupService_Lookup_ByDomain(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

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
	err := storage.StoreRecord(context.Background(), "txid123", 0, metadata)
	require.NoError(t, err)

	// Lookup by domain
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   testutil.MakeQuery(map[string]interface{}{"domain": "example.com"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "txid123", results[0].Txid)
	assert.Equal(t, 0, results[0].OutputIndex)
}

func TestAppsLookupService_Lookup_ByPublisher(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

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
	err := storage.StoreRecord(context.Background(), "txid1", 0, metadata1)
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
	err = storage.StoreRecord(context.Background(), "txid2", 0, metadata2)
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
	err = storage.StoreRecord(context.Background(), "txid3", 0, metadata3)
	require.NoError(t, err)

	// Lookup by publisher
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   testutil.MakeQuery(map[string]interface{}{"publisher": publisherKey}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestAppsLookupService_Lookup_ByName(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

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
	err := storage.StoreRecord(context.Background(), "txid456", 0, metadata)
	require.NoError(t, err)

	// Lookup by name (fuzzy)
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   testutil.MakeQuery(map[string]interface{}{"name": "calc"}),
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
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

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
	err := storage.StoreRecord(context.Background(), "txid789", 2, metadata)
	require.NoError(t, err)

	// Lookup by outpoint
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   testutil.MakeQuery(map[string]interface{}{"outpoint": "txid789.2"}),
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
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

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
	err := storage.StoreRecord(context.Background(), "txid1", 0, metadata1)
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
	err = storage.StoreRecord(context.Background(), "txid2", 0, metadata2)
	require.NoError(t, err)

	// Lookup by tags
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   testutil.MakeQuery(map[string]interface{}{"tags": []string{"productivity"}}),
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
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

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
	err := storage.StoreRecord(context.Background(), "txid1", 0, metadata1)
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
	err = storage.StoreRecord(context.Background(), "txid2", 0, metadata2)
	require.NoError(t, err)

	// Lookup by category
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   testutil.MakeQuery(map[string]interface{}{"category": "Games"}),
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
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

	// Lookup non-existent domain
	question := &lookup.LookupQuestion{
		Service: "ls_apps",
		Query:   testutil.MakeQuery(map[string]interface{}{"domain": "nonexistent.com"}),
	}
	answer, err := ls.Lookup(context.Background(), question)
	require.NoError(t, err)
	require.NotNil(t, answer)
	assert.Equal(t, lookup.AnswerTypeOutputList, answer.Type)

	results, ok := answer.Result.([]UTXOReference)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestAppsLookupService_OutputSpent(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

	// Store a record first
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
	err := storage.StoreRecord(context.Background(), txidHex, 1, metadata)
	require.NoError(t, err)

	// Verify it exists
	results, err := storage.FindByOutpoint(context.Background(), txidHex+".1")
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Mark as spent
	txidHash := testutil.MakeHashFromHex(txidHex)
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
	results, err = storage.FindByOutpoint(context.Background(), txidHex+".1")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAppsLookupService_OutputSpent_WrongTopic(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

	// Store a record first
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
	err := storage.StoreRecord(context.Background(), txidHex, 0, metadata)
	require.NoError(t, err)

	// Try to mark as spent with wrong topic
	txidHash := testutil.MakeHashFromHex(txidHex)
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
	results, err := storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestAppsLookupService_OutputEvicted(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

	// Store a record first
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
	err := storage.StoreRecord(context.Background(), txidHex, 0, metadata)
	require.NoError(t, err)

	// Evict the output
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputEvicted(context.Background(), outpoint)
	require.NoError(t, err)

	// Verify it's deleted
	results, err := storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAppsLookupService_OutputNoLongerRetainedInHistory(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

	// Store a record first
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
	err := storage.StoreRecord(context.Background(), txidHex, 0, metadata)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_apps")
	require.NoError(t, err)

	// Verify it's deleted
	results, err := storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAppsLookupService_OutputNoLongerRetainedInHistory_WrongTopic(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

	// Store a record first
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
	err := storage.StoreRecord(context.Background(), txidHex, 0, metadata)
	require.NoError(t, err)

	// Call OutputNoLongerRetainedInHistory with wrong topic
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: 0,
	}
	err = ls.OutputNoLongerRetainedInHistory(context.Background(), outpoint, "tm_other")
	require.NoError(t, err)

	// Verify it still exists (was not deleted)
	results, err := storage.FindByOutpoint(context.Background(), txidHex+".0")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestAppsLookupService_OutputBlockHeightUpdated(t *testing.T) {
	storage := NewMockAppsStorage()
	ls := NewAppsLookupServiceWithStorage(storage)

	// This is a no-op for Apps, just verify it doesn't error
	txidHex := "0000000000000000000000000000000000000000000000000000000000000002"
	txidHash := testutil.MakeHashFromHex(txidHex)
	require.NotNil(t, txidHash)

	err := ls.OutputBlockHeightUpdated(context.Background(), txidHash, 12345, 0)
	require.NoError(t, err)
}
