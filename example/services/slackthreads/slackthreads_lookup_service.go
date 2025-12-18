package slackthreads

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"go.mongodb.org/mongo-driver/mongo"
)

const lookupDocs = `
# SlackThread Lookup Service Documentation

The **SlackThread Lookup Service** (service ID: ` + "`ls_slackthread`" + `) lets clients search the on-chain *SlackThread* messages that are indexed by the **SlackThread Topic Manager**. Each record represents a Pay-to-Push-Drop output whose single field is a UTF-8 message of at least two characters.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// find all
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_slackthread",
    Query:   map[string]interface{}{},
}, 10000)

// find by thread hash
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_slackthread",
    Query: map[string]interface{}{
        "threadHash": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    },
}, 10000)

// find by txid
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_slackthread",
    Query: map[string]interface{}{
        "txid": "some txid",
    },
}, 10000)
` + "```" + `
`

// SlackThreadLookupService implements a lookup service for the SlackThread protocol.
// Each admitted output stores exactly one 32-byte hash in the locking script.
// This service indexes those thread hashes so they can be queried later.
type SlackThreadLookupService struct {
	storage SlackThreadsStorageEngine
}

// NewSlackThreadLookupService creates a new SlackThreadLookupService instance
func NewSlackThreadLookupService(db *mongo.Database) *SlackThreadLookupService {
	return &SlackThreadLookupService{
		storage: NewSlackThreadsStorage(db),
	}
}

// NewSlackThreadsLookupServiceWithStorage creates a new SlackThreadLookupService with a custom storage engine
func NewSlackThreadsLookupServiceWithStorage(storage SlackThreadsStorageEngine) *SlackThreadLookupService {
	return &SlackThreadLookupService{
		storage: storage,
	}
}

// Ensure SlackThreadLookupService implements engine.LookupService
var _ engine.LookupService = (*SlackThreadLookupService)(nil)

// OutputAdmittedByTopic is invoked when a new output is added to the overlay
func (ls *SlackThreadLookupService) OutputAdmittedByTopic(ctx context.Context, payload *engine.OutputAdmittedByTopic) error {
	if payload.Topic != "tm_slackthread" {
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

	// Extract the hash from the locking script
	chunks, err := lockingScript.ParseOps()
	if err != nil {
		err := fmt.Errorf("failed to parse script chunks: %w", err)
		slog.Error("SlackThreadLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	if len(chunks) != 3 {
		err := fmt.Errorf("invalid SlackThread token: expected 3 chunks, got %d", len(chunks))
		slog.Error("SlackThreadLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	// Extract the hash from chunk[1].Data
	threadHashBytes := chunks[1].Data
	if len(threadHashBytes) != 32 {
		err := fmt.Errorf("invalid SlackThread token: thread hash must be exactly 32 bytes, got %d", len(threadHashBytes))
		slog.Error("SlackThreadLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	// Convert to hex string
	threadHashString := hex.EncodeToString(threadHashBytes)

	// Persist for future lookup
	err = ls.storage.StoreRecord(txid, outputIndex, threadHashString)
	if err != nil {
		slog.Error("SlackThreadLookupService: failed to store record", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	return nil
}

// OutputSpent is invoked when a UTXO is spent
func (ls *SlackThreadLookupService) OutputSpent(ctx context.Context, payload *engine.OutputSpent) error {
	if payload.Topic != "tm_slackthread" {
		return nil
	}

	txid := payload.Outpoint.Txid.String()
	outputIndex := int(payload.Outpoint.Index)

	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputNoLongerRetainedInHistory is called when historical retention is no longer required
func (ls *SlackThreadLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	// For the SlackThread service, we don't need to do anything special here
	return nil
}

// OutputEvicted permanently removes the referenced UTXO from all indices maintained by the Lookup Service
func (ls *SlackThreadLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)
	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputBlockHeightUpdated is called when the block height of an output is updated
func (ls *SlackThreadLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIndex uint64) error {
	// For the SlackThread service, we don't track block heights
	return nil
}

// Lookup answers a lookup query
func (ls *SlackThreadLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	if question.Service != "ls_slackthread" {
		return nil, fmt.Errorf("lookup service not supported: %s", question.Service)
	}

	// Parse query
	var query SlackThreadQuery
	queryBytes, err := json.Marshal(question.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}
	if err := json.Unmarshal(queryBytes, &query); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query: %w", err)
	}

	// Set defaults
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}

	// Validate
	if query.Limit < 0 {
		return nil, fmt.Errorf("limit must be a non-negative number")
	}
	if query.Skip < 0 {
		return nil, fmt.Errorf("skip must be a non-negative number")
	}

	// Parse dates if provided
	var startDate, endDate *time.Time
	if query.StartDate != "" {
		parsed, err := time.Parse(time.RFC3339, query.StartDate)
		if err != nil {
			return nil, fmt.Errorf("invalid startDate provided: %w", err)
		}
		startDate = &parsed
	}
	if query.EndDate != "" {
		parsed, err := time.Parse(time.RFC3339, query.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid endDate provided: %w", err)
		}
		endDate = &parsed
	}

	// Execute query based on provided parameters
	var results []UTXOReference
	if query.ThreadHash != "" {
		// Search by thread hash
		results, err = ls.storage.FindByThreadHash(query.ThreadHash, query.Limit, query.Skip, query.SortOrder)
		if err != nil {
			return nil, err
		}
	} else if query.Txid != "" {
		// Search by txid
		results, err = ls.storage.FindByTxid(query.Txid, query.Limit, query.Skip, query.SortOrder)
		if err != nil {
			return nil, err
		}
	} else {
		// Find all with optional date filtering
		results, err = ls.storage.FindAll(query.Limit, query.Skip, startDate, endDate, query.SortOrder)
		if err != nil {
			return nil, err
		}
	}

	// Return results as LookupAnswer
	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeOutputList,
		Result: results,
	}, nil
}

// GetDocumentation returns the documentation for this lookup service
func (ls *SlackThreadLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *SlackThreadLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "SlackThread Lookup Service",
		Description: "Find threads on-chain.",
	}
}
