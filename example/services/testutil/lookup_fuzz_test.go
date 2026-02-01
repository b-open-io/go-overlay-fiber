package testutil

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLookupService is a minimal lookup service implementation for fuzz testing.
// It exercises common JSON parsing and query handling patterns.
type MockLookupService struct{}

func (m *MockLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, &LookupError{Message: "nil question"}
	}

	// Parse query as generic map to exercise JSON parsing
	var query map[string]interface{}
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, &LookupError{Message: "invalid query JSON: " + err.Error()}
	}

	// Handle outpoint queries (common pattern across services)
	if outpoint, ok := query["outpoint"].(string); ok {
		if _, _, err := ParseOutpoint(outpoint); err != nil {
			return nil, err
		}
	}

	// Return empty result for valid queries
	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeOutputList,
		Result: []UTXOReference{},
	}, nil
}

// LookupError is a simple error type for lookup operations
type LookupError struct {
	Message string
}

func (e *LookupError) Error() string {
	return e.Message
}

// FuzzParseQueryJSON tests lookup services with random JSON query inputs
// to ensure they handle malformed and edge-case JSON gracefully.
func FuzzParseQueryJSON(f *testing.F) {
	// Seed corpus with valid query JSON examples
	f.Add(`{"outpoint": "txid123.0"}`)
	f.Add(`{"field": "value"}`)
	f.Add(`{"numericField": 12345}`)
	f.Add(`{"boolField": true}`)
	f.Add(`{"nested": {"key": "value"}}`)

	// Seed corpus with invalid/edge-case JSON
	f.Add(`{}`)
	f.Add(`null`)
	f.Add(`"string"`)
	f.Add(`123`)
	f.Add(`true`)
	f.Add(`[1, 2, 3]`)
	f.Add(`{"field": null}`)
	f.Add(`{"outpoint": 123}`)

	// Seed corpus with edge cases
	f.Add(`{"field": ""}`)
	f.Add(`{"outpoint": ""}`)
	f.Add(`{"outpoint": "invalid"}`)
	f.Add(`{"outpoint": "txid.abc"}`)

	service := &MockLookupService{}

	f.Fuzz(func(t *testing.T, jsonStr string) {
		// First, try to unmarshal to ensure it's valid JSON
		var queryInterface interface{}
		err := json.Unmarshal([]byte(jsonStr), &queryInterface)
		if err != nil {
			// Invalid JSON should be rejected, but shouldn't panic
			return
		}

		// Create a lookup question with the fuzzed query
		question := &lookup.LookupQuestion{
			Service: "ls_test",
			Query:   json.RawMessage(jsonStr),
		}

		// Function should not panic on any input
		_, err = service.Lookup(context.Background(), question)

		// We don't validate the result or error, just ensure no panic occurs
		_ = err
	})
}

// FuzzOutpointFormat tests the outpoint parsing with various formats.
// This is a common pattern used by many lookup services.
func FuzzOutpointFormat(f *testing.F) {
	// Seed corpus with various outpoint formats
	f.Add("txid123.0")
	f.Add("abc.1")
	f.Add("1234567890abcdef.99")
	f.Add("")
	f.Add(".")
	f.Add("txid.")
	f.Add(".0")
	f.Add("txid.abc")
	f.Add("txid.-1")
	f.Add("txid.0.extra")
	f.Add("txid")
	f.Add("0")

	// Very long txid (64 hex chars)
	longTxid := ""
	for i := 0; i < 64; i++ {
		longTxid += "a"
	}
	f.Add(longTxid + ".0")

	service := &MockLookupService{}

	f.Fuzz(func(t *testing.T, outpoint string) {
		query := map[string]interface{}{
			"outpoint": outpoint,
		}
		queryJSON, err := json.Marshal(query)
		if err != nil {
			return
		}

		question := &lookup.LookupQuestion{
			Service: "ls_test",
			Query:   queryJSON,
		}

		// Function should not panic on any input
		_, err = service.Lookup(context.Background(), question)

		// We don't validate the error, just ensure no panic occurs
		_ = err
	})
}

// FuzzNumericParameters tests numeric parameter handling in queries
func FuzzNumericParameters(f *testing.F) {
	// Seed corpus with various numeric values
	f.Add(uint64(0), uint64(0))
	f.Add(uint64(1), uint64(1024))
	f.Add(uint64(1735689600), uint64(1024*1024*1024))
	f.Add(uint64(18446744073709551615), uint64(18446744073709551615)) // Max uint64

	service := &MockLookupService{}

	f.Fuzz(func(t *testing.T, param1, param2 uint64) {
		query := map[string]interface{}{
			"numericParam1": param1,
			"numericParam2": param2,
		}
		queryJSON, err := json.Marshal(query)
		if err != nil {
			return
		}

		question := &lookup.LookupQuestion{
			Service: "ls_test",
			Query:   queryJSON,
		}

		// Function should not panic on any input
		_, err = service.Lookup(context.Background(), question)

		// We don't validate the error, just ensure no panic occurs
		_ = err
	})
}

