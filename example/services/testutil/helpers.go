package testutil

import (
	"encoding/json"
	"strconv"

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
