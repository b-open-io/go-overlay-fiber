package desktopintegrity

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

const topicDocs = `# DesktopIntegrity Topic Manager Documentation

The **DesktopIntegrity Topic Manager** (topic ID: ` + "`tm_desktopintegrity`" + `) lets clients push hashes to chain.

Links hashes to off-chain values for future lookup.

Locking script example:

` + "```asm" + `
OP_FALSE OP_RETURN <32 byte hash>
` + "```" + `
`

// DesktopIntegrityTopicManager implements a topic manager for DesktopIntegrity protocol
type DesktopIntegrityTopicManager struct{}

// NewDesktopIntegrityTopicManager creates a new DesktopIntegrityTopicManager instance
func NewDesktopIntegrityTopicManager() *DesktopIntegrityTopicManager {
	return &DesktopIntegrityTopicManager{}
}

// Ensure DesktopIntegrityTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*DesktopIntegrityTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the DesktopIntegrity transaction that are admissible
func (tm *DesktopIntegrityTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins []uint32,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	slog.Info("DesktopIntegrity topic manager was invoked")

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("DesktopIntegrity topic manager parsed transaction", "txid", txid)

	// Validate params
	if len(parsedTransaction.Outputs) < 1 {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("missing parameter: outputs")
	}

	// Inspect every output
	for i, output := range parsedTransaction.Outputs {
		outputIndex := uint32(i)

		if err := tm.validateOutput(output); err != nil {
			slog.Debug("Output validation failed", "index", i, "error", err)
			continue
		}

		slog.Debug("DesktopIntegrity output admitted", "index", i)
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid DesktopIntegrity outputs found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  []uint32{},
		}, nil
	}

	slog.Info("DesktopIntegrity outputs admitted", "count", len(outputsToAdmit))

	// The DesktopIntegrity protocol never retains previous coins
	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  []uint32{},
	}, nil
}

// validateOutput validates that an output matches the DesktopIntegrity pattern
// Pattern: OP_FALSE OP_RETURN <32-byte hash>
func (tm *DesktopIntegrityTopicManager) validateOutput(output *transaction.TransactionOutput) error {
	if output.LockingScript == nil {
		return fmt.Errorf("missing locking script")
	}

	chunks, err := output.LockingScript.ParseOps()
	if err != nil {
		return fmt.Errorf("failed to parse script: %w", err)
	}

	// Must have exactly 2 chunks: OP_FALSE OP_RETURN
	if len(chunks) != 2 {
		return fmt.Errorf("invalid locking script: expected 2 chunks, got %d", len(chunks))
	}

	// Chunk 0: OP_FALSE
	if chunks[0].Op != script.OpFALSE {
		return fmt.Errorf("invalid locking script: chunk 0 must be OP_FALSE")
	}

	// Chunk 1: OP_RETURN
	if chunks[1].Op != script.OpRETURN {
		return fmt.Errorf("invalid locking script: chunk 1 must be OP_RETURN")
	}

	return nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for DesktopIntegrity protocol)
func (tm *DesktopIntegrityTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// DesktopIntegrity protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *DesktopIntegrityTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *DesktopIntegrityTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "DesktopIntegrity Topic Manager",
		Description: "Saves hashes of files and integrity off-chain values",
		Version:     "0.1.0",
	}
}
