package slackthreads

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

const (
	// OP_SHA256 = 0xA8 (168)
	OP_SHA256 = 0xA8
	// OP_EQUAL = 0x87 (135)
	OP_EQUAL = 0x87
)

// SlackThreadsTopicManager implements the TopicManager interface for the SlackThreads protocol.
// It validates outputs that match the pattern: OP_SHA256 <32-byte hash> OP_EQUAL
type SlackThreadsTopicManager struct{}

// NewSlackThreadsTopicManager creates a new instance of SlackThreadsTopicManager
func NewSlackThreadsTopicManager() *SlackThreadsTopicManager {
	return &SlackThreadsTopicManager{}
}

// IdentifyAdmissibleOutputs identifies which outputs in the transaction are admissible
// based on the SlackThreads protocol rules.
func (tm *SlackThreadsTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins []uint32,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	// Parse the transaction from BEEF
	tx, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: outputsToAdmit,
			CoinsToRetain:  []uint32{},
		}, fmt.Errorf("failed to parse BEEF: %w", err)
	}

	if tx.Outputs == nil || len(tx.Outputs) == 0 {
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: outputsToAdmit,
			CoinsToRetain:  []uint32{},
		}, fmt.Errorf("missing parameter: outputs")
	}

	// Inspect every output
	for i, output := range tx.Outputs {
		if err := tm.validateOutput(output); err != nil {
			slog.Debug("Error processing output", "index", i, "error", err)
			continue
		}

		outputsToAdmit = append(outputsToAdmit, uint32(i))
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid SlackThreads outputs found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  []uint32{},
		}, nil
	}

	slog.Info("Admitted SlackThreads outputs", "count", len(outputsToAdmit))

	// The SlackThreads protocol never retains previous coins
	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  []uint32{},
	}, nil
}

// validateOutput validates that an output matches the SlackThreads pattern:
// OP_SHA256 <32-byte hash> OP_EQUAL
func (tm *SlackThreadsTopicManager) validateOutput(output *transaction.TransactionOutput) error {
	if output.LockingScript == nil {
		return fmt.Errorf("missing locking script")
	}

	chunks, err := output.LockingScript.ParseOps()
	if err != nil {
		return fmt.Errorf("failed to parse script: %w", err)
	}

	// Must have exactly 3 chunks
	if len(chunks) != 3 {
		return fmt.Errorf("invalid locking script: expected 3 chunks, got %d", len(chunks))
	}

	// First chunk must be OP_SHA256
	if chunks[0].Op != OP_SHA256 {
		return fmt.Errorf("invalid locking script: first chunk must be OP_SHA256")
	}

	// Second chunk must be a 32-byte data push
	if len(chunks[1].Data) != 32 {
		return fmt.Errorf("invalid locking script: second chunk must be 32-byte data push, got %d bytes", len(chunks[1].Data))
	}

	// Third chunk must be OP_EQUAL
	if chunks[2].Op != OP_EQUAL {
		return fmt.Errorf("invalid locking script: third chunk must be OP_EQUAL")
	}

	return nil
}

// IdentifyNeededInputs identifies which inputs are needed for validation.
// For SlackThreads, we don't need any specific inputs.
func (tm *SlackThreadsTopicManager) IdentifyNeededInputs(
	ctx context.Context,
	beef []byte,
) ([]*transaction.Outpoint, error) {
	return nil, nil
}

// GetDocumentation returns the documentation for the SlackThreads topic manager
func (tm *SlackThreadsTopicManager) GetDocumentation() string {
	return `# SlackThread Topic Manager Documentation

The **SlackThread Topic Manager** (topic ID: ` + "`tm_slackthread`" + `) lets clients Push hashes to chain and remove them by revealing the preimage.

Either there is a locking script of the form:

` + "```" + `asm
OP_SHA256 <32 byte hash> OP_EQUAL
` + "```" + `

or something which spends one of those locking scripts, with a preimage of any form in the unlocking script.
`
}

// GetMetaData returns metadata about the SlackThreads topic manager
func (tm *SlackThreadsTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "SlackThreads Topic Manager",
		Description: "Saves hashes of slack threads",
	}
}

// Ensure SlackThreadsTopicManager implements the TopicManager interface
var _ engine.TopicManager = (*SlackThreadsTopicManager)(nil)
