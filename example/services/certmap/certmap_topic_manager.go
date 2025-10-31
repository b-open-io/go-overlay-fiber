package certmap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
)

const topicDocs = `# CertMap Topic Manager Documentation

The **CertMap Topic Manager** defines which transaction outputs are
_admissible_ as certificate type registrations.

## Admissibility Rules

- The transaction must include at least one output.
- For an output to be admitted **all** of the following must hold:
  1. The locking script is a valid **PushDrop** script consisting of
     exactly **7 data fields** **plus** its signature field.
  2. The data fields decode to valid certificate type registration data.
  3. Required fields are present and non-empty:
     ` + "`type`" + `, ` + "`name`" + `, ` + "`iconURL`" + `, ` + "`description`" + `,
     ` + "`documentationURL`" + `, ` + "`certFields`" + `, and ` + "`registryOperator`" + `.
  4. The ` + "`certFields`" + ` field is valid JSON.
  5. The BRC-48 signature is valid for the claimed registry operator identity key
     using protocol [1, 'certmap'].
  6. The locking public key matches the expected derived key.

Outputs failing any check are ignored and **not** admitted.
`

// CertMapTopicManager implements a topic manager for CertMap name registry
type CertMapTopicManager struct{}

// NewCertMapTopicManager creates a new CertMapTopicManager instance
func NewCertMapTopicManager() *CertMapTopicManager {
	return &CertMapTopicManager{}
}

// Ensure CertMapTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*CertMapTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the CertMap transaction that are admissible
func (tm *CertMapTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}
	coinsToRetain := []uint32{}

	slog.Info("CertMap topic manager was invoked")

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("CertMap topic manager parsed transaction", "txid", txid)

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

		// CertMap tokens should have exactly 8 fields (7 data fields + signature)
		if len(result.Fields) != 8 {
			slog.Debug("CertMap token field count invalid", "index", i, "expected", 8, "got", len(result.Fields))
			continue
		}

		// Parse and validate certificate type registration data
		certType := string(result.Fields[0])
		name := string(result.Fields[1])
		iconURL := string(result.Fields[2])
		description := string(result.Fields[3])
		documentationURL := string(result.Fields[4])

		// Validate certFields is valid JSON
		var certFields map[string]interface{}
		if err := json.Unmarshal(result.Fields[5], &certFields); err != nil {
			slog.Debug("Failed to parse certFields JSON", "index", i, "error", err)
			continue
		}

		registryOperator := string(result.Fields[6])

		// Validate required fields
		if certType == "" {
			slog.Debug("CertMap registration missing type", "index", i)
			continue
		}
		if name == "" {
			slog.Debug("CertMap registration missing name", "index", i)
			continue
		}
		if iconURL == "" {
			slog.Debug("CertMap registration missing iconURL", "index", i)
			continue
		}
		if description == "" {
			slog.Debug("CertMap registration missing description", "index", i)
			continue
		}
		if documentationURL == "" {
			slog.Debug("CertMap registration missing documentationURL", "index", i)
			continue
		}
		if registryOperator == "" {
			slog.Debug("CertMap registration missing registryOperator", "index", i)
			continue
		}

		// Verify signature
		if err := tm.verifySignature(result.LockingPublicKey, registryOperator, result.Fields); err != nil {
			slog.Debug("Signature verification failed", "index", i, "error", err)
			continue
		}

		slog.Debug("CertMap signature verified", "index", i)

		// Output is valid
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	// Retain all previous coins
	for vin := range previousCoins {
		coinsToRetain = append(coinsToRetain, vin)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid CertMap tokens found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  coinsToRetain,
		}, nil
	}

	slog.Info("CertMap outputs admitted", "count", len(outputsToAdmit))

	if len(coinsToRetain) > 0 {
		slog.Info("Previous CertMap coins retained", "count", len(coinsToRetain))
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// verifySignature verifies the CertMap registration signature using BRC-48
func (tm *CertMapTopicManager) verifySignature(lockingPublicKey *ec.PublicKey, registryOperatorHex string, fields [][]byte) error {
	// Parse registry operator identity key from hex string
	registryOperatorKey, err := ec.PublicKeyFromString(registryOperatorHex)
	if err != nil {
		return fmt.Errorf("invalid registry operator key: %w", err)
	}

	// The signature is the last field
	if len(fields) < 2 {
		return fmt.Errorf("not enough fields for signature verification")
	}
	signatureBytes := fields[len(fields)-1]
	dataFields := fields[:len(fields)-1]

	// Concatenate data fields for signature verification (all except signature)
	var data bytes.Buffer
	for _, field := range dataFields {
		data.Write(field)
	}

	// Parse signature
	sig, err := ec.ParseSignature(signatureBytes)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	// Create "anyone" wallet for BRC-48 verification
	anyoneWallet, err := wallet.NewProtoWallet(wallet.ProtoWalletArgs{
		Type: wallet.ProtoWalletArgsTypeAnyone,
	})
	if err != nil {
		return fmt.Errorf("failed to create anyone wallet: %w", err)
	}

	// Verify signature using BRC-48 protocol
	// Protocol ID matches TypeScript: [1, 'certmap']
	verifyArgs := wallet.VerifySignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "certmap",
			},
			KeyID: "1",
			Counterparty: wallet.Counterparty{
				Type:         wallet.CounterpartyTypeOther,
				Counterparty: registryOperatorKey,
			},
		},
		Data:      data.Bytes(),
		Signature: sig,
	}

	result, err := anyoneWallet.VerifySignature(
		context.Background(),
		verifyArgs,
		"",
	)
	if err != nil {
		return fmt.Errorf("signature verification error: %w", err)
	}

	if !result.Valid {
		return fmt.Errorf("signature does not match registry operator key via BRC-48")
	}

	// Verify the locking public key matches the expected derived key
	publicKeyArgs := wallet.GetPublicKeyArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "certmap",
			},
			KeyID: "1",
			Counterparty: wallet.Counterparty{
				Type:         wallet.CounterpartyTypeOther,
				Counterparty: registryOperatorKey,
			},
		},
	}

	derivedKey, err := anyoneWallet.GetPublicKey(
		context.Background(),
		publicKeyArgs,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to derive expected locking key: %w", err)
	}

	// Compare locking public keys
	if lockingPublicKey.ToDERHex() != derivedKey.PublicKey.ToDERHex() {
		return fmt.Errorf("locking public key does not match expected derived key")
	}

	return nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for CertMap protocol)
func (tm *CertMapTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// CertMap protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *CertMapTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *CertMapTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "CertMap Topic Manager",
		Description: "Admits PushDrop tokens representing certificate type registrations into an overlay.",
		Version:     "0.1.0",
	}
}
