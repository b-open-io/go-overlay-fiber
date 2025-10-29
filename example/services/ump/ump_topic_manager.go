package ump

import (
	"context"
	"fmt"
	"log"

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

	log.Printf("UMP topic manager was invoked with %d previous UTXOs", len(previousCoins))

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	log.Printf("UMP topic manager has parsed the transaction: %s", txid)

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
			log.Printf("Output %d: failed to decode PushDrop", i)
			continue
		}

		// UMP tokens must have at least 11 fields
		if len(result.Fields) < 11 {
			log.Printf("Output %d: UMP token must have at least 11 fields, got %d fields", i, len(result.Fields))
			continue
		}

		// Extract presentationHash (field 6) and recoveryHash (field 7) for validation
		presentationHash := result.Fields[6]
		recoveryHash := result.Fields[7]

		// Both hashes should be 32 bytes (256 bits)
		if len(presentationHash) != 32 {
			log.Printf("Output %d: presentationHash must be 32 bytes, got %d bytes", i, len(presentationHash))
			continue
		}

		if len(recoveryHash) != 32 {
			log.Printf("Output %d: recoveryHash must be 32 bytes, got %d bytes", i, len(recoveryHash))
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
		log.Printf("UMP topic manager: no valid UMP tokens found")
		return overlay.AdmittanceInstructions{}, fmt.Errorf("no valid UMP tokens found")
	}

	if len(outputsToAdmit) > 0 {
		log.Printf("Admitted %d UMP output(s)", len(outputsToAdmit))
	}

	if len(coinsToRetain) > 0 {
		log.Printf("Retained %d previous UMP coin(s)", len(coinsToRetain))
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
