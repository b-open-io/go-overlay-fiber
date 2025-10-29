package ump

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

const topicDocs = `# User Management Protocol Topic Manager Docs

To have outputs accepted into the UMP overlay network, use PushDrop to create valid locking scripts with UMP token fields.

Submit transactions that create new UMP tokens or consume existing ones.

The latest state of all UMP tokens will be tracked and available through the corresponding UMP Lookup Service.

UMP tokens contain the following fields:
- Field 0: passwordSalt
- Field 1: passwordPresentationPrimary
- Field 2: passwordRecoveryPrimary
- Field 3: presentationRecoveryPrimary
- Field 4: passwordPrimaryPrivileged
- Field 5: presentationRecoveryPrivileged
- Field 6: presentationHash (used for lookups)
- Field 7: recoveryHash (used for lookups)
- Field 8: presentationKeyEncrypted
- Field 9: passwordKeyEncrypted
- Field 10: recoveryKeyEncrypted
- Field 11 (optional): profilesEncrypted

Valid UMP tokens must have at least 11 fields in PushDrop format.
`

// UMPTopicManager implements a topic manager for User Management Protocol
type UMPTopicManager struct{}

// NewUMPTopicManager creates a new UMPTopicManager instance
func NewUMPTopicManager() *UMPTopicManager {
	return &UMPTopicManager{}
}

// Ensure UMPTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*UMPTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the UMP transaction that are admissible
func (tm *UMPTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}
	coinsToRetain := []uint32{}

	slog.Info("UMP topic manager was invoked", "previousUTXOs", len(previousCoins))

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("UMP topic manager parsed transaction", "txid", txid)

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

		// UMP tokens must have at least 11 fields
		if len(result.Fields) < 11 {
			slog.Debug("UMP token field count invalid", "index", i, "expected", ">=11", "got", len(result.Fields))
			continue
		}

		// Extract presentationHash (field 6) and recoveryHash (field 7) for validation
		presentationHash := result.Fields[6]
		recoveryHash := result.Fields[7]

		// Both hashes should be 32 bytes (256 bits)
		if len(presentationHash) != 32 {
			slog.Debug("presentationHash length invalid", "index", i, "expected", 32, "got", len(presentationHash))
			continue
		}

		if len(recoveryHash) != 32 {
			slog.Debug("recoveryHash length invalid", "index", i, "expected", 32, "got", len(recoveryHash))
			continue
		}

		// Output is valid
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	// Retain all previous coins (UMP tokens can be consumed and replaced)
	for vin := range previousCoins {
		coinsToRetain = append(coinsToRetain, vin)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid UMP tokens found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  coinsToRetain,
		}, nil
	}

	slog.Info("UMP outputs admitted", "count", len(outputsToAdmit))

	if len(coinsToRetain) > 0 {
		slog.Info("Previous UMP coins retained", "count", len(coinsToRetain))
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for UMP protocol)
func (tm *UMPTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// UMP protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *UMPTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *UMPTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "User Management Protocol",
		Description: "Manages CWI-style wallet account descriptors.",
	}
}
