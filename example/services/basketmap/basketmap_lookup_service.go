package basketmap

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

const lookupDocs = `# BasketMap Lookup Service

The BasketMap Lookup Service is responsible for managing the rules of admissibility for BasketMap tokens and handling queries related to them.

To use this service, send a query that includes:
- basketID AND registryOperators (to find by basket ID)
- OR name AND registryOperators (to find by basket name with fuzzy matching)

The associated basket registration tokens will be returned.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by basket ID
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_basketmap",
    Query: map[string]interface{}{
        "basketID": "my-basket",
        "registryOperators": []string{"operator1", "operator2"},
    },
}, 10000)

// Find by basket name (fuzzy search)
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_basketmap",
    Query: map[string]interface{}{
        "name": "payments",
        "registryOperators": []string{"operator1"},
    },
}, 10000)
` + "```" + `

## Query Parameters

- **basketID** (string): Basket type identifier (exact match)
- **name** (string): Basket display name (supports fuzzy matching)
- **registryOperators** (string[]): Array of trusted registry operator identity keys

## Response

Returns an array of UTXO references (txid + outputIndex) for all basket registrations matching the query criteria.

Fuzzy name search allows partial matches, e.g., searching for "pay" will match "payment", "payday", etc.
`

// BasketMapLookupService implements a lookup service for BasketMap registry
type BasketMapLookupService struct {
	storage *BasketMapStorage
}

// NewBasketMapLookupService creates a new BasketMapLookupService instance
func NewBasketMapLookupService(db *mongo.Database) *BasketMapLookupService {
	return &BasketMapLookupService{
		storage: NewBasketMapStorage(db),
	}
}

// Ensure BasketMapLookupService implements engine.LookupService
var _ engine.LookupService = (*BasketMapLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *BasketMapLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *BasketMapLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "BasketMap Lookup Service",
		Description: "Basket name resolution",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *BasketMapLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_basketmap" {
		return nil
	}

	// Parse the AtomicBEEF to get the transaction
	tx, err := transaction.NewTransactionFromBEEF(output.AtomicBEEF)
	if err != nil {
		return fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := tx.TxID().String()
	outputIndex := int(output.OutputIndex)

	// Get the locking script from the transaction output
	lockingScript := tx.Outputs[output.OutputIndex].LockingScript

	// Decode the BasketMap fields from the locking script
	result := pushdrop.Decode(lockingScript)
	if result == nil {
		return fmt.Errorf("failed to decode PushDrop from locking script")
	}

	// BasketMap tokens should have exactly 7 fields
	if len(result.Fields) != 7 {
		return fmt.Errorf("invalid BasketMap token: expected 7 fields, got %d", len(result.Fields))
	}

	// Extract fields
	basketIDBytes := result.Fields[0]
	nameBytes := result.Fields[1]
	registryOperatorBytes := result.Fields[5]

	// Create registration
	registration := BasketMapRegistration{
		BasketID:         string(basketIDBytes),
		Name:             string(nameBytes),
		RegistryOperator: string(registryOperatorBytes),
	}

	// Store basket registration
	if err := ls.storage.StoreRecord(ctx, txid, outputIndex, registration); err != nil {
		return fmt.Errorf("failed to store BasketMap record: %w", err)
	}

	slog.Debug("BasketMap token admitted",
		"txid", txid,
		"outputIndex", outputIndex,
		"basketID", registration.BasketID,
		"name", registration.Name)

	return nil
}

// OutputSpent is called when an output is spent
func (ls *BasketMapLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_basketmap" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete BasketMap record: %w", err)
	}

	slog.Debug("BasketMap token spent", "txid", txid, "outputIndex", outputIndex)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *BasketMapLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_basketmap" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete BasketMap record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *BasketMapLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete BasketMap record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *BasketMapLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// BasketMap doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *BasketMapLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	// Parse the query
	var query BasketMapQuery
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	// Find the records based on which query parameters are provided
	var results []UTXOReference
	var err error

	if query.BasketID != "" && len(query.RegistryOperators) > 0 {
		results, err = ls.storage.FindByID(ctx, query.BasketID, query.RegistryOperators)
	} else if query.Name != "" && len(query.RegistryOperators) > 0 {
		results, err = ls.storage.FindByName(ctx, query.Name, query.RegistryOperators)
	} else {
		return nil, fmt.Errorf("query parameters must include (basketID and registryOperators) or (name and registryOperators)")
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no results found, return empty result
	if results == nil {
		results = []UTXOReference{}
	}

	slog.Debug("BasketMap lookup completed", "resultCount", len(results))

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeFreeform,
		Result: results,
	}, nil
}
