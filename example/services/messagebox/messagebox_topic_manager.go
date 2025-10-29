package messagebox

import (
	"context"
	"fmt"
	"log"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

const topicDocs = `
# MessageBox Topic Manager

The **MessageBox Topic Manager** defines SHIP overlay admittance rules for the ` + "`tm_messagebox`" + ` topic. It ensures that only properly signed and structured advertisements from identity keys are admitted into the overlay network.

## Overview

This Topic Manager is responsible for filtering and validating outputs that represent host advertisements. Each advertisement is a PushDrop-encoded output containing an identityKey, host URL, and a digital signature. The Topic Manager ensures that only valid advertisements signed by the advertising identity key are admitted.

## Output Admittance Criteria

To be admitted into the ` + "`tm_messagebox`" + ` topic, an output must:

1. Contain a valid PushDrop script with 3 fields:
   - Identity Key (raw public key bytes)
   - Host (UTF-8 string)
   - Signature (binary)
2. Have a valid signature that matches the concatenated data fields:
   ` + "```" + `
   data = identityKey + host
   ` + "```" + `
3. The signature must verify against the identity key

If the signature is valid for the identity key, the output is added to the list of admissible outputs.
`

// MessageBoxTopicManager implements a topic manager for the MessageBox host advertisement protocol.
type MessageBoxTopicManager struct{}

// NewMessageBoxTopicManager creates a new MessageBoxTopicManager instance
func NewMessageBoxTopicManager() *MessageBoxTopicManager {
	return &MessageBoxTopicManager{}
}

// Ensure MessageBoxTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*MessageBoxTopicManager)(nil)

// IdentifyAdmissibleOutputs identifies which outputs in the supplied transaction are admissible.
func (tm *MessageBoxTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	log.Println("MessageBox topic manager invoked")

	// Parse transaction from BEEF
	tx, err := transaction.NewTransactionFromBEEF(beef)
	if err != nil {
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: outputsToAdmit,
			CoinsToRetain:  []uint32{},
		}, fmt.Errorf("failed to parse transaction from BEEF: %w", err)
	}

	if len(tx.Outputs) == 0 {
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: outputsToAdmit,
			CoinsToRetain:  []uint32{},
		}, fmt.Errorf("missing parameter: outputs")
	}

	log.Printf("[TOPIC MANAGER] Decoding transaction with %d outputs", len(tx.Outputs))

	// Inspect every output
	for index, output := range tx.Outputs {
		if err := tm.validateOutput(output, index); err == nil {
			log.Printf("[OUTPUT %d] PASSED validation", index)
			outputsToAdmit = append(outputsToAdmit, uint32(index))
		} else {
			log.Printf("[OUTPUT %d] FAILED validation: %v", index, err)
		}
	}

	if len(outputsToAdmit) > 0 {
		log.Printf("[TOPIC MANAGER] Outputs to admit: %v", outputsToAdmit)
	}

	// MessageBox protocol retains previous coins
	coinsToRetain := []uint32{}
	for key := range previousCoins {
		coinsToRetain = append(coinsToRetain, key)
	}

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// validateOutput validates a single output according to MessageBox protocol rules
func (tm *MessageBoxTopicManager) validateOutput(output *transaction.TransactionOutput, index int) error {
	// Decode PushDrop
	result := pushdrop.Decode(output.LockingScript)
	if result == nil {
		return fmt.Errorf("not a valid PushDrop output")
	}

	log.Printf("[OUTPUT %d] PushDrop decoded fields count: %d", index, len(result.Fields))

	// Must have at least 3 fields (identityKey + host + signature)
	if len(result.Fields) < 3 {
		return fmt.Errorf("invalid field count: %d (expected 3: identityKey, host, signature)", len(result.Fields))
	}

	// Extract fields
	identityKeyBuf := result.Fields[0]
	hostBuf := result.Fields[1]
	signature := result.Fields[len(result.Fields)-1]

	// Basic admissibility checks
	if len(identityKeyBuf) == 0 || len(hostBuf) == 0 {
		return fmt.Errorf("empty identityKey or host field")
	}

	// Decode host as UTF-8
	host := string(hostBuf)
	if host == "" {
		return fmt.Errorf("invalid host: empty after UTF-8 decoding")
	}

	log.Printf("[OUTPUT %d] Decoded host: %s", index, host)

	// Parse identity key as public key
	pubKey, err := ec.ParsePubKey(identityKeyBuf)
	if err != nil {
		return fmt.Errorf("invalid identity key: %w", err)
	}

	// Concatenate data fields for signature verification (identityKey + host)
	dataToVerify := append(identityKeyBuf, hostBuf...)

	log.Printf("[OUTPUT %d] Verifying signature over %d bytes of data", index, len(dataToVerify))

	// Parse and verify signature
	sig, err := ec.ParseSignature(signature)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	if !sig.Verify(dataToVerify, pubKey) {
		return fmt.Errorf("signature verification failed")
	}

	log.Printf("[OUTPUT %d] Signature PASSED verification", index)

	return nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for MessageBox protocol)
func (tm *MessageBoxTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// MessageBox protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation associated with this topic manager
func (tm *MessageBoxTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about the topic manager
func (tm *MessageBoxTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "MessageBox Topic Manager",
		Description: "Advertises and validates hosts for message routing.",
	}
}