// FuzzStringParameters tests string parameter handling in queries
func FuzzStringParameters(f *testing.F) {
	// Seed corpus with various string values
	f.Add("normal_string")
	f.Add("")
	f.Add("string with spaces")
	f.Add("string\nwith\nnewlines")
	f.Add("string\twith\ttabs")
	f.Add("unicode: 日本語 emoji: 🎉")
	f.Add("special: <>&\"'")

	// Very long string
	longStr := ""
	for i := 0; i < 1000; i++ {
		longStr += "a"
	}
	f.Add(longStr)

	service := &MockLookupService{}

	f.Fuzz(func(t *testing.T, param string) {
		query := map[string]interface{}{
			"stringParam": param,
		}
		queryJSON, err := json.Marshal(query)
		if err != nil {
			return
		}

		question := &lookup.LookupQuestion{
			Service: "ls_test",
			Query:   queryJSON,
		}

		// Function should not panic on any input
		_, err = service.Lookup(context.Background(), question)

		// We don't validate the error, just ensure no panic occurs
		_ = err
	})
}

// TestRecord is a simple record type for testing MockStorageBase
type TestRecord struct {
	ID    string
	Value string
	Count int
}

// TestMockStorageBase_ConcurrentStore tests thread safety of concurrent store operations
func TestMockStorageBase_ConcurrentStore(t *testing.T) {
	storage := NewMockStorageBase[TestRecord]()

	var wg sync.WaitGroup
	numGoroutines := 10
	recordsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				key := MakeKey(string(rune('a'+idx)), j)
				err := storage.Store(key, TestRecord{
					ID:    key,
					Value: "value",
					Count: j,
				})
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all records were stored
	assert.Equal(t, numGoroutines*recordsPerGoroutine, storage.Count())
}

// TestMockStorageBase_ConcurrentStoreAndRead tests thread safety of concurrent store and read operations
func TestMockStorageBase_ConcurrentStoreAndRead(t *testing.T) {
	storage := NewMockStorageBase[TestRecord]()

	// Pre-populate some data
	for i := 0; i < 50; i++ {
		key := MakeKey("init", i)
		err := storage.Store(key, TestRecord{ID: key, Value: "initial", Count: i})
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 200)

	// Writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				key := MakeKey("writer"+string(rune('0'+idx)), j)
				err := storage.Store(key, TestRecord{ID: key, Value: "written", Count: j})
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				// Read from pre-populated data
				key := MakeKey("init", j%50)
				_, _ = storage.Get(key)

				// Also try GetAll
				_ = storage.GetAll()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent store/read error: %v", err)
	}
}

// TestMockStorageBase_ConcurrentDelete tests thread safety of concurrent delete operations
func TestMockStorageBase_ConcurrentDelete(t *testing.T) {
	storage := NewMockStorageBase[TestRecord]()

	// Pre-populate data
	for i := 0; i < 100; i++ {
		key := MakeKey("delete", i)
		err := storage.Store(key, TestRecord{ID: key, Value: "to_delete", Count: i})
		require.NoError(t, err)
	}

	var wg sync.WaitGroup

	// Delete concurrently - some goroutines will try to delete the same keys
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				// Each goroutine deletes overlapping ranges
				key := MakeKey("delete", (idx*10+j)%100)
				_ = storage.Delete(key)
			}
		}(i)
	}

	wg.Wait()

	// Some records should be deleted (exact count depends on overlap)
	assert.Less(t, storage.Count(), 100)
}

// TestMockStorageBase_ConcurrentFilter tests thread safety of concurrent filter operations
func TestMockStorageBase_ConcurrentFilter(t *testing.T) {
	storage := NewMockStorageBase[TestRecord]()

	// Pre-populate data with different counts
	for i := 0; i < 100; i++ {
		key := MakeKey("filter", i)
		err := storage.Store(key, TestRecord{ID: key, Value: "filterable", Count: i % 10})
		require.NoError(t, err)
	}

	var wg sync.WaitGroup

	// Run concurrent filters
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				targetCount := idx
				results := storage.Filter(func(r TestRecord) bool {
					return r.Count == targetCount
				})
				// Each count value should appear 10 times (100 records / 10 possible counts)
				assert.Equal(t, 10, len(results))
			}
		}(i)
	}

	wg.Wait()
}

// TestMockStorageBase_ConcurrentMixedOperations tests thread safety with all operations running concurrently
func TestMockStorageBase_ConcurrentMixedOperations(t *testing.T) {
	storage := NewMockStorageBase[TestRecord]()

	var wg sync.WaitGroup

	// Store operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			key := MakeKey("mixed", i)
			_ = storage.Store(key, TestRecord{ID: key, Value: "mixed", Count: i})
		}
	}()

	// Get operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			key := MakeKey("mixed", i%50)
			_, _ = storage.Get(key)
		}
	}()

	// Delete operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 25; i++ {
			key := MakeKey("mixed", i)
			_ = storage.Delete(key)
		}
	}()

	// Filter operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = storage.Filter(func(r TestRecord) bool {
				return r.Count > 25
			})
		}
	}()

	// GetAll operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = storage.GetAll()
		}
	}()

	// Count operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = storage.Count()
		}
	}()

	wg.Wait()

	// Just verify no panics occurred - exact state depends on timing
}
