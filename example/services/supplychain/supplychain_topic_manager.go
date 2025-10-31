package supplychain

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

const topicDocs = `# SupplyChain Topic Manager Documentation

The **SupplyChain Topic Manager** validates transaction outputs for supply chain
tracking using a simple PushDrop-like script pattern with off-chain values.

## Admissibility Rules

For an output to be admitted:
1. The locking script must have exactly 5 chunks
2. Chunk 0: Data push (metadata)
3. Chunk 1: Data push (additional data)
4. Chunk 2: OP_2DROP
5. Chunk 3: 33-byte public key
6. Chunk 4: OP_CHECKSIG

Outputs failing validation are ignored and **not** admitted.

## Off-Chain Values

The protocol relies on off-chain values (provided separately from the locking script)
that contain supply chain metadata including:
- ` + "`chainId`" + `: Required supply chain identifier
- Additional custom fields as needed

The off-chain values are stored as JSON and indexed by chainId.
`

// SupplyChainTopicManager implements a topic manager for SupplyChain protocol
type SupplyChainTopicManager struct{}

// NewSupplyChainTopicManager creates a new SupplyChainTopicManager instance
func NewSupplyChainTopicManager() *SupplyChainTopicManager {
	return &SupplyChainTopicManager{}
}

// Ensure SupplyChainTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*SupplyChainTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the SupplyChain transaction that are admissible
func (tm *SupplyChainTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	slog.Info("SupplyChain topic manager was invoked")

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("SupplyChain topic manager parsed transaction", "txid", txid)

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

		slog.Debug("SupplyChain output admitted", "index", i)
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid SupplyChain outputs found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  []uint32{},
		}, nil
	}

	slog.Info("SupplyChain outputs admitted", "count", len(outputsToAdmit))

	// The SupplyChain protocol never retains previous coins
	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  []uint32{},
	}, nil
}

// validateOutput validates that an output matches the SupplyChain pattern
func (tm *SupplyChainTopicManager) validateOutput(output *transaction.TransactionOutput) error {
	if output.LockingScript == nil {
		return fmt.Errorf("missing locking script")
	}

	chunks, err := output.LockingScript.ParseOps()
	if err != nil {
		return fmt.Errorf("failed to parse script: %w", err)
	}

	// PushDrop script has length of 5
	if len(chunks) != 5 {
		return fmt.Errorf("invalid locking script: expected 5 chunks, got %d", len(chunks))
	}

	// Chunk 0: PushDrop metadata (must be data push)
	if chunks[0].Data == nil || len(chunks[0].Data) == 0 {
		return fmt.Errorf("invalid locking script: chunk 0 must be data push")
	}

	// Chunk 1: Additional data (must be data push)
	if chunks[1].Data == nil || len(chunks[1].Data) == 0 {
		return fmt.Errorf("invalid locking script: chunk 1 must be data push")
	}

	// Chunk 2: OP_2DROP
	if chunks[2].Op != script.Op2DROP {
		return fmt.Errorf("invalid locking script: chunk 2 must be OP_2DROP")
	}

	// Chunk 3: Public key (must be 33 bytes)
	if chunks[3].Data == nil || len(chunks[3].Data) != 33 {
		return fmt.Errorf("invalid locking script: chunk 3 must be 33-byte public key")
	}

	// Chunk 4: OP_CHECKSIG
	if chunks[4].Op != script.OpCHECKSIG {
		return fmt.Errorf("invalid locking script: chunk 4 must be OP_CHECKSIG")
	}

	return nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for SupplyChain protocol)
func (tm *SupplyChainTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// SupplyChain protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *SupplyChainTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *SupplyChainTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "SupplyChain Topic Manager",
		Description: "Saves hashes of files and integrity off-chain values",
		Version:     "0.1.0",
	}
}
