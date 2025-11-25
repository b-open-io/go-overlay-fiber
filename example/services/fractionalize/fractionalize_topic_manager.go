package fractionalize

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

const topicDocs = `# Fractionalize Topic Manager Documentation

The **Fractionalize Topic Manager** validates transaction outputs for the
fractionalized ownership proof-of-concept protocol.

## Admissibility Rules

This topic manager validates three types of outputs based on their locking scripts:

### 1. Server Token (Ordinal + MultiSig)
- Contains both OP_IF (ordinal) and OP_CHECKMULTISIG (multisig)
- Represents token mint or server change output
- Validates against specific script template

### 2. Transfer Token (Ordinal only)
- Contains OP_IF but not OP_CHECKMULTISIG
- Represents token transfer to a user
- Validates against specific script template

### 3. Payment (MultiSig only)
- Contains OP_CHECKMULTISIG but not OP_IF
- Represents payment output
- Validates against specific script template

Outputs failing validation are ignored and **not** admitted.
`

// Script templates for validation (with zeroed-out hash values)
const (
	serverTokenTemplate  = "00630300000000000000000000000000000000000000005112000000000000000000000000000000000000000000240000000000000000000000000000000000000000686e7ea9140000000000000000000000000000000000000000886b6b516c6c52ae6a200000000000000000000000000000000000000000"
	transferTokenTemplate = "006303000000000000000000000000000000000000000051120000000000000000000000000000000000000000002400000000000000000000000000000000000000006876a914000000000000000000000000000000000000000088ac6a200000000000000000000000000000000000000000"
	paymentTemplate      = "6e7ea9140000000000000000000000000000000000000000886b6b516c6c52ae"
)

// FractionalizeTopicManager implements a topic manager for Fractionalize protocol
type FractionalizeTopicManager struct{}

// NewFractionalizeTopicManager creates a new FractionalizeTopicManager instance
func NewFractionalizeTopicManager() *FractionalizeTopicManager {
	return &FractionalizeTopicManager{}
}

// Ensure FractionalizeTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*FractionalizeTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the Fractionalize transaction that are admissible
func (tm *FractionalizeTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins []uint32,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	slog.Info("Fractionalize topic manager was invoked")

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("Fractionalize topic manager parsed transaction", "txid", txid)

	// Validate params
	if len(parsedTransaction.Outputs) < 1 {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("missing parameter: outputs")
	}

	// Check each output's lockingScript and verify the format
	for i, output := range parsedTransaction.Outputs {
		outputIndex := uint32(i)

		// Check output type by looking for specific opcodes
		lockingScript := output.LockingScript
		if lockingScript == nil {
			slog.Debug("Output has no locking script", "index", i)
			continue
		}

		hasOrdinal := containsOpCode(lockingScript, script.OpIF)
		hasMultiSig := containsOpCode(lockingScript, script.OpCHECKMULTISIG)

		// If both true, this is an ordinal token mint or server change output
		if hasOrdinal && hasMultiSig {
			if checkScriptFormat(lockingScript, serverTokenTemplate) {
				slog.Debug("Server token output admitted", "index", i)
				outputsToAdmit = append(outputsToAdmit, outputIndex)
			} else {
				slog.Debug("Server token validation failed", "index", i)
			}
		} else if hasOrdinal && !hasMultiSig {
			// If only ordinal is true, this is a token transfer to a user
			if checkScriptFormat(lockingScript, transferTokenTemplate) {
				slog.Debug("Transfer token output admitted", "index", i)
				outputsToAdmit = append(outputsToAdmit, outputIndex)
			} else {
				slog.Debug("Transfer token validation failed", "index", i)
			}
		} else if hasMultiSig && !hasOrdinal {
			// If only multisig is true, this is a payment output
			if checkScriptFormat(lockingScript, paymentTemplate) {
				slog.Debug("Payment output admitted", "index", i)
				outputsToAdmit = append(outputsToAdmit, outputIndex)
			} else {
				slog.Debug("Payment validation failed", "index", i)
			}
		}
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid Fractionalize outputs found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  []uint32{},
		}, nil
	}

	slog.Info("Fractionalize outputs admitted", "count", len(outputsToAdmit))

	// The Fractionalize protocol never retains previous coins
	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  []uint32{},
	}, nil
}

// containsOpCode checks if a script contains a specific opcode
func containsOpCode(s *script.Script, opcode byte) bool {
	chunks, err := s.ParseOps()
	if err != nil {
		return false
	}
	for _, chunk := range chunks {
		if chunk.Op == opcode {
			return true
		}
	}
	return false
}

// checkScriptFormat validates a script against a template by blanking out variable data
func checkScriptFormat(s *script.Script, template string) bool {
	chunks, err := s.ParseOps()
	if err != nil {
		return false
	}

	// Create a new script with blanked data
	blankedScript := &script.Script{}
	for _, chunk := range chunks {
		// Blank out data that is not exactly one byte with value 0x21 (33)
		// This matches the TypeScript logic that preserves [33] but zeros out other data
		if chunk.Data != nil {
			if len(chunk.Data) != 1 || chunk.Data[0] != 0x21 {
				// Zero out the data
				blankedData := make([]byte, len(chunk.Data))
				blankedScript.AppendOpcodes(chunk.Op)
				blankedScript.AppendPushData(blankedData)
			} else {
				// Preserve [33]
				blankedScript.AppendOpcodes(chunk.Op)
				blankedScript.AppendPushData(chunk.Data)
			}
		} else {
			blankedScript.AppendOpcodes(chunk.Op)
		}
	}

	// Convert to hex and compare with template
	scriptHex := hex.EncodeToString(*blankedScript)

	return scriptHex == template
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for Fractionalize protocol)
func (tm *FractionalizeTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// Fractionalize protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *FractionalizeTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *FractionalizeTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "Fractionalize Topic Manager",
		Description: "Fractionalize topic manager for the fractionalized ownership PoC",
		Version:     "0.1.0",
	}
}
