package walletconfig

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
)

const topicDocs = `# WalletConfig Topic Manager Documentation

The **WalletConfig Topic Manager** defines which transaction outputs are
_admissible_ as wallet configuration registrations.

## Admissibility Rules

- The transaction must include at least one output.
- For an output to be admitted **all** of the following must hold:
  1. The locking script is a valid **PushDrop** script consisting of
     exactly **8 data fields** **plus** its signature field.
  2. The data fields decode to valid wallet configuration data.
  3. Required fields are present and non-empty:
     ` + "`configID`" + `, ` + "`name`" + `, ` + "`icon`" + `, ` + "`wab`" + `,
     ` + "`storage`" + `, ` + "`messagebox`" + `, ` + "`legal`" + `, and ` + "`registryOperator`" + `.
  4. The BRC-48 signature is valid for the claimed registry operator identity key
     using protocol [1, 'wallet config option'].
  5. The locking public key matches the expected derived key.

Outputs failing any check are ignored and **not** admitted.
`

// WalletConfigTopicManager implements a topic manager for WalletConfig registry
type WalletConfigTopicManager struct{}

// NewWalletConfigTopicManager creates a new WalletConfigTopicManager instance
func NewWalletConfigTopicManager() *WalletConfigTopicManager {
	return &WalletConfigTopicManager{}
}

// Ensure WalletConfigTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*WalletConfigTopicManager)(nil)

// IdentifyAdmissibleOutputs returns the outputs from the WalletConfig transaction that are admissible
func (tm *WalletConfigTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins []uint32,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}
	coinsToRetain := []uint32{}

	slog.Info("WalletConfig topic manager was invoked")

	// Parse the transaction from BEEF
	parsedTransaction, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	txid := parsedTransaction.TxID()
	slog.Debug("WalletConfig topic manager parsed transaction", "txid", txid)

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

		// WalletConfig tokens should have exactly 9 fields (8 data fields + signature)
		if len(result.Fields) != 9 {
			slog.Debug("WalletConfig token field count invalid", "index", i, "expected", 9, "got", len(result.Fields))
			continue
		}

		// Parse and validate wallet configuration data
		configID := string(result.Fields[0])
		name := string(result.Fields[1])
		icon := string(result.Fields[2])
		wab := string(result.Fields[3])
		storage := string(result.Fields[4])
		messagebox := string(result.Fields[5])
		legal := string(result.Fields[6])
		registryOperator := string(result.Fields[7])

		// Validate required fields
		if configID == "" {
			slog.Debug("WalletConfig registration missing configID", "index", i)
			continue
		}
		if name == "" {
			slog.Debug("WalletConfig registration missing name", "index", i)
			continue
		}
		if icon == "" {
			slog.Debug("WalletConfig registration missing icon", "index", i)
			continue
		}
		if wab == "" {
			slog.Debug("WalletConfig registration missing wab", "index", i)
			continue
		}
		if storage == "" {
			slog.Debug("WalletConfig registration missing storage", "index", i)
			continue
		}
		if messagebox == "" {
			slog.Debug("WalletConfig registration missing messagebox", "index", i)
			continue
		}
		if legal == "" {
			slog.Debug("WalletConfig registration missing legal", "index", i)
			continue
		}
		if registryOperator == "" {
			slog.Debug("WalletConfig registration missing registryOperator", "index", i)
			continue
		}

		// Verify signature
		if err := tm.verifySignature(result.LockingPublicKey, registryOperator, result.Fields); err != nil {
			slog.Debug("Signature verification failed", "index", i, "error", err)
			continue
		}

		slog.Debug("WalletConfig signature verified", "index", i)

		// Output is valid
		outputsToAdmit = append(outputsToAdmit, outputIndex)
	}

	// Retain all previous coins
	for _, vin := range previousCoins {
		coinsToRetain = append(coinsToRetain, vin)
	}

	if len(outputsToAdmit) == 0 {
		slog.Debug("No valid WalletConfig tokens found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  coinsToRetain,
		}, nil
	}

	slog.Info("WalletConfig outputs admitted", "count", len(outputsToAdmit))

	if len(coinsToRetain) > 0 {
		slog.Info("Previous WalletConfig coins retained", "count", len(coinsToRetain))
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// verifySignature verifies the WalletConfig registration signature using BRC-48
func (tm *WalletConfigTopicManager) verifySignature(lockingPublicKey *ec.PublicKey, registryOperatorHex string, fields [][]byte) error {
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
	// Protocol ID matches TypeScript: [1, 'wallet config option']
	verifyArgs := wallet.VerifySignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "wallet config option",
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
				Protocol:      "wallet config option",
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

// IdentifyNeededInputs identifies inputs needed for validation (not used for WalletConfig protocol)
func (tm *WalletConfigTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// WalletConfig protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation for this topic manager
func (tm *WalletConfigTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about this topic manager
func (tm *WalletConfigTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "WalletConfig Topic Manager",
		Description: "Admits PushDrop tokens representing wallet configuration options into an overlay.",
		Version:     "0.1.0",
	}
}
