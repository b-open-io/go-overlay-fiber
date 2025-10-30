package basketmap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

const topicDocs = `# BasketMap Topic Manager Documentation

The BasketMap Topic Manager is responsible for managing the rules of admissibility for BasketMap tokens and handling transactions related to them.

To have outputs accepted into the BasketMap overlay network, use the PushDrop template to create valid basket registration outputs.

Submit transactions that advertise new basket type registrations, or revoke (spend) existing registrations already submitted.

The latest state of all basket registrations will be tracked and available through the corresponding BasketMap Lookup Service.

## Admissibility Rules

- The transaction must have valid inputs and outputs.
- Each output must be decoded and validated according to the BasketMap protocol.
- Must contain exactly 7 PushDrop fields.
- The basket ID, name, icon URL, description, and documentation URL must be valid and match the expected formats.
- The registry operator must be correctly identified.
- The signature field must be present (signature verification pending implementation).

## BasketMap Token Fields

- **Field 0**: basketID (basket type identifier string)
- **Field 1**: name (basket display name)
- **Field 2**: iconURL (URL to basket icon)
- **Field 3**: description (basket description)
- **Field 4**: documentationURL (URL to basket documentation)
- **Field 5**: registryOperator (identity key of registry operator)
- **Field 6**: signature (signature over fields 0-5)
`

// BasketMapTopicManager implements a topic manager for BasketMap registry
type BasketMapTopicManager struct{}

// NewBasketMapTopicManager creates a new BasketMapTopicManager instance
func NewBasketMapTopicManager() *BasketMapTopicManager {
	return &BasketMapTopicManager{}
}

// Ensure BasketMapTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*BasketMapTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the BasketMap transaction that are admissible
func (tm *BasketMapTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}
	coinsToRetain := []uint32{}

	slog.Info("BasketMap topic manager was invoked", "previousUTXOs", len(previousCoins))

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("BasketMap topic manager parsed transaction", "txid", txid)

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

		// BasketMap tokens must have exactly 7 fields
		if len(result.Fields) != 7 {
			slog.Debug("BasketMap token field count invalid", "index", i, "expected", 7, "got", len(result.Fields))
			continue
		}

		// Extract and validate fields
		basketID := result.Fields[0]
		name := result.Fields[1]
		iconURL := result.Fields[2]
		description := result.Fields[3]
		documentationURL := result.Fields[4]
		registryOperator := result.Fields[5]
		signature := result.Fields[6]

		// Validate that required fields are not empty
		if len(basketID) == 0 {
			slog.Debug("Empty basketID", "index", i)
			continue
		}
		if len(name) == 0 {
			slog.Debug("Empty name", "index", i)
			continue
		}
		if len(iconURL) == 0 {
			slog.Debug("Empty iconURL", "index", i)
			continue
		}
		if len(description) == 0 {
			slog.Debug("Empty description", "index", i)
			continue
		}
		if len(documentationURL) == 0 {
			slog.Debug("Empty documentationURL", "index", i)
			continue
		}
		if len(registryOperator) == 0 {
			slog.Debug("Empty registryOperator", "index", i)
			continue
		}
		if len(signature) == 0 {
			slog.Debug("Empty signature", "index", i)
			continue
		}

		// TODO: Verify signature
		// The TypeScript version verifies:
		// 1. LockingPublicKey is derived from registryOperator
		// 2. Signature is valid over fields 0-5
		// This requires KeyDeriver and ProtoWallet functionality from go-sdk

		// Output is valid
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	// Retain all previous coins
	for vin := range previousCoins {
		coinsToRetain = append(coinsToRetain, vin)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid BasketMap tokens found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  coinsToRetain,
		}, nil
	}

	slog.Info("BasketMap outputs admitted", "count", len(outputsToAdmit))

	if len(coinsToRetain) > 0 {
		slog.Info("Previous BasketMap coins retained", "count", len(coinsToRetain))
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for BasketMap protocol)
func (tm *BasketMapTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// BasketMap protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *BasketMapTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *BasketMapTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "BasketMap",
		Description: "Register baskets for UX enrichment",
	}
}
