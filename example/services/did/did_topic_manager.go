package did

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

const topicDocs = `# DID Topic Manager Documentation

The DID Topic Manager is responsible for managing the rules of admissibility for DID tokens.

## Admissibility Rules

- The transaction must have valid inputs and outputs.
- Each output must be decoded and validated according to the DID token protocol.
- The serial number must be a valid base64 string.

For more details, refer to the official DID protocol documentation.
`

// DIDTopicManager implements a topic manager for DID tokens
type DIDTopicManager struct{}

// NewDIDTopicManager creates a new DIDTopicManager instance
func NewDIDTopicManager() *DIDTopicManager {
	return &DIDTopicManager{}
}

// Ensure DIDTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*DIDTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the DID transaction that are admissible
func (tm *DIDTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	slog.Info("DID topic manager was invoked", "previousUTXOs", len(previousCoins))

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("DID topic manager parsed transaction", "txid", txid)

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

		// Check that there is exactly one field + signature (2 fields total)
		if len(result.Fields) != 2 {
			slog.Debug("DID token field count invalid", "index", i, "expected", 2, "got", len(result.Fields))
			continue
		}

		// Extract serial number (convert to UTF-8 string)
		serialNumber := string(result.Fields[0])

		if serialNumber == "" {
			slog.Debug("DID token missing serial number", "index", i)
			continue
		}

		// Output is valid
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	if len(outputsToAdmit) == 0 && len(previousCoins) == 0 {
		slog.Debug("DID topic manager: no outputs admitted and no previous coins consumed")
		return overlay.AdmittanceInstructions{}, fmt.Errorf("no outputs admitted")
	}

	if len(outputsToAdmit) > 0 {
		slog.Info("DID outputs admitted", "count", len(outputsToAdmit))
	}

	if len(previousCoins) > 0 {
		slog.Info("Consumed previous DID coins", "count", len(previousCoins))
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  []uint32{},
	}, nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for DID protocol)
func (tm *DIDTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// DID protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *DIDTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *DIDTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "DID Topic Manager",
		Description: "DID Resolution Protocol",
	}
}
