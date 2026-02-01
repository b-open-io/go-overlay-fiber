package testutil

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// MakeQuery creates a json.RawMessage from a map for use in lookup tests
func MakeQuery(m map[string]interface{}) json.RawMessage {
	data, _ := json.Marshal(m)
	return data
}

// MakeHashFromHex creates a chainhash.Hash from a hex string, padding if necessary
func MakeHashFromHex(hexStr string) *chainhash.Hash {
	// Pad to 64 characters (32 bytes)
	for len(hexStr) < 64 {
		hexStr = "0" + hexStr
	}
	hash, _ := chainhash.NewHashFromHex(hexStr)
	return hash
}

// MakeKey creates a composite key from txid and outputIndex for mock storage
func MakeKey(txid string, outputIndex int) string {
	return txid + ":" + strconv.Itoa(outputIndex)
}

// Contains checks if a string slice contains a specific string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ParseOutpoint parses an outpoint string in the format "txid.outputIndex"
// and returns the txid and output index. This is a common pattern used
// by many lookup services.
func ParseOutpoint(outpoint string) (string, int, error) {
	if outpoint == "" {
		return "", 0, fmt.Errorf("empty outpoint")
	}

	parts := strings.Split(outpoint, ".")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid outpoint format, expected txid.outputIndex")
	}

	txid := parts[0]
	if txid == "" {
		return "", 0, fmt.Errorf("empty txid in outpoint")
	}

	outputIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid output index in outpoint: %w", err)
	}

	if outputIndex < 0 {
		return "", 0, fmt.Errorf("negative output index in outpoint")
	}

	return txid, outputIndex, nil
}

// UTXOReference is a common type for lookup results
type UTXOReference struct {
	Txid        string
	OutputIndex int
}

// MockStorageBase provides a generic, thread-safe mock storage for testing.
// Services can embed this and add their specific lookup/filter logic.
type MockStorageBase[T any] struct {
	mu      sync.RWMutex
	records map[string]T

	// Error injection for testing error paths
	StoreError  error
	DeleteError error
	LookupError error
}

// NewMockStorageBase creates a new generic mock storage
func NewMockStorageBase[T any]() *MockStorageBase[T] {
	return &MockStorageBase[T]{
		records: make(map[string]T),
	}
}

// Store adds or updates a record in the storage
func (m *MockStorageBase[T]) Store(key string, record T) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.StoreError != nil {
		return m.StoreError
	}
	m.records[key] = record
	return nil
}

// Delete removes a record from the storage
func (m *MockStorageBase[T]) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.DeleteError != nil {
		return m.DeleteError
	}
	delete(m.records, key)
	return nil
}

// Get retrieves a record by key
func (m *MockStorageBase[T]) Get(key string) (T, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.records[key]
	return record, ok
}

// GetAll returns all records
func (m *MockStorageBase[T]) GetAll() []T {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]T, 0, len(m.records))
	for _, record := range m.records {
		results = append(results, record)
	}
	return results
}

// Filter returns records that match the predicate
func (m *MockStorageBase[T]) Filter(predicate func(T) bool) []T {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []T
	for _, record := range m.records {
		if predicate(record) {
			results = append(results, record)
		}
	}
	return results
}

// Count returns the number of records
func (m *MockStorageBase[T]) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.records)
}

// Clear removes all records
func (m *MockStorageBase[T]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.records = make(map[string]T)
}
