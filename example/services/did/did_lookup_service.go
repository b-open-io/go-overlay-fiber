package did

import (
	"context"
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

const lookupDocs = `# DID Lookup Service Documentation

The DID Lookup Service is responsible for managing the rules of admissibility for DID tokens and handling queries related to them.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by serial number
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_did",
    Query: map[string]interface{}{
        "serialNumber": "abc123...",
    },
}, 10000)

// Find by outpoint
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_did",
    Query: map[string]interface{}{
        "outpoint": "txid.0",
    },
}, 10000)
` + "```" + `
`

// DIDLookupService implements a lookup service for DID tokens
type DIDLookupService struct {
	storage *DIDStorage
}

// NewDIDLookupService creates a new DIDLookupService instance
func NewDIDLookupService(db *mongo.Database) *DIDLookupService {
	return &DIDLookupService{
		storage: NewDIDStorage(db),
	}
}

// Ensure DIDLookupService implements engine.LookupService
var _ engine.LookupService = (*DIDLookupService)(nil)

// OutputAdmittedByTopic is invoked when a new output is added to the overlay
func (ls *DIDLookupService) OutputAdmittedByTopic(ctx context.Context, payload *engine.OutputAdmittedByTopic) error {
	if payload.Topic != "tm_did" {
		return nil
	}

	// Parse the AtomicBEEF to get the transaction
	tx, err := transaction.NewTransactionFromBEEF(payload.AtomicBEEF)
	if err != nil {
		return fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := tx.TxID().String()
	outputIndex := int(payload.OutputIndex)

	slog.Debug("DID lookup service output admitted", "txid", txid, "outputIndex", outputIndex)

	// Get the locking script from the transaction output
	lockingScript := tx.Outputs[payload.OutputIndex].LockingScript

	// Decode the DID token fields from the Bitcoin outputScript
	result := pushdrop.Decode(lockingScript)
	if result == nil || len(result.Fields) < 2 {
		err := fmt.Errorf("invalid DID token: invalid PushDrop structure")
		slog.Error("DIDLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	// Extract serial number (convert to UTF-8 string)
	serialNumber := string(result.Fields[0])

	slog.Debug("DID lookup service storing record", "txid", txid, "outputIndex", outputIndex, "serialNumber", serialNumber)

	// Store DID record
	err = ls.storage.StoreRecord(txid, outputIndex, serialNumber)
	if err != nil {
		slog.Error("DIDLookupService: failed to store record", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	return nil
}

// OutputSpent is invoked when a UTXO is spent
func (ls *DIDLookupService) OutputSpent(ctx context.Context, payload *engine.OutputSpent) error {
	if payload.Topic != "tm_did" {
		return nil
	}

	txid := payload.Outpoint.Txid.String()
	outputIndex := int(payload.Outpoint.Index)

	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputNoLongerRetainedInHistory is called when historical retention is no longer required
func (ls *DIDLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	// For the DID service, we don't need to do anything special here
	return nil
}

// OutputEvicted permanently removes the referenced UTXO from all indices
func (ls *DIDLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)
	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputBlockHeightUpdated is called when the block height of an output is updated
func (ls *DIDLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIndex uint64) error {
	// For the DID service, we don't track block heights
	return nil
}

// Lookup answers a lookup query
func (ls *DIDLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	slog.Debug("DID lookup query received", "service", question.Service, "query", question.Query)

	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	if question.Service != "ls_did" {
		return nil, fmt.Errorf("unsupported lookup service: %s", question.Service)
	}

	// Parse query into DIDQuery struct
	queryBytes, err := json.Marshal(question.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	var query DIDQuery
	if err := json.Unmarshal(queryBytes, &query); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query: %w", err)
	}

	var results []UTXOReference

	// Find by serial number
	if query.SerialNumber != "" {
		results, err = ls.storage.FindByCertificateSerialNumber(query.SerialNumber)
		if err != nil {
			return nil, err
		}
		return &lookup.LookupAnswer{
			Type:   "output-list",
			Result: results,
		}, nil
	}

	// Find by outpoint
	if query.Outpoint != "" {
		results, err = ls.storage.FindByOutpoint(query.Outpoint)
		if err != nil {
			return nil, err
		}
		return &lookup.LookupAnswer{
			Type:   "output-list",
			Result: results,
		}, nil
	}

	return nil, fmt.Errorf("no valid query parameters provided")
}

// GetDocumentation returns the documentation for this lookup service
func (ls *DIDLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *DIDLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "DID Lookup Service",
		Description: "DID resolution made easy.",
	}
}
