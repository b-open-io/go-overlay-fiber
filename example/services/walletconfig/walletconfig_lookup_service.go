package walletconfig

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

const lookupDocs = `# WalletConfig Lookup Service Documentation

The **WalletConfig Lookup Service** resolves on-chain PushDrop tokens that
represent wallet configuration options for service discovery.

## Supported Query Parameters

| Parameter   | Type               | Description                                      |
|-------------|--------------------|--------------------------------------------------|
| ` + "`configID`" + `       | ` + "`string`" + `          | Configuration ID to search for |
| ` + "`name`" + `       | ` + "`string`" + `          | Configuration name (fuzzy search) |
| ` + "`wab`" + `   | ` + "`string`" + ` | Wallet Authentication Backend URL           |
| ` + "`storage`" + `   | ` + "`string`" + ` | Wallet storage URL           |
| ` + "`messagebox`" + `   | ` + "`string`" + ` | Messagebox URL           |
| ` + "`registryOperators`" + `   | ` + "`string[]`" + ` (required) | Registry operator identity keys           |

The service identifier is **` + "`ls_walletconfig`" + `**.

## Example

` + "```go" + `
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by configID
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_walletconfig",
    Query: map[string]interface{}{
        "configID": "my-wallet-config",
        "registryOperators": []string{"operator_identity_key"},
    },
}, 10000)

// Find by name (fuzzy)
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_walletconfig",
    Query: map[string]interface{}{
        "name": "My Wallet",
        "registryOperators": []string{"operator_identity_key"},
    },
}, 10000)

// List all configs from trusted operators
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_walletconfig",
    Query: map[string]interface{}{
        "registryOperators": []string{"operator_identity_key"},
    },
}, 10000)
` + "```" + `
`

// WalletConfigLookupService implements a lookup service for WalletConfig registry
type WalletConfigLookupService struct {
	storage WalletConfigStorageEngine
}

// NewWalletConfigLookupService creates a new WalletConfigLookupService instance
func NewWalletConfigLookupService(db *mongo.Database) *WalletConfigLookupService {
	return &WalletConfigLookupService{
		storage: NewWalletConfigStorage(db),
	}
}

// NewWalletConfigLookupServiceWithStorage creates a new WalletConfigLookupService with a custom storage engine
func NewWalletConfigLookupServiceWithStorage(storage WalletConfigStorageEngine) *WalletConfigLookupService {
	return &WalletConfigLookupService{
		storage: storage,
	}
}

// Ensure WalletConfigLookupService implements engine.LookupService
var _ engine.LookupService = (*WalletConfigLookupService)(nil)

// GetDocumentation returns the documentation for this lookup service
func (ls *WalletConfigLookupService) GetDocumentation() string {
	return lookupDocs
}

// GetMetaData returns metadata about this lookup service
func (ls *WalletConfigLookupService) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "WalletConfig Lookup Service",
		Description: "Wallet configuration service discovery",
	}
}

