package protomap

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

const lookupDocs = `# ProtoMap Lookup Service

The ProtoMap Lookup Service is responsible for managing the rules of admissibility for ProtoMap tokens and handling queries related to them.

To use this service, send a query that includes:
- name AND registryOperators (to find by protocol name)
- OR protocolID AND registryOperators (to find by protocol ID)

The associated protocol registration tokens will be returned.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by protocol name
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_protomap",
    Query: map[string]interface{}{
        "name": "my-protocol",
        "registryOperators": []string{"operator1", "operator2"},
    },
}, 10000)

// Find by protocol ID
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_protomap",
    Query: map[string]interface{}{
        "protocolID": map[string]interface{}{
            "securityLevel": 1,
            "protocol": "my-protocol",
        },
        "registryOperators": []string{"operator1"},
    },
}, 10000)
` + "```" + `

## Query Parameters

- **name** (string): Protocol display name
- **protocolID** (object): Protocol identifier with securityLevel (0, 1, or 2) and protocol string
- **registryOperators** (string[]): Array of trusted registry operator identity keys

## Response

Returns an array of UTXO references (txid + outputIndex) for all protocol registrations matching the query criteria.
`

// ProtoMapLookupService implements a lookup service for ProtoMap protocol registry
type ProtoMapLookupService struct {
	storage ProtoMapStorageEngine
}

// NewProtoMapLookupService creates a new ProtoMapLookupService instance
func NewProtoMapLookupService(db *mongo.Database) *ProtoMapLookupService {
	return &ProtoMapLookupService{
		storage: NewProtoMapStorage(db),
	}
}

// NewProtoMapLookupServiceWithStorage creates a new ProtoMapLookupService with a custom storage engine
func NewProtoMapLookupServiceWithStorage(storage ProtoMapStorageEngine) *ProtoMapLookupService {
	return &ProtoMapLookupService{
		storage: storage,
	}
}

// Ensure ProtoMapLookupService implements engine.LookupService
var _ engine.LookupService = (*ProtoMapLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *ProtoMapLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *ProtoMapLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "ProtoMap",
		Description: "Register protocolIDs for UX enrichment",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *ProtoMapLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_protomap" {
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

	// Decode the ProtoMap fields from the locking script
	result := pushdrop.Decode(lockingScript)
	if result == nil {
		return fmt.Errorf("failed to decode PushDrop from locking script")
	}

	// ProtoMap tokens should have exactly 7 fields
	if len(result.Fields) != 7 {
		return fmt.Errorf("invalid ProtoMap token: expected 7 fields, got %d", len(result.Fields))
	}

	// Extract fields
	protocolIDBytes := result.Fields[0]
	nameBytes := result.Fields[1]
	registryOperatorBytes := result.Fields[5]

	// Parse protocolID
	var protocolIDArray []interface{}
	if err := json.Unmarshal(protocolIDBytes, &protocolIDArray); err != nil {
		return fmt.Errorf("failed to parse protocolID: %w", err)
	}

	if len(protocolIDArray) != 2 {
		return fmt.Errorf("invalid protocolID structure")
	}

	securityLevel, ok := protocolIDArray[0].(float64)
	if !ok {
		return fmt.Errorf("invalid security level type")
	}

	protocol, ok := protocolIDArray[1].(string)
	if !ok {
		return fmt.Errorf("invalid protocol type")
	}

	// Create registration
	registration := ProtoMapRegistration{
		RegistryOperator: string(registryOperatorBytes),
		ProtocolID: ProtocolID{
			SecurityLevel: int(securityLevel),
			Protocol:      protocol,
		},
		Name: string(nameBytes),
	}

	// Store protocol registration
	if err := ls.storage.StoreRecord(ctx, txid, outputIndex, registration); err != nil {
		return fmt.Errorf("failed to store ProtoMap record: %w", err)
	}

	slog.Debug("ProtoMap token admitted",
		"txid", txid,
		"outputIndex", outputIndex,
		"name", registration.Name,
		"protocol", registration.ProtocolID.Protocol)

	return nil
}

// OutputSpent is called when an output is spent
func (ls *ProtoMapLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_protomap" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete ProtoMap record: %w", err)
	}

	slog.Debug("ProtoMap token spent", "txid", txid, "outputIndex", outputIndex)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *ProtoMapLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_protomap" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete ProtoMap record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *ProtoMapLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete ProtoMap record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *ProtoMapLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// ProtoMap doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *ProtoMapLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	// Parse the query
	var query ProtoMapQuery
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	// Find the records based on which query parameters are provided
	var results []UTXOReference
	var err error

	if query.Name != "" && len(query.RegistryOperators) > 0 {
		results, err = ls.storage.FindByName(ctx, query.Name, query.RegistryOperators)
	} else if query.ProtocolID != nil && len(query.RegistryOperators) > 0 {
		results, err = ls.storage.FindByProtocolID(ctx, *query.ProtocolID, query.RegistryOperators)
	} else {
		return nil, fmt.Errorf("query parameters must include (name and registryOperators) or (protocolID and registryOperators)")
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no results found, return empty result
	if results == nil {
		results = []UTXOReference{}
	}

	slog.Debug("ProtoMap lookup completed", "resultCount", len(results))

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeOutputList,
		Result: results,
	}, nil
}

// deserializeProtocolID parses a protocol ID from hex string
func deserializeProtocolID(hexStr string) (*ProtocolID, error) {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}

	var protocolIDArray []interface{}
	if err := json.Unmarshal(data, &protocolIDArray); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protocolID: %w", err)
	}

	if len(protocolIDArray) != 2 {
		return nil, fmt.Errorf("invalid protocolID format")
	}

	securityLevel, ok := protocolIDArray[0].(float64)
	if !ok || (securityLevel != 0 && securityLevel != 1 && securityLevel != 2) {
		return nil, fmt.Errorf("invalid security level")
	}

	protocol, ok := protocolIDArray[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid protocol string")
	}

	return &ProtocolID{
		SecurityLevel: int(securityLevel),
		Protocol:      protocol,
	}, nil
}
