package uhrp

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
	"github.com/bsv-blockchain/go-sdk/storage"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"go.mongodb.org/mongo-driver/mongo"
)

const lookupDocs = `
# Universal Hash Resolution Protocol Lookup Service

To use this service, send a query that comprises an outpoint, uhrpUrl, expiryTime, fileSize, and/or hostIdentityKey.

The associated token will be returned.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by UHRP URL (generated from file hash)
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_uhrp",
    Query: map[string]interface{}{
        "uhrpUrl": "uhrp://abc123...",
    },
}, 10000)

// Find by host identity key
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_uhrp",
    Query: map[string]interface{}{
        "hostIdentityKey": "02abc...",
    },
}, 10000)

// Find by outpoint
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_uhrp",
    Query: map[string]interface{}{
        "outpoint": "txid.0",
    },
}, 10000)
` + "```" + `
`

// UHRPLookupService implements a lookup service for the UHRP protocol
type UHRPLookupService struct {
	storage UHRPStorageEngine
}

// NewUHRPLookupService creates a new UHRPLookupService instance
func NewUHRPLookupService(db *mongo.Database) *UHRPLookupService {
	return &UHRPLookupService{
		storage: NewUHRPStorage(db),
	}
}

// NewUHRPLookupServiceWithStorage creates a new UHRPLookupService with a custom storage engine
func NewUHRPLookupServiceWithStorage(storage UHRPStorageEngine) *UHRPLookupService {
	return &UHRPLookupService{
		storage: storage,
	}
}

// Ensure UHRPLookupService implements engine.LookupService
var _ engine.LookupService = (*UHRPLookupService)(nil)

// OutputAdmittedByTopic is invoked when a new output is added to the overlay
func (ls *UHRPLookupService) OutputAdmittedByTopic(ctx context.Context, payload *engine.OutputAdmittedByTopic) error {
	if payload.Topic != "tm_uhrp" {
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
	if result == nil || len(result.Fields) < 6 {
		err := fmt.Errorf("invalid UHRP token: invalid PushDrop structure")
		slog.Error("UHRPLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	// Extract UHRP advertisement fields
	hostIdentityKeyBuf := result.Fields[0]
	hashBuf := result.Fields[1]
	hostedFileLocationBuf := result.Fields[2]
	expiryTimeBuf := result.Fields[3]
	fileSizeBuf := result.Fields[4]

	// Convert identity key to hex
	hostIdentityKey := hex.EncodeToString(hostIdentityKeyBuf)

	// Generate UHRP URL from hash
	uhrpUrl, err := storage.GetURLForHash(hashBuf)
	if err != nil {
		err := fmt.Errorf("failed to generate UHRP URL: %w", err)
		slog.Error("UHRPLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	// Decode file location
	hostedFileLocation := string(hostedFileLocationBuf)

	// Parse expiry time (varint)
	expiryTime, err := readVarInt(expiryTimeBuf)
	if err != nil {
		err := fmt.Errorf("failed to parse expiry time: %w", err)
		slog.Error("UHRPLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	// Parse file size (varint)
	fileSize, err := readVarInt(fileSizeBuf)
	if err != nil {
		err := fmt.Errorf("failed to parse file size: %w", err)
		slog.Error("UHRPLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	slog.Debug("Decoded UHRP advertisement", "uhrpUrl", uhrpUrl, "location", hostedFileLocation, "expiry", expiryTime, "size", fileSize)

	// Store the advertisement
	err = ls.storage.StoreRecord(uhrpUrl, txid, outputIndex, hostIdentityKey, hostedFileLocation, expiryTime, fileSize)
	if err != nil {
		slog.Error("UHRPLookupService: failed to store record", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	return nil
}

// OutputSpent is invoked when a UTXO is spent
func (ls *UHRPLookupService) OutputSpent(ctx context.Context, payload *engine.OutputSpent) error {
	if payload.Topic != "tm_uhrp" {
		return nil
	}

	txid := payload.Outpoint.Txid.String()
	outputIndex := int(payload.Outpoint.Index)

	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputNoLongerRetainedInHistory is called when historical retention is no longer required
func (ls *UHRPLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	// For the UHRP service, we don't need to do anything special here
	return nil
}

// OutputEvicted permanently removes the referenced UTXO from all indices
func (ls *UHRPLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)
	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputBlockHeightUpdated is called when the block height of an output is updated
func (ls *UHRPLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIndex uint64) error {
	// For the UHRP service, we don't track block heights
	return nil
}

// Lookup answers a lookup query
func (ls *UHRPLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	if question.Service != "ls_uhrp" {
		return nil, fmt.Errorf("unsupported lookup service: %s", question.Service)
	}

	// Parse query into UHRPQuery struct
	queryBytes, err := json.Marshal(question.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	var query UHRPQuery
	if err := json.Unmarshal(queryBytes, &query); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query: %w", err)
	}

	// Execute query
	results, err := ls.storage.Lookup(&query)
	if err != nil {
		return nil, err
	}

	// Return results as LookupAnswer
	return &lookup.LookupAnswer{
		Type:   "output-list",
		Result: results,
	}, nil
}

// GetDocumentation returns the documentation for this lookup service
func (ls *UHRPLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *UHRPLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "UHRP Lookup Service",
		Description: "Lookup Service for User file hosting commitment tokens",
	}
}
