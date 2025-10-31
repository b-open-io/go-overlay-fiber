package supplychain

import (
	"context"
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

const lookupDocs = `# SupplyChain Lookup Service Documentation

The **SupplyChain Lookup Service** indexes supply chain records with off-chain values
and provides lookup functionality by txid, chainID, or date range.

## Supported Query Parameters

| Parameter   | Type               | Description                                      |
|-------------|--------------------|--------------------------------------------------|
| ` + "`txid`" + `       | ` + "`string`" + `          | Transaction ID to lookup |
| ` + "`chainId`" + `    | ` + "`string`" + `          | Supply chain ID to search for |
| ` + "`limit`" + `      | ` + "`number`" + ` _(opt.)_ | Max documents to return (default: 50)           |
| ` + "`skip`" + `       | ` + "`number`" + ` _(opt.)_ | Number of results to skip (default: 0)          |
| ` + "`startDate`" + `  | ` + "`string`" + ` _(opt.)_ | ISO-8601 start date for date range filtering   |
| ` + "`endDate`" + `    | ` + "`string`" + ` _(opt.)_ | ISO-8601 end date for date range filtering     |
| ` + "`sortOrder`" + `  | ` + "`string`" + ` _(opt.)_ | Sort direction: 'asc' or 'desc' (default: 'desc') |

The service identifier is **` + "`ls_supplychain`" + `**.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by chainId
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_supplychain",
    Query: map[string]interface{}{
        "chainId": "supply-chain-id",
    },
}, 10000)

// Find by txid
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_supplychain",
    Query: map[string]interface{}{
        "txid": "transaction_id",
    },
}, 10000)

// Find all with date range
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_supplychain",
    Query: map[string]interface{}{
        "startDate": "2025-01-01T00:00:00Z",
        "endDate": "2025-12-31T23:59:59Z",
        "limit": 100,
    },
}, 10000)
` + "```" + `
`

// SupplyChainLookupService implements a lookup service for SupplyChain protocol
type SupplyChainLookupService struct {
	storage *SupplyChainStorage
}

// NewSupplyChainLookupService creates a new SupplyChainLookupService instance
func NewSupplyChainLookupService(db *mongo.Database) *SupplyChainLookupService {
	return &SupplyChainLookupService{
		storage: NewSupplyChainStorage(db),
	}
}

// Ensure SupplyChainLookupService implements engine.LookupService
var _ engine.LookupService = (*SupplyChainLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *SupplyChainLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *SupplyChainLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "SupplyChain Lookup Service",
		Description: "Find files on-chain.",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *SupplyChainLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_supplychain" {
		return nil
	}

	slog.Debug("SupplyChain lookup service outputAdded", "txid", output.Outpoint.Txid.String(), "outputIndex", output.Outpoint.Index)

	// Parse off-chain values from the output
	var offChainValuesObject map[string]interface{}
	if len(output.OffChainValues) > 0 {
		if err := json.Unmarshal(output.OffChainValues, &offChainValuesObject); err != nil {
			return fmt.Errorf("failed to parse off-chain values JSON: %w", err)
		}
	} else {
		// If no off-chain values provided, create minimal object with txid as chainId
		offChainValuesObject = map[string]interface{}{
			"chainId": output.Outpoint.Txid.String(),
		}
	}

	// Verify chainId is present
	if _, ok := offChainValuesObject["chainId"]; !ok {
		return fmt.Errorf("missing chainId in off-chain values")
	}

	slog.Debug("SupplyChain lookup service storing record",
		"txid", output.Outpoint.Txid.String(),
		"outputIndex", output.Outpoint.Index,
		"chainId", offChainValuesObject["chainId"])

	// Store SupplyChain record
	if err := ls.storage.StoreRecord(ctx, output.Outpoint.Txid.String(), int(output.Outpoint.Index), offChainValuesObject); err != nil {
		return fmt.Errorf("failed to store SupplyChain record: %w", err)
	}

	return nil
}

// OutputSpent is called when an output is spent
func (ls *SupplyChainLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_supplychain" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)
	spendingTxid := output.SpendingTxid.String()

	if err := ls.storage.SpendRecord(ctx, txid, outputIndex, spendingTxid); err != nil {
		return fmt.Errorf("failed to update SupplyChain record: %w", err)
	}

	slog.Debug("SupplyChain token spent", "txid", txid, "outputIndex", outputIndex, "spendingTxid", spendingTxid)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *SupplyChainLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_supplychain" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete SupplyChain record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *SupplyChainLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete SupplyChain record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *SupplyChainLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// SupplyChain doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *SupplyChainLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	slog.Debug("SupplyChain lookup", "query", string(question.Query))

	// Parse the query
	var query SupplyChainQuery
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

	// Txid lookup
	if query.Txid != "" {
		limit := query.Limit
		if limit == 0 {
			limit = 50
		}
		results, err = ls.storage.FindByTxid(ctx, query.Txid, limit, query.Skip, query.SortOrder)
	} else if query.ChainID != "" {
		// ChainID lookup
		limit := query.Limit
		if limit == 0 {
			limit = 8
		}
		results, err = ls.storage.FindByChainID(ctx, query.ChainID, limit, query.Skip)
	} else {
		// Find all with optional filters
		limit := query.Limit
		if limit == 0 {
			limit = 50
		}
		results, err = ls.storage.FindAll(ctx, limit, query.Skip, startDate, endDate, query.SortOrder)
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no results found, return empty result
	if results == nil {
		results = []UTXOReference{}
	}

	slog.Debug("SupplyChain lookup completed", "resultCount", len(results))

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeFreeform,
		Result: results,
	}, nil
}
