package hello

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"go.mongodb.org/mongo-driver/mongo"
)

const lookupDocs = `
# HelloWorld Lookup Documentation

Query HelloWorld messages by text search or date range.

## Query Parameters

- **message**: Text to search for (uses full-text search)
- **limit**: Maximum results to return (default: 50)
- **skip**: Number of results to skip for pagination (default: 0)
- **startDate**: Filter by creation date (ISO 8601)
- **endDate**: Filter by creation date (ISO 8601)
- **sortOrder**: Sort direction - "asc" or "desc" (default: "desc")

## Example Query

{
  "service": "ls_helloworld",
  "query": {
    "message": "hello",
    "limit": 10,
    "sortOrder": "desc"
  }
}
`

// HelloWorldLookupService implements a lookup service for the HelloWorld protocol
type HelloWorldLookupService struct {
	storage *HelloWorldStorage
}

// NewHelloWorldLookupService creates a new HelloWorldLookupService instance
func NewHelloWorldLookupService(db *mongo.Database) *HelloWorldLookupService {
	return &HelloWorldLookupService{
		storage: NewHelloWorldStorage(db),
	}
}

// Ensure HelloWorldLookupService implements engine.LookupService
var _ engine.LookupService = (*HelloWorldLookupService)(nil)

// OutputAdmittedByTopic is invoked when a new output is added to the overlay
func (ls *HelloWorldLookupService) OutputAdmittedByTopic(ctx context.Context, payload *engine.OutputAdmittedByTopic) error {
	if payload.Topic != "tm_helloworld" {
		return nil
	}

	txid := payload.Outpoint.Txid.String()
	outputIndex := int(payload.Outpoint.Index)

	// Decode the PushDrop token
	result := pushdrop.Decode(payload.LockingScript)
	if result == nil || len(result.Fields) < 2 {
		err := fmt.Errorf("invalid HelloWorld token: invalid PushDrop structure")
		log.Printf("HelloWorldLookupService: failed to index %s.%d: %v", txid, outputIndex, err)
		return err
	}

	// Extract message (all fields except the last one which is the signature)
	messageFields := result.Fields[:len(result.Fields)-1]
	if len(messageFields) != 1 {
		err := fmt.Errorf("invalid HelloWorld token: wrong field count")
		log.Printf("HelloWorldLookupService: failed to index %s.%d: %v", txid, outputIndex, err)
		return err
	}

	message := string(messageFields[0])
	if len(message) < 2 {
		err := fmt.Errorf("invalid HelloWorld token: message too short")
		log.Printf("HelloWorldLookupService: failed to index %s.%d: %v", txid, outputIndex, err)
		return err
	}

	// Store the message
	err := ls.storage.StoreRecord(txid, outputIndex, message)
	if err != nil {
		log.Printf("HelloWorldLookupService: failed to store %s.%d: %v", txid, outputIndex, err)
		return err
	}

	return nil
}

// OutputSpent is invoked when a UTXO is spent
func (ls *HelloWorldLookupService) OutputSpent(ctx context.Context, payload *engine.OutputSpent) error {
	if payload.Topic != "tm_helloworld" {
		return nil
	}

	txid := payload.Outpoint.Txid.String()
	outputIndex := int(payload.Outpoint.Index)

	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputNoLongerRetainedInHistory is called when historical retention is no longer required
func (ls *HelloWorldLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	// For the HelloWorld service, we don't need to do anything special here
	return nil
}

// OutputEvicted permanently removes the referenced UTXO from all indices
func (ls *HelloWorldLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)
	return ls.storage.DeleteRecord(txid, outputIndex)
}

// OutputBlockHeightUpdated is called when the block height of an output is updated
func (ls *HelloWorldLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIndex uint64) error {
	// For the HelloWorld service, we don't track block heights
	return nil
}

// Lookup answers a lookup query
func (ls *HelloWorldLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	if question.Service != "ls_helloworld" {
		return nil, fmt.Errorf("lookup service not supported: %s", question.Service)
	}

	// Parse query
	var query HelloWorldQuery
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
	if query.Message != "" {
		// Search by message text
		results, err = ls.storage.FindByMessage(query.Message, query.Limit, query.Skip, query.SortOrder)
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
		Type:   "output-list",
		Result: results,
	}, nil
}

// GetDocumentation returns the documentation for this lookup service
func (ls *HelloWorldLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *HelloWorldLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "HelloWorld Lookup Service",
		Description: "Query and discover HelloWorld messages.",
	}
}
