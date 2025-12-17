package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"go.mongodb.org/mongo-driver/mongo"
)

const lookupDocs = `# Identity Lookup Service

The Identity Lookup Service is responsible for managing the rules of admissibility for Identity tokens and handling queries related to them.

To use this service, send a query with one of the following combinations:
- serialNumber (unique lookup)
- attributes AND certifiers
- identityKey AND certifiers AND optionally certificateTypes
- certifiers only

The associated identity certificates will be returned.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by certifiers
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_identity",
    Query: map[string]interface{}{
        "certifiers": []string{"certifier1", "certifier2"},
    },
}, 10000)

// Find by identity key
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_identity",
    Query: map[string]interface{}{
        "identityKey": "02abc...",
        "certifiers": []string{"certifier1"},
    },
}, 10000)

// Find by attributes
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_identity",
    Query: map[string]interface{}{
        "attributes": map[string]string{
            "name": "John",
            "country": "USA",
        },
        "certifiers": []string{"certifier1"},
    },
}, 10000)
` + "```" + `

## Query Parameters

- **serialNumber** (string): Unique certificate serial number (returns single match)
- **attributes** (object): Key-value pairs of certified attributes (supports fuzzy matching)
  - Special key "any": searches across all fields
- **identityKey** (string): Public identity key (subject)
- **certifiers** (string[]): Array of trusted certifier public keys
- **certificateTypes** (string[]): Array of certificate types to filter by

## Response

Returns an array of UTXO references (txid + outputIndex) for all certificates matching the query criteria.

Fuzzy attribute search allows partial matches, e.g., searching for "John" will match "Johnny", "Johnson", etc.
`

// IdentityLookupService implements a lookup service for Identity registry
type IdentityLookupService struct {
	storage IdentityStorageEngine
}

// NewIdentityLookupService creates a new IdentityLookupService instance
func NewIdentityLookupService(db *mongo.Database) *IdentityLookupService {
	return &IdentityLookupService{
		storage: NewIdentityStorage(db),
	}
}

// NewIdentityLookupServiceWithStorage creates a new IdentityLookupService with a custom storage engine
func NewIdentityLookupServiceWithStorage(storage IdentityStorageEngine) *IdentityLookupService {
	return &IdentityLookupService{
		storage: storage,
	}
}

// Ensure IdentityLookupService implements engine.LookupService
var _ engine.LookupService = (*IdentityLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *IdentityLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *IdentityLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "Identity Lookup Service",
		Description: "Identity resolution made easy.",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *IdentityLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_identity" {
		return nil
	}

	// Parse the AtomicBEEF to get the transaction
	tx, err := transaction.NewTransactionFromBEEF(output.AtomicBEEF)
	if err != nil {
		return fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := tx.TxID().String()
	outputIndex := int(output.OutputIndex)

	slog.Debug("Identity lookup service outputAdded", "txid", txid, "outputIndex", outputIndex)

	// Get the locking script from the transaction output
	lockingScript := tx.Outputs[output.OutputIndex].LockingScript

	// Decode the Identity fields from the locking script
	result := pushdrop.Decode(lockingScript)
	if result == nil {
		return fmt.Errorf("failed to decode PushDrop from locking script")
	}

	// Identity tokens should have at least 1 field
	if len(result.Fields) < 1 {
		return fmt.Errorf("invalid Identity token: expected at least 1 field, got %d", len(result.Fields))
	}

	// Parse certificate from first field
	var certData map[string]interface{}
	if err := json.Unmarshal(result.Fields[0], &certData); err != nil {
		return fmt.Errorf("failed to parse certificate JSON: %w", err)
	}

	// Convert to VerifiableCertificate struct
	certJSON, err := json.Marshal(certData)
	if err != nil {
		return fmt.Errorf("failed to marshal certificate: %w", err)
	}

	var verifiableCert certificates.VerifiableCertificate
	if err := json.Unmarshal(certJSON, &verifiableCert); err != nil {
		return fmt.Errorf("failed to unmarshal VerifiableCertificate: %w", err)
	}

	// TODO: Decrypt certificate fields using "anyone" wallet for public field revelation
	// The TypeScript version decrypts fields here to ensure they're publicly revealed
	// This requires full wallet.Interface support which is pending in go-sdk
	// For now, we store the certificate with its encrypted fields
	// The keyring allows selective revelation to authorized verifiers

	// Ensure at least some fields are present
	if len(verifiableCert.Fields) == 0 {
		return fmt.Errorf("no fields present in certificate")
	}

	slog.Debug("Identity lookup service storing record",
		"txid", txid,
		"outputIndex", outputIndex,
		"subject", verifiableCert.Subject.ToDERHex(),
		"certifier", verifiableCert.Certifier.ToDERHex())

	// Store identity certificate
	if err := ls.storage.StoreRecord(ctx, txid, outputIndex, &verifiableCert.Certificate); err != nil {
		return fmt.Errorf("failed to store Identity record: %w", err)
	}

	return nil
}

// OutputSpent is called when an output is spent
func (ls *IdentityLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_identity" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete Identity record: %w", err)
	}

	slog.Debug("Identity token spent", "txid", txid, "outputIndex", outputIndex)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *IdentityLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_identity" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete Identity record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *IdentityLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete Identity record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *IdentityLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// Identity doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *IdentityLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	slog.Debug("Identity lookup", "query", string(question.Query))

	// Parse the query
	var query IdentityQuery
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	// Find the records based on query type (in priority order)
	var results []UTXOReference
	var err error

	// 1. Serial number lookup (unique)
	if query.SerialNumber != "" {
		results, err = ls.storage.FindByCertificateSerialNumber(ctx, query.SerialNumber)
		if err != nil {
			return nil, fmt.Errorf("lookup by serial number failed: %w", err)
		}
		slog.Debug("Identity lookup by serial number", "resultCount", len(results))
		return &lookup.LookupAnswer{
			Type:   lookup.AnswerTypeFreeform,
			Result: results,
		}, nil
	}

	// 2. Attributes + certifiers
	if len(query.Attributes) > 0 && len(query.Certifiers) > 0 {
		results, err = ls.storage.FindByAttribute(ctx, query.Attributes, query.Certifiers)
	} else if query.IdentityKey != "" && len(query.CertificateTypes) > 0 && len(query.Certifiers) > 0 {
		// 3. Identity key + certificate types + certifiers
		results, err = ls.storage.FindByCertificateType(ctx, query.CertificateTypes, query.IdentityKey, query.Certifiers)
	} else if query.IdentityKey != "" && len(query.Certifiers) > 0 {
		// 4. Identity key + certifiers
		results, err = ls.storage.FindByIdentityKey(ctx, query.IdentityKey, query.Certifiers)
	} else if len(query.Certifiers) > 0 {
		// 5. Certifiers only
		results, err = ls.storage.FindByCertifier(ctx, query.Certifiers)
	} else {
		return nil, fmt.Errorf("query parameters must include one of: serialNumber, (attributes + certifiers), (identityKey + certifiers), or certifiers")
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no results found, return empty result
	if results == nil {
		results = []UTXOReference{}
	}

	slog.Debug("Identity lookup completed", "resultCount", len(results))

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeFreeform,
		Result: results,
	}, nil
}
