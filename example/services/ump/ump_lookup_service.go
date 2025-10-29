package ump

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

const lookupDocs = `# User Management Protocol Lookup Service

To use this service, send a query that comprises an outpoint, presentationHash, or recoveryHash.

The associated token will be returned.

Example queries:
- By presentationHash: {"presentationHash": "abc123..."}
- By recoveryHash: {"recoveryHash": "def456..."}
- By outpoint: {"outpoint": "txid.outputIndex"}

The lookup service returns the newest UMP token matching the query criteria.
`

// UMPLookupService implements a lookup service for User Management Protocol
type UMPLookupService struct {
	storage *UMPStorage
}

// NewUMPLookupService creates a new UMPLookupService instance
func NewUMPLookupService(db *mongo.Database) *UMPLookupService {
	return &UMPLookupService{
		storage: NewUMPStorage(db),
	}
}

// Ensure UMPLookupService implements engine.LookupService
var _ engine.LookupService = (*UMPLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *UMPLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *UMPLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "UMP Lookup Service",
		Description: "Lookup Service for User Management Protocol tokens",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *UMPLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_users" {
		return nil
	}

	// Decode the UMP fields from the locking script
	result := pushdrop.Decode(output.LockingScript)
	if result == nil {
		return fmt.Errorf("failed to decode PushDrop from locking script")
	}

	// UMP tokens should have at least 11 fields
	if len(result.Fields) < 11 {
		return fmt.Errorf("invalid UMP token: expected at least 11 fields, got %d", len(result.Fields))
	}

	// Extract presentationHash (field 6) and recoveryHash (field 7)
	presentationHash := hex.EncodeToString(result.Fields[6])
	recoveryHash := hex.EncodeToString(result.Fields[7])

	// Store UMP fields in database
	record := &UMPRecord{
		Txid:             output.Outpoint.Txid.String(),
		OutputIndex:      int(output.Outpoint.Index),
		PresentationHash: presentationHash,
		RecoveryHash:     recoveryHash,
	}

	if err := ls.storage.InsertRecord(ctx, record); err != nil {
		return fmt.Errorf("failed to insert UMP record: %w", err)
	}

	slog.Debug("UMP token admitted", "txid", record.Txid, "outputIndex", record.OutputIndex, "presentationHash", presentationHash[:16]+"...")

	return nil
}

// OutputSpent is called when an output is spent
func (ls *UMPLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_users" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete UMP record: %w", err)
	}

	slog.Debug("UMP token spent", "txid", txid, "outputIndex", outputIndex)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *UMPLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_users" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete UMP record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *UMPLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete UMP record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *UMPLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// UMP doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *UMPLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	// Parse the query
	var query UMPQuery
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	// Find the record based on which query parameter is provided
	var record *UMPRecord
	var err error

	if query.PresentationHash != "" {
		record, err = ls.storage.FindByPresentationHash(ctx, query.PresentationHash)
	} else if query.RecoveryHash != "" {
		record, err = ls.storage.FindByRecoveryHash(ctx, query.RecoveryHash)
	} else if query.Outpoint != "" {
		record, err = ls.storage.FindByOutpoint(ctx, query.Outpoint)
	} else {
		return nil, fmt.Errorf("query parameters must include presentationHash, recoveryHash, or outpoint")
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no record found, return empty result
	if record == nil {
		return &lookup.LookupAnswer{
			Type:   lookup.AnswerTypeFreeform,
			Result: []UTXOReference{},
		}, nil
	}

	// Return the result as a UTXOReference
	result := []UTXOReference{
		{
			Txid:        record.Txid,
			OutputIndex: record.OutputIndex,
		},
	}

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeFreeform,
		Result: result,
	}, nil
}
