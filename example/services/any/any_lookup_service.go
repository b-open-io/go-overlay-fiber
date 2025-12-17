package any

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

const lookupDocs = `
# Any Lookup Documentation

Literally any transaction you submitted prior can be looked up here by txid or findAll.
`

// AnyLookupService implements a lookup service for the Any protocol
type AnyLookupService struct {
	storage AnyStorageEngine
}

// NewAnyLookupService creates a new AnyLookupService instance
func NewAnyLookupService(db *mongo.Database) *AnyLookupService {
	return &AnyLookupService{
		storage: NewAnyStorage(db),
	}
}

// NewAnyLookupServiceWithStorage creates a new AnyLookupService with a custom storage engine
func NewAnyLookupServiceWithStorage(storage AnyStorageEngine) *AnyLookupService {
	return &AnyLookupService{
		storage: storage,
	}
}

// Ensure AnyLookupService implements engine.LookupService
var _ engine.LookupService = (*AnyLookupService)(nil)

// OutputAdmittedByTopic is invoked when a new output is added to the overlay
func (ls *AnyLookupService) OutputAdmittedByTopic(ctx context.Context, payload *engine.OutputAdmittedByTopic) error {
	if payload.Topic != "tm_anytx" {
		return nil
	}

	// Parse the AtomicBEEF to get the transaction
	tx, err := transaction.NewTransactionFromBEEF(payload.AtomicBEEF)
	if err != nil {
		return fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := tx.TxID().String()
	outputIndex := int(payload.OutputIndex)

	err = ls.storage.StoreRecord(txid, outputIndex)
	if err != nil {
		slog.Error("AnyLookupService: failed to index output", "txid", txid, "outputIndex", outputIndex, "error", err)
		return err
	}

	return nil
}

// OutputSpent is invoked when a UTXO is spent
func (ls *AnyLookupService) OutputSpent(ctx context.Context, payload *engine.OutputSpent) error {
	if payload.Topic != "tm_anytx" {
		return nil
	}

	txid := payload.Outpoint.Txid.String()
	outputIndex := int(payload.Outpoint.Index)
	spendingTxid := payload.SpendingTxid.String()

	return ls.storage.SpendRecord(txid, outputIndex, spendingTxid)
}

// OutputNoLongerRetainedInHistory is called when historical retention is no longer required
func (ls *AnyLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	// For the Any service, we don't need to do anything special here
	return nil
}

// OutputEvicted permanently removes the referenced UTXO from all indices
func (ls *AnyLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)
	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputBlockHeightUpdated is called when the block height of an output is updated
func (ls *AnyLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIndex uint64) error {
	// For the Any service, we don't track block heights
	return nil
}

// Lookup answers a lookup query
func (ls *AnyLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	if question.Service != "ls_anytx" {
		return nil, fmt.Errorf("lookup service not supported: %s", question.Service)
	}

	// Parse query
	var query AnyQuery
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

	// Execute query
	var results []UTXOReference
	if query.Txid != "" {
		result, err := ls.storage.FindByTxid(query.Txid)
		if err != nil {
			return nil, err
		}
		if result != nil {
			results = []UTXOReference{*result}
		} else {
			results = []UTXOReference{}
		}
	} else {
		results, err = ls.storage.FindAll(query.Limit, query.Skip, startDate, endDate, query.SortOrder)
		if err != nil {
			return nil, err
		}
	}

	// Return results as LookupAnswer
	return &lookup.LookupAnswer{
		Type:   "output-list",
		Result: results,
	}, nil
}

// GetDocumentation returns the documentation for this lookup service
func (ls *AnyLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *AnyLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "Any Lookup Service",
		Description: "Lookup your outputs.",
	}
}
