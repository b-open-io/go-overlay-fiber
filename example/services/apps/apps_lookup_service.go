package apps

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

const lookupDocs = `# Apps Lookup Service Documentation

The **Apps Lookup Service** resolves on-chain PushDrop tokens that
represent published Metanet applications and answers catalog queries.

## Supported Query Parameters

| Parameter   | Type               | Description                                      |
|-------------|--------------------|--------------------------------------------------|
| ` + "`name`" + `       | ` + "`string`" + `          | Fuzzy-matched app name |
| ` + "`domain`" + `     | ` + "`string`" + `          | Exact match on the apps primary domain           |
| ` + "`publisher`" + `  | ` + "`string`" + `          | Identity key of the publisher that signed the token           |
| ` + "`outpoint`" + `   | ` + "`string`" + ` (` + "`\"txid.outputIndex\"`" + `) | Direct UTXO reference                                    |
| ` + "`tags`" + `       | ` + "`string[]`" + `        | Filter by tags (any tag matches)                 |
| ` + "`category`" + `   | ` + "`string`" + `          | Filter by category                               |
| ` + "`limit`" + `      | ` + "`number`" + ` _(opt.)_ | Max documents to return (default: 50)           |
| ` + "`skip`" + `       | ` + "`number`" + ` _(opt.)_ | Number of results to skip (default: 0)          |
| ` + "`sortOrder`" + `  | ` + "`string`" + ` _(opt.)_ | Sort direction: 'asc' or 'desc' (default: 'desc') |

The service identifier is **` + "`ls_apps`" + `**.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by domain
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_apps",
    Query: map[string]interface{}{
        "domain": "myapp.com",
    },
}, 10000)

// Find by name (fuzzy)
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_apps",
    Query: map[string]interface{}{
        "name": "Calculator",
        "limit": 10,
    },
}, 10000)

// Find by publisher
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_apps",
    Query: map[string]interface{}{
        "publisher": "publisher_identity_key",
    },
}, 10000)

// Find by tags
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_apps",
    Query: map[string]interface{}{
        "tags": []string{"productivity", "finance"},
    },
}, 10000)
` + "```" + `
`

// AppsLookupService implements a lookup service for Apps catalog
type AppsLookupService struct {
	storage *AppsStorage
}

// NewAppsLookupService creates a new AppsLookupService instance
func NewAppsLookupService(db *mongo.Database) *AppsLookupService {
	return &AppsLookupService{
		storage: NewAppsStorage(db),
	}
}

// Ensure AppsLookupService implements engine.LookupService
var _ engine.LookupService = (*AppsLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *AppsLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *AppsLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "Apps Lookup Service",
		Description: "Find published Metanet Apps with ease.",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *AppsLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_apps" {
		return nil
	}

	slog.Debug("Apps lookup service outputAdded", "txid", output.Outpoint.Txid.String(), "outputIndex", output.Outpoint.Index)

	// Decode the Apps token fields from the locking script
	result := pushdrop.Decode(output.LockingScript)
	if result == nil {
		return fmt.Errorf("failed to decode PushDrop from locking script")
	}

	// Apps tokens should have exactly 2 fields (metadata + signature)
	if len(result.Fields) != 2 {
		return fmt.Errorf("App token must have exactly one metadata field + signature")
	}

	// Parse metadata from first field
	var metadata PublishedAppMetadata
	if err := json.Unmarshal(result.Fields[0], &metadata); err != nil {
		return fmt.Errorf("failed to parse app metadata JSON: %w", err)
	}

	// Validate required fields
	if metadata.Version == "" ||
		metadata.Name == "" ||
		metadata.Description == "" ||
		metadata.Icon == "" ||
		(metadata.HTTPURL == "" && metadata.UHRPURL == "") ||
		metadata.Domain == "" ||
		metadata.Publisher == "" ||
		metadata.ReleaseDate == "" {
		return fmt.Errorf("App metadata missing required fields")
	}

	slog.Debug("Apps lookup service storing record",
		"txid", output.Outpoint.Txid.String(),
		"outputIndex", output.Outpoint.Index,
		"name", metadata.Name,
		"domain", metadata.Domain)

	// Store Apps catalog record
	if err := ls.storage.StoreRecord(ctx, output.Outpoint.Txid.String(), int(output.Outpoint.Index), &metadata); err != nil {
		return fmt.Errorf("failed to store Apps catalog record: %w", err)
	}

	return nil
}

// OutputSpent is called when an output is spent
func (ls *AppsLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_apps" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete Apps catalog record: %w", err)
	}

	slog.Debug("Apps token spent", "txid", txid, "outputIndex", outputIndex)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *AppsLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_apps" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete Apps catalog record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *AppsLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete Apps catalog record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *AppsLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// Apps doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *AppsLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	slog.Debug("Apps lookup", "query", string(question.Query))

	// Parse the query
	var query AppCatalogQuery
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	var results []UTXOReference
	var err error

	// Domain lookup
	if query.Domain != "" {
		results, err = ls.storage.FindByDomain(ctx, query.Domain, query.Limit, query.Skip, query.SortOrder)
	} else if query.Publisher != "" {
		// Publisher lookup
		results, err = ls.storage.FindByPublisher(ctx, query.Publisher, query.Limit, query.Skip, query.SortOrder)
	} else if len(query.Tags) > 0 {
		// Tag lookup
		results, err = ls.storage.FindByTags(ctx, query.Tags, query.Limit, query.Skip, query.SortOrder)
	} else if query.Category != "" {
		// Category lookup
		results, err = ls.storage.FindByCategory(ctx, query.Category, query.Limit, query.Skip, query.SortOrder)
	} else if query.Name != "" {
		// Fuzzy name lookup
		results, err = ls.storage.FindByNameFuzzy(ctx, query.Name, query.Limit, query.Skip, query.SortOrder)
	} else if query.Outpoint != "" {
		// Outpoint lookup
		results, err = ls.storage.FindByOutpoint(ctx, query.Outpoint)
	} else {
		// No specific query parameters - return all apps
		results, err = ls.storage.FindAllApps(ctx, query.Limit, query.Skip, query.SortOrder)
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no results found, return empty result
	if results == nil {
		results = []UTXOReference{}
	}

	slog.Debug("Apps lookup completed", "resultCount", len(results))

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeFreeform,
		Result: results,
	}, nil
}