// OutputAdmittedByTopic is called when an output is admitted to the topic
func (ls *WalletConfigLookupService) OutputAdmittedByTopic(ctx context.Context, output *engine.OutputAdmittedByTopic) error {
	if output.Topic != "tm_walletconfig" {
		return nil
	}

	// Parse the AtomicBEEF to get the transaction
	tx, err := transaction.NewTransactionFromBEEF(output.AtomicBEEF)
	if err != nil {
		return fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := tx.TxID().String()
	outputIndex := int(output.OutputIndex)

	slog.Debug("WalletConfig lookup service outputAdded", "txid", txid, "outputIndex", outputIndex)

	// Get the locking script from the transaction output
	lockingScript := tx.Outputs[output.OutputIndex].LockingScript

	// Decode the WalletConfig token fields from the locking script
	result := pushdrop.Decode(lockingScript)
	if result == nil {
		return fmt.Errorf("failed to decode PushDrop from locking script")
	}

	// WalletConfig tokens should have 9 fields (8 data fields + signature)
	if len(result.Fields) != 9 {
		return fmt.Errorf("WalletConfig token must have exactly 8 data fields + signature")
	}

	// Parse wallet configuration data from fields
	configID := string(result.Fields[0])
	name := string(result.Fields[1])
	icon := string(result.Fields[2])
	wab := string(result.Fields[3])
	storage := string(result.Fields[4])
	messagebox := string(result.Fields[5])
	legal := string(result.Fields[6])
	registryOperator := string(result.Fields[7])

	// Validate required fields
	if configID == "" || name == "" || icon == "" || wab == "" ||
		storage == "" || messagebox == "" || legal == "" || registryOperator == "" {
		return fmt.Errorf("WalletConfig registration missing required fields")
	}

	registration := &WalletConfigRegistration{
		ConfigID:         configID,
		Name:             name,
		Icon:             icon,
		WAB:              wab,
		Storage:          storage,
		Messagebox:       messagebox,
		Legal:            legal,
		RegistryOperator: registryOperator,
	}

	slog.Debug("WalletConfig lookup service storing record",
		"txid", txid,
		"outputIndex", outputIndex,
		"configID", configID,
		"name", name)

	// Store WalletConfig record (with duplicate check)
	if err := ls.storage.StoreRecord(ctx, txid, outputIndex, registration); err != nil {
		return fmt.Errorf("failed to store WalletConfig record: %w", err)
	}

	return nil
}

// OutputSpent is called when an output is spent
func (ls *WalletConfigLookupService) OutputSpent(ctx context.Context, output *engine.OutputSpent) error {
	if output.Topic != "tm_walletconfig" {
		return nil
	}

	txid := output.Outpoint.Txid.String()
	outputIndex := int(output.Outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete WalletConfig record: %w", err)
	}

	slog.Debug("WalletConfig token spent", "txid", txid, "outputIndex", outputIndex)

	return nil
}

// OutputNoLongerRetainedInHistory is called when an output is no longer retained
func (ls *WalletConfigLookupService) OutputNoLongerRetainedInHistory(ctx context.Context, outpoint *transaction.Outpoint, topic string) error {
	if topic != "tm_walletconfig" {
		return nil
	}

	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete WalletConfig record: %w", err)
	}

	return nil
}

// OutputEvicted is called when an output is evicted
func (ls *WalletConfigLookupService) OutputEvicted(ctx context.Context, outpoint *transaction.Outpoint) error {
	txid := outpoint.Txid.String()
	outputIndex := int(outpoint.Index)

	if err := ls.storage.DeleteRecord(ctx, txid, outputIndex); err != nil {
		return fmt.Errorf("failed to delete WalletConfig record: %w", err)
	}

	return nil
}

// OutputBlockHeightUpdated is called when an output's block height is updated
func (ls *WalletConfigLookupService) OutputBlockHeightUpdated(ctx context.Context, txid *chainhash.Hash, blockHeight uint32, blockIdx uint64) error {
	// WalletConfig doesn't track block height
	return nil
}

// Lookup performs a lookup query
func (ls *WalletConfigLookupService) Lookup(ctx context.Context, question *lookup.LookupQuestion) (*lookup.LookupAnswer, error) {
	if question == nil {
		return nil, fmt.Errorf("a valid query must be provided")
	}

	slog.Debug("WalletConfig lookup", "query", string(question.Query))

	// Parse the query
	var query WalletConfigQuery
	if err := json.Unmarshal(question.Query, &query); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	// Validate that registryOperators is provided
	if len(query.RegistryOperators) == 0 {
		return nil, fmt.Errorf("registryOperators must be provided")
	}

	var results []UTXOReference
	var err error

	// Query by configID
	if query.ConfigID != "" {
		results, err = ls.storage.FindByConfigID(ctx, query.ConfigID, query.RegistryOperators)
	} else if query.Name != "" {
		// Query by name (fuzzy)
		results, err = ls.storage.FindByName(ctx, query.Name, query.RegistryOperators)
	} else if query.WAB != "" {
		// Query by WAB
		results, err = ls.storage.FindByWAB(ctx, query.WAB, query.RegistryOperators)
	} else if query.Storage != "" {
		// Query by storage
		results, err = ls.storage.FindByStorage(ctx, query.Storage, query.RegistryOperators)
	} else if query.Messagebox != "" {
		// Query by messagebox
		results, err = ls.storage.FindByMessagebox(ctx, query.Messagebox, query.RegistryOperators)
	} else {
		// List all configs (when only registryOperators is provided)
		results, err = ls.storage.ListAll(ctx, query.RegistryOperators)
	}

	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}

	// If no results found, return empty result
	if results == nil {
		results = []UTXOReference{}
	}

	slog.Debug("WalletConfig lookup completed", "resultCount", len(results))

	return &lookup.LookupAnswer{
		Type:   lookup.AnswerTypeOutputList,
		Result: results,
	}, nil
}
