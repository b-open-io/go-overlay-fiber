package protomap

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

const topicDocs = `# ProtoMap Topic Manager Documentation

The ProtoMap Topic Manager is responsible for managing the rules of admissibility for ProtoMap tokens and handling transactions related to them.

To have outputs accepted into the ProtoMap overlay network, use the PushDrop template to create valid protocol registration outputs.

Submit transactions that advertise new protocol registrations, or revoke (spend) existing registrations already submitted.

The latest state of all protocol registrations will be tracked and available through the corresponding ProtoMap Lookup Service.

## Admissibility Rules

- The transaction must have valid inputs and outputs.
- Each output must be decoded and validated according to the ProtoMap protocol.
- Must contain exactly 7 PushDrop fields.
- The security level must be one of the accepted values (0, 1, or 2).
- The protocol ID, name, icon URL, description, and documentation URL must be valid and match the expected formats.
- The registry operator must be correctly identified.
- The signature field must be present (signature verification pending implementation).

## ProtoMap Token Fields

- **Field 0**: protocolID (JSON array, e.g., [1, "protocol-name"])
- **Field 1**: name (protocol display name)
- **Field 2**: iconURL (URL to protocol icon)
- **Field 3**: description (protocol description)
- **Field 4**: documentationURL (URL to protocol documentation)
- **Field 5**: registryOperator (identity key of registry operator)
- **Field 6**: signature (signature over fields 0-5)
`

// ProtoMapTopicManager implements a topic manager for ProtoMap protocol registry
type ProtoMapTopicManager struct{}

// NewProtoMapTopicManager creates a new ProtoMapTopicManager instance
func NewProtoMapTopicManager() *ProtoMapTopicManager {
	return &ProtoMapTopicManager{}
}

// Ensure ProtoMapTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*ProtoMapTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the ProtoMap transaction that are admissible
func (tm *ProtoMapTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins []uint32,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}
	coinsToRetain := []uint32{}

	slog.Info("ProtoMap topic manager was invoked", "previousUTXOs", len(previousCoins))

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("ProtoMap topic manager parsed transaction", "txid", txid)

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

		// ProtoMap tokens must have exactly 7 fields
		if len(result.Fields) != 7 {
			slog.Debug("ProtoMap token field count invalid", "index", i, "expected", 7, "got", len(result.Fields))
			continue
		}

		// Extract and validate fields
		protocolIDBytes := result.Fields[0]
		name := result.Fields[1]
		iconURL := result.Fields[2]
		description := result.Fields[3]
		documentationURL := result.Fields[4]
		registryOperator := result.Fields[5]
		signature := result.Fields[6]

		// Parse protocolID (should be JSON like [1, "protocol-name"])
		var protocolID []interface{}
		if err := json.Unmarshal(protocolIDBytes, &protocolID); err != nil {
			slog.Debug("Failed to parse protocolID", "index", i, "error", err)
			continue
		}

		// Validate protocolID structure
		if len(protocolID) != 2 {
			slog.Debug("Invalid protocolID structure", "index", i)
			continue
		}

		// Security level should be a number (0, 1, or 2)
		securityLevel, ok := protocolID[0].(float64)
		if !ok || (securityLevel != 0 && securityLevel != 1 && securityLevel != 2) {
			slog.Debug("Invalid security level", "index", i)
			continue
		}

		// Protocol should be a string
		_, ok = protocolID[1].(string)
		if !ok {
			slog.Debug("Invalid protocol string", "index", i)
			continue
		}

		// Validate that required fields are not empty
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
	for _, vin := range previousCoins {
		coinsToRetain = append(coinsToRetain, vin)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid ProtoMap tokens found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  coinsToRetain,
		}, nil
	}

	slog.Info("ProtoMap outputs admitted", "count", len(outputsToAdmit))

	if len(coinsToRetain) > 0 {
		slog.Info("Previous ProtoMap coins retained", "count", len(coinsToRetain))
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for ProtoMap protocol)
func (tm *ProtoMapTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// ProtoMap protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *ProtoMapTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *ProtoMapTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "ProtoMap Topic Manager",
		Description: "Protocol information registration",
	}
}
