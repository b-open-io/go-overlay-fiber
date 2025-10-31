package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

const topicDocs = `# Apps Topic Manager Documentation

The **Apps Topic Manager** defines which transaction outputs are
_admissible_ as on-chain app listings.

## Admissibility Rules

- The transaction must include at least one output.
- For an output to be admitted **all** of the following must hold:
  1. The locking script is a valid **PushDrop** script consisting of
     exactly **one** data field **plus** its signature field.
  2. The data field decodes to valid app metadata.
  3. Required JSON properties are present and non-empty:
     ` + "`version`" + `, ` + "`name`" + `, ` + "`description`" + `, ` + "`icon`" + `, ` + "`domain`" + `,
     ` + "`publisher`" + `, and ` + "`release_date`" + `.
  4. At least one of **` + "`httpURL`" + `** _or_ **` + "`uhrpURL`" + `** is provided.
  5. Optional properties—` + "`short_name`" + `, ` + "`category`" + `, ` + "`tags`" + `,
     ` + "`changelog`" + `, ` + "`banner_image_url`" + `, ` + "`screenshot_urls`" + `—may be
     included but are not validated beyond basic type checks.

Outputs failing any check are ignored and **not** admitted.
`

// AppsTopicManager implements a topic manager for Apps catalog
type AppsTopicManager struct{}

// NewAppsTopicManager creates a new AppsTopicManager instance
func NewAppsTopicManager() *AppsTopicManager {
	return &AppsTopicManager{}
}

// Ensure AppsTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*AppsTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the Apps transaction that are admissible
func (tm *AppsTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}
	coinsToRetain := []uint32{}

	slog.Info("Apps topic manager was invoked")

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("Apps topic manager parsed transaction", "txid", txid)

	// Validate params
	if len(parsedTransaction.Inputs) < 1 {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("missing parameter: inputs")
	}
	if len(parsedTransaction.Outputs) < 1 {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("missing parameter: outputs")
	}

	// Try to decode and validate transaction outputs
	for i, output := range parsedTransaction.Outputs {
		outputIndex := uint32(i)
		// Decode the fields
		result := pushdrop.Decode(output.LockingScript)
		if result == nil {
			slog.Debug("Output failed to decode PushDrop", "index", i)
			continue
		}

		// App tokens should have exactly 2 fields (metadata + signature)
		if len(result.Fields) != 2 {
			slog.Debug("App token field count invalid", "index", i, "expected", 2, "got", len(result.Fields))
			continue
		}

		// Parse metadata from first field
		var metadata PublishedAppMetadata
		if err := json.Unmarshal(result.Fields[0], &metadata); err != nil {
			slog.Debug("Failed to parse app metadata JSON", "index", i, "error", err)
			continue
		}

		// Validate required fields
		if metadata.Version == "" {
			slog.Debug("App metadata missing version", "index", i)
			continue
		}
		if metadata.Name == "" {
			slog.Debug("App metadata missing name", "index", i)
			continue
		}
		if metadata.Description == "" {
			slog.Debug("App metadata missing description", "index", i)
			continue
		}
		if metadata.Icon == "" {
			slog.Debug("App metadata missing icon", "index", i)
			continue
		}
		if metadata.HTTPURL == "" && metadata.UHRPURL == "" {
			slog.Debug("App metadata missing both httpURL and uhrpURL", "index", i)
			continue
		}
		if metadata.Domain == "" {
			slog.Debug("App metadata missing domain", "index", i)
			continue
		}
		if metadata.Publisher == "" {
			slog.Debug("App metadata missing publisher", "index", i)
			continue
		}
		if metadata.ReleaseDate == "" {
			slog.Debug("App metadata missing release_date", "index", i)
			continue
		}

		// TODO: Check signature verification
		// The TypeScript version uses:
		// isTokenSignatureCorrectlyLinked(result.lockingPublicKey, metadata.publisher, result.fields)
		// This requires ProtoWallet.VerifySignature with protocol [1, 'metanet apps']
		// For now, we skip this check pending full wallet infrastructure in go-sdk

		// Output is valid
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	// Retain all previous coins
	for vin := range previousCoins {
		coinsToRetain = append(coinsToRetain, vin)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid Apps tokens found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  coinsToRetain,
		}, nil
	}

	slog.Info("Apps outputs admitted", "count", len(outputsToAdmit))

	if len(coinsToRetain) > 0 {
		slog.Info("Previous Apps coins retained", "count", len(coinsToRetain))
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for Apps protocol)
func (tm *AppsTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// Apps protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *AppsTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *AppsTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "Apps Topic Manager",
		Description: "Admits PushDrop tokens representing published Metanet Apps into an overlay.",
		Version:     "0.1.0",
	}
}
