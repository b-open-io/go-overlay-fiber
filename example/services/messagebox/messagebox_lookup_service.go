package messagebox

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"go.mongodb.org/mongo-driver/mongo"
)

const lookupDocs = `
# MessageBox Lookup Service

The **MessageBox Lookup Service** is a SHIP-compatible overlay service that maps identity keys to MessageBox hosts. It enables identity-based message routing in the MessageBox ecosystem by resolving which hosts have been anointed to receive messages for a given identity.

## Overview

This service listens for advertisements broadcast to the ` + "`tm_messagebox`" + ` topic. Each advertisement contains a digitally signed payload that proves an identity key has anointed a particular host to receive its messages.

When queried, this service can return the list of hosts associated with an identity key, enabling clients to route messages dynamically based on the overlay network.

## Behavior

### On Output Addition

When an advertisement output is added, the following steps occur:

1. The output is decoded using PushDrop format.
2. The fields extracted include:
   - Identity Key
   - Host URL
3. The signature is validated to ensure the advertisement was authorized by the identity key.
4. The advertisement is saved to the internal database.

### On Output Spend or Deletion

When a matching output is spent or deleted, the associated advertisement is removed from the database.

## Lookup Support

### Service Name

` + "`ls_messagebox`" + `

### Query Format

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find all hosts for an identity key
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_messagebox",
    Query: map[string]interface{}{
        "identityKey": "02abc1234567890def1234567890abc1234567890def1234567890abc1234567890",
    },
}, 10000)

// Find specific host for an identity key
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_messagebox",
    Query: map[string]interface{}{
        "identityKey": "02abc1234567890def1234567890abc1234567890def1234567890abc1234567890",
        "host":        "https://alice-messagebox.example.com",
    },
}, 10000)
` + "```" + `

### Response Format

Returns array of UTXO references (txid + outputIndex) ordered by recency.
`

// MessageBoxLookupService implements a lookup service for the MessageBox protocol
type MessageBoxLookupService struct {
	storage MessageBoxStorageEngine
}

// NewMessageBoxLookupService creates a new MessageBoxLookupService instance
func NewMessageBoxLookupService(db *mongo.Database) *MessageBoxLookupService {
	return &MessageBoxLookupService{
		storage: NewMessageBoxStorage(db),
	}
}

// NewMessageBoxLookupServiceWithStorage creates a new MessageBoxLookupService with a custom storage engine
func NewMessageBoxLookupServiceWithStorage(storage MessageBoxStorageEngine) *MessageBoxLookupService {
	return &MessageBoxLookupService{
		storage: storage,
	}
}

// Ensure MessageBoxLookupService implements engine.LookupService
var _ engine.LookupService = (*MessageBoxLookupService)(nil)

// OutputAdmittedByTopic is invoked when a new output is added to the overlay
func (ls *MessageBoxLookupService) OutputAdmittedByTopic(ctx context.Context, payload *engine.OutputAdmittedByTopic) error {
	if payload.Topic != "tm_messagebox" {
		return nil
	}

	// Parse the AtomicBEEF to get the transaction
	tx, err := transaction.NewTransactionFromBEEF(payload.AtomicBEEF)
	if err != nil {
		return fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := tx.TxID().String()
	outputIndex := int(payload.OutputIndex)

	// Get the locking script from the transaction output
	lockingScript := tx.Outputs[payload.OutputIndex].LockingScript

	// Decode the PushDrop token
	result := pushdrop.Decode(lockingScript)
	if result == nil || len(result.Fields) < 3 {
		err := fmt.Errorf("invalid MessageBox token: invalid PushDrop structure")
		slog.Error("MessageBoxLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	// Extract identity key and host
	identityKeyBuf := result.Fields[0]
	hostBuf := result.Fields[1]

	if len(identityKeyBuf) == 0 || len(hostBuf) == 0 {
		err := fmt.Errorf("invalid MessageBox token: empty fields")
		slog.Error("MessageBoxLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	identityKey := hex.EncodeToString(identityKeyBuf)
	host := string(hostBuf)

	slog.Debug("MessageBox decoded advertisement", "identityKey", identityKey, "host", host)

	// Store the advertisement
	err = ls.storage.StoreRecord(identityKey, host, txid, outputIndex)
	if err != nil {
		slog.Error("MessageBoxLookupService: failed to store record", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	return nil
}

// OutputSpent is invoked when a UTXO is spent
func (ls *MessageBoxLookupService) OutputSpent(ctx context.Context, payload *engine.OutputSpent) error {
	if payload.Topic != "tm_messagebox" {
		return nil
	}

	txid := payload.Outpoint.Txid.String()
	outputIndex := int(payload.Outpoint.Index)

	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputNoLongerRetainedInHistory is called when historical retention is no longer required
func (ls *MessageBoxLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	// For the MessageBox service, we don't need to do anything special here
	return nil
}

// OutputEvicted permanently removes the referenced UTXO from all indices
func (ls *MessageBoxLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)
	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputBlockHeightUpdated is called when the block height of an output is updated
func (ls *MessageBoxLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIndex uint64) error {
	// For the MessageBox service, we don't track block heights
	return nil
}

// Lookup answers a lookup query
func (ls *MessageBoxLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	if question.Service != "ls_messagebox" {
		return nil, fmt.Errorf("unsupported lookup service: %s", question.Service)
	}

	// Parse query
	var query MessageBoxQuery
	queryBytes, err := json.Marshal(question.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}
	if err := json.Unmarshal(queryBytes, &query); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query: %w", err)
	}

	// Validate query
	if query.IdentityKey == "" {
		return nil, fmt.Errorf("identityKey query missing")
	}

	// Execute query
	results, err := ls.storage.FindAdvertisements(query.IdentityKey, query.Host)
	if err != nil {
		return nil, err
	}

	// Return results as LookupAnswer
	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeOutputList,
		Result: results,
	}, nil
}

// GetDocumentation returns the documentation for this lookup service
func (ls *MessageBoxLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *MessageBoxLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "MessageBox Lookup Service",
		Description: "Lookup overlay hosts for identity keys (MessageBox)",
	}
}
