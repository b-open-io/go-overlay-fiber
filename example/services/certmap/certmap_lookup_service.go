package certmap

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

const lookupDocs = `# CertMap Lookup Service Documentation

The **CertMap Lookup Service** resolves on-chain PushDrop tokens that
represent certificate type registrations and answers lookup queries.

## Supported Query Parameters

| Parameter   | Type               | Description                                      |
|-------------|--------------------|--------------------------------------------------|
| ` + "`type`" + `       | ` + "`string`" + `          | Certificate type to search for |
| ` + "`name`" + `       | ` + "`string`" + `          | Certificate name (fuzzy search) |
| ` + "`registryOperators`" + `   | ` + "`string[]`" + ` | Registry operator identity keys           |

The service identifier is **` + "`ls_certmap`" + `**.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by type
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_certmap",
    Query: map[string]interface{}{
        "type": "Certificate Type ID",
        "registryOperators": []string{"operator_identity_key"},
    },
}, 10000)

// Find by name (fuzzy)
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_certmap",
    Query: map[string]interface{}{
        "name": "Certificate Name",
        "registryOperators": []string{"operator_identity_key"},
    },
}, 10000)
` + "```" + `
`

// CertMapLookupService implements a lookup service for CertMap name registry
type CertMapLookupService struct {
	storage *CertMapStorage
}

// NewCertMapLookupService creates a new CertMapLookupService instance
func NewCertMapLookupService(db *mongo.Database) *CertMapLookupService {
	return &CertMapLookupService{
		storage: NewCertMapStorage(db),
	}
}

// Ensure CertMapLookupService implements engine.LookupService
var _ engine.LookupService = (*CertMapLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *CertMapLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *CertMapLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "CertMap Lookup Service",
		Description: "Certificate information registration",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *CertMapLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_certmap" {
		return nil
	}

	slog.Debug("CertMap lookup service outputAdded", "txid", output.Outpoint.Txid.String(), "outputIndex", output.Outpoint.Index)

	// Decode the CertMap token fields from the locking script
	result := pushdrop.Decode(output.LockingScript)
	if result == nil {
		return fmt.Errorf("failed to decode PushDrop from locking script")
	}

	// CertMap tokens should have 8 fields (type, name, iconURL, description, documentationURL, certFields, registryOperator, signature)
	if len(result.Fields) != 8 {
		return fmt.Errorf("CertMap token must have exactly 7 data fields + signature")
	}

	// Parse certificate registration data from fields
	certType := string(result.Fields[0])
	name := string(result.Fields[1])
	iconURL := string(result.Fields[2])
	description := string(result.Fields[3])
	documentationURL := string(result.Fields[4])

	var certFields map[string]interface{}
	if err := json.Unmarshal(result.Fields[5], &certFields); err != nil {
		return fmt.Errorf("failed to parse certFields JSON: %w", err)
	}

	registryOperator := string(result.Fields[6])

	// Validate required fields
	if certType == "" || name == "" || iconURL == "" || description == "" ||
		documentationURL == "" || registryOperator == "" {
		return fmt.Errorf("CertMap registration missing required fields")
	}

	registration := &CertMapRegistration{
		Type:             certType,
		Name:             name,
		IconURL:          iconURL,
		Description:      description,
		DocumentationURL: documentationURL,
		CertFields:       certFields,
		RegistryOperator: registryOperator,
	}

	slog.Debug("CertMap lookup service storing record",
		"txid", output.Outpoint.Txid.String(),
		"outputIndex", output.Outpoint.Index,
		"type", certType,
		"name", name)

	// Store CertMap record
	if err := ls.storage.StoreRecord(ctx, output.Outpoint.Txid.String(), int(output.Outpoint.Index), registration); err != nil {
		return fmt.Errorf("failed to store CertMap record: %w", err)
	}

	return nil
}

// OutputSpent is called when an output is spent
func (ls *CertMapLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_certmap" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete CertMap record: %w", err)
	}

	slog.Debug("CertMap token spent", "txid", txid, "outputIndex", outputIndex)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *CertMapLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_certmap" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete CertMap record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *CertMapLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete CertMap record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *CertMapLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// CertMap doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *CertMapLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	slog.Debug("CertMap lookup", "query", string(question.Query))

	// Parse the query
	var query CertMapLookupQuery
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	// Validate that registryOperators is provided
	if len(query.RegistryOperators) == 0 {
		return nil, fmt.Errorf("registryOperators must be provided")
	}

	var results []UTXOReference
	var err error

	// Type lookup
	if query.Type != "" {
		results, err = ls.storage.FindByType(ctx, query.Type, query.RegistryOperators)
	} else if query.Name != "" {
		// Name lookup (fuzzy)
		results, err = ls.storage.FindByName(ctx, query.Name, query.RegistryOperators)
	} else {
		return nil, fmt.Errorf("type or name must be provided")
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no results found, return empty result
	if results == nil {
		results = []UTXOReference{}
	}

	slog.Debug("CertMap lookup completed", "resultCount", len(results))

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeFreeform,
		Result: results,
	}, nil
}
