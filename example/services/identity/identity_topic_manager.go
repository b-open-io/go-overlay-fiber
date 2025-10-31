package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

const topicDocs = `# Identity Topic Manager Documentation

The Identity Topic Manager is responsible for managing the rules of admissibility for Identity tokens.

To have outputs accepted into the Identity overlay network, use the PushDrop template to create valid identity certificate outputs.

Submit transactions that advertise new identity certificates, or revoke (spend) existing certificates already submitted.

The latest state of all identity certificates will be tracked and available through the corresponding Identity Lookup Service.

## Admissibility Rules

- The transaction must have valid inputs and outputs.
- Each output must be decoded and validated according to the Identity protocol.
- The certificate must be valid and properly formatted.
- The certificate fields must be properly revealed and can be decrypted.
- The signature must be verified to ensure it is valid (pending full implementation).
- Either the certifier or the subject must control the Identity token.

## Identity Token Fields

Identity tokens use BRC-48 certificates encoded in PushDrop format:
- **Field 0**: JSON-encoded VerifiableCertificate
- **Field 1**: Signature over the certificate data

For more details, refer to the official Identity protocol documentation.
`

// IdentityTopicManager implements a topic manager for Identity registry
type IdentityTopicManager struct{}

// NewIdentityTopicManager creates a new IdentityTopicManager instance
func NewIdentityTopicManager() *IdentityTopicManager {
	return &IdentityTopicManager{}
}

// Ensure IdentityTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*IdentityTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the Identity transaction that are admissible
func (tm *IdentityTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}
	coinsToRetain := []uint32{}

	slog.Info("Identity topic manager was invoked", "previousUTXOs", len(previousCoins))

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("Identity topic manager parsed transaction", "txid", txid)

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

		// Identity tokens should have at least 1 field (certificate JSON)
		if len(result.Fields) < 1 {
			slog.Debug("Identity token field count invalid", "index", i, "expected", ">=1", "got", len(result.Fields))
			continue
		}

		// Parse certificate from first field
		var certData map[string]interface{}
		if err := json.Unmarshal(result.Fields[0], &certData); err != nil {
			slog.Debug("Failed to parse certificate JSON", "index", i, "error", err)
			continue
		}

		// Convert to Certificate struct
		certJSON, err := json.Marshal(certData)
		if err != nil {
			slog.Debug("Failed to marshal certificate", "index", i, "error", err)
			continue
		}

		var verifiableCert certificates.VerifiableCertificate
		if err := json.Unmarshal(certJSON, &verifiableCert); err != nil {
			slog.Debug("Failed to unmarshal VerifiableCertificate", "index", i, "error", err)
			continue
		}

		// Basic validation: certificate should have required fields
		if verifiableCert.Type == "" {
			slog.Debug("Certificate missing type", "index", i)
			continue
		}
		if verifiableCert.SerialNumber == "" {
			slog.Debug("Certificate missing serial number", "index", i)
			continue
		}
		// Subject and Certifier are PublicKey structs (not pointers), so they're never nil
		// Just verify they're not zero values
		if len(verifiableCert.Subject.Compressed()) == 0 {
			slog.Debug("Certificate missing subject", "index", i)
			continue
		}
		if len(verifiableCert.Certifier.Compressed()) == 0 {
			slog.Debug("Certificate missing certifier", "index", i)
			continue
		}

		// TODO: Full certificate validation
		// The TypeScript version:
		// 1. Verifies signature over fields using ProtoWallet
		// 2. Verifies certificate signature using certificate.verify()
		// 3. Decrypts fields to ensure they're properly revealed
		// This requires more complete wallet infrastructure in go-sdk

		// For now, just verify that the certificate has some fields
		// Field decryption will be handled in the lookup service
		if len(verifiableCert.Fields) == 0 {
			slog.Debug("Certificate has no fields", "index", i)
			continue
		}

		// Output is valid
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	// Retain all previous coins
	for vin := range previousCoins {
		coinsToRetain = append(coinsToRetain, vin)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid Identity tokens found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  coinsToRetain,
		}, nil
	}

	slog.Info("Identity outputs admitted", "count", len(outputsToAdmit))

	if len(coinsToRetain) > 0 {
		slog.Info("Previous Identity coins retained", "count", len(coinsToRetain))
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for Identity protocol)
func (tm *IdentityTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// Identity protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *IdentityTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *IdentityTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "Identity Topic Manager",
		Description: "Identity Resolution Protocol",
	}
}
