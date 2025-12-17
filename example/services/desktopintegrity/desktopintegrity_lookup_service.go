package desktopintegrity

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

const lookupDocs = `# DesktopIntegrity Lookup Service Documentation

The **DesktopIntegrity Lookup Service** (service ID: ` + "`ls_desktopintegrity`" + `) lets clients search the on-chain *DesktopIntegrity* messages that are indexed by the **DesktopIntegrity Topic Manager**. Each record represents an OP_RETURN output whose data is a 32-byte file hash.

## Supported Query Parameters

| Parameter   | Type               | Description                                      |
|-------------|--------------------|--------------------------------------------------|
| ` + "`fileHash`" + `    | ` + "`string`" + `          | File hash to lookup (hex-encoded 32 bytes) |
| ` + "`txid`" + `        | ` + "`string`" + `          | Transaction ID to search for |
| ` + "`limit`" + `       | ` + "`number`" + ` _(opt.)_ | Max documents to return (default: 50)           |
| ` + "`skip`" + `        | ` + "`number`" + ` _(opt.)_ | Number of results to skip (default: 0)          |
| ` + "`startDate`" + `   | ` + "`string`" + ` _(opt.)_ | ISO-8601 start date for date range filtering   |
| ` + "`endDate`" + `     | ` + "`string`" + ` _(opt.)_ | ISO-8601 end date for date range filtering     |
| ` + "`sortOrder`" + `   | ` + "`string`" + ` _(opt.)_ | Sort direction: 'asc' or 'desc' (default: 'desc') |

The service identifier is **` + "`ls_desktopintegrity`" + `**.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find all
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_desktopintegrity",
    Query: map[string]interface{}{},
}, 10000)

// Find by file hash
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_desktopintegrity",
    Query: map[string]interface{}{
        "fileHash": "abc123...",
    },
}, 10000)

// Find by txid
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_desktopintegrity",
    Query: map[string]interface{}{
        "txid": "transaction_id",
    },
}, 10000)
` + "```" + `
`

// DesktopIntegrityLookupService implements a lookup service for DesktopIntegrity protocol
type DesktopIntegrityLookupService struct {
	storage *DesktopIntegrityStorage
}

// NewDesktopIntegrityLookupService creates a new DesktopIntegrityLookupService instance
func NewDesktopIntegrityLookupService(db *mongo.Database) *DesktopIntegrityLookupService {
	return &DesktopIntegrityLookupService{
		storage: NewDesktopIntegrityStorage(db),
	}
}

// Ensure DesktopIntegrityLookupService implements engine.LookupService
var _ engine.LookupService = (*DesktopIntegrityLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *DesktopIntegrityLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *DesktopIntegrityLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "DesktopIntegrity Lookup Service",
		Description: "Find files on-chain.",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *DesktopIntegrityLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_desktopintegrity" {
		return nil
	}

	// Parse the AtomicBEEF to get the transaction
	tx, err := transaction.NewTransactionFromBEEF(output.AtomicBEEF)
	if err != nil {
		return fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := tx.TxID().String()
	outputIndex := int(output.OutputIndex)

	slog.Debug("DesktopIntegrity lookup service outputAdded", "txid", txid, "outputIndex", outputIndex)

	// Get the locking script from the transaction output
	lockingScript := tx.Outputs[output.OutputIndex].LockingScript

	// Parse the locking script to extract the file hash
	chunks, err := lockingScript.ParseOps()
	if err != nil {
		return fmt.Errorf("failed to parse locking script: %w", err)
	}

	if len(chunks) != 2 {
		return fmt.Errorf("invalid locking script: expected 2 chunks, got %d", len(chunks))
	}

	// The file hash is in chunk 1 (OP_RETURN data)
	fileHashData := chunks[1].Data
	if len(fileHashData) == 0 {
		return fmt.Errorf("invalid DesktopIntegrity token: file hash data is empty")
	}

	// First byte should be length (32), followed by 32 bytes of hash
	if fileHashData[0] != 32 || len(fileHashData) != 33 {
		return fmt.Errorf("invalid DesktopIntegrity token: file hash must be exactly 32 bytes (got %d bytes)", len(fileHashData)-1)
	}

	// Extract the 32-byte hash (skip the length byte)
	fileHashString := hex.EncodeToString(fileHashData[1:])

	slog.Debug("DesktopIntegrity lookup service storing record",
		"txid", txid,
		"outputIndex", outputIndex,
		"fileHash", fileHashString)

	// Store DesktopIntegrity record
	if err := ls.storage.StoreRecord(ctx, txid, outputIndex, fileHashString, output.OffChainValues); err != nil {
		return fmt.Errorf("failed to store DesktopIntegrity record: %w", err)
	}

	return nil
}

// OutputSpent is called when an output is spent
func (ls *DesktopIntegrityLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_desktopintegrity" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete DesktopIntegrity record: %w", err)
	}

	slog.Debug("DesktopIntegrity record spent", "txid", txid, "outputIndex", outputIndex)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *DesktopIntegrityLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_desktopintegrity" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete DesktopIntegrity record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *DesktopIntegrityLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete DesktopIntegrity record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *DesktopIntegrityLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// DesktopIntegrity doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *DesktopIntegrityLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	slog.Debug("DesktopIntegrity lookup", "query", string(question.Query))

	// Parse the query
	var query DesktopIntegrityQuery
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	// Validate parameters
	if query.Limit < 0 {
		return nil, fmt.Errorf("limit must be a non-negative number")
	}
	if query.Skip < 0 {
		return nil, fmt.Errorf("skip must be a non-negative number")
	}

	// Set defaults
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}

	// Parse date parameters if provided
	var startDate, endDate *time.Time
	if query.StartDate != "" {
		t, err := time.Parse(time.RFC3339, query.StartDate)
		if err != nil {
			return nil, fmt.Errorf("invalid startDate format (expected ISO-8601): %w", err)
		}
		startDate = &t
	}
	if query.EndDate != "" {
		t, err := time.Parse(time.RFC3339, query.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid endDate format (expected ISO-8601): %w", err)
		}
		endDate = &t
	}

	var results []UTXOReference
	var err error

	// FileHash lookup
	if query.FileHash != "" {
		results, err = ls.storage.FindByFileHash(ctx, query.FileHash, query.Limit, query.Skip, query.SortOrder)
	} else if query.Txid != "" {
		// Txid lookup
		results, err = ls.storage.FindByTxid(ctx, query.Txid, query.Limit, query.Skip, query.SortOrder)
	} else {
		// Find all with optional filters
		results, err = ls.storage.FindAll(ctx, query.Limit, query.Skip, startDate, endDate, query.SortOrder)
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no results found, return empty result
	if results == nil {
		results = []UTXOReference{}
	}

	slog.Debug("DesktopIntegrity lookup completed", "resultCount", len(results))

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeFreeform,
		Result: results,
	}, nil
}
