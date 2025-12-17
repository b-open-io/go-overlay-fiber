package hello

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

const topicDocs = `
# HelloWorld Topic Manager Documentation

## Overview
The **HelloWorld Topic Manager** is a lightweight overlay protocol that lets users broadcast a short, UTF‑8 encoded message to the world using BRC‑48 Pay‑to‑Push‑Drop outputs. Each eligible transaction output becomes a permanent, verifiable "shout‑out" on-chain.

| Requirement | Description |
|-------------|-------------|
| **Protocol ID** | "HelloWorld" |
| **Fields** | *Exactly one (the message)* |
| **Message length** | ≥ 2 UTF‑8 characters |
| **Signature** | ECDSA over the concatenated field data, verified against the locking public key |
`

// HelloWorldTopicManager implements a topic manager for the HelloWorld messaging protocol.
type HelloWorldTopicManager struct{}

// NewHelloWorldTopicManager creates a new HelloWorldTopicManager instance
func NewHelloWorldTopicManager() *HelloWorldTopicManager {
	return &HelloWorldTopicManager{}
}

// Ensure HelloWorldTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*HelloWorldTopicManager)(nil)

// IdentifyAdmissibleOutputs identifies which outputs in the supplied transaction are admissible.
func (tm *HelloWorldTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins []uint32,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	slog.Info("HelloWorld topic manager invoked")

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

	// Inspect every output
	for index, output := range tx.Outputs {
		if err := tm.validateOutput(output); err == nil {
			outputsToAdmit = append(outputsToAdmit, uint32(index))
		}
	}

	if len(outputsToAdmit) > 0 {
		slog.Info("HelloWorld outputs admitted", "count", len(outputsToAdmit))
	}

	// The HelloWorld protocol never retains previous coins
	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  []uint32{},
	}, nil
}

// validateOutput validates a single output according to HelloWorld protocol rules
func (tm *HelloWorldTopicManager) validateOutput(output *transaction.TransactionOutput) error {
	// Decode PushDrop
	result := pushdrop.Decode(output.LockingScript)
	if result == nil {
		return fmt.Errorf("not a valid PushDrop output")
	}

	// Must have at least 2 fields (message + signature)
	if len(result.Fields) < 2 {
		return fmt.Errorf("invalid field count: %d", len(result.Fields))
	}

	// Extract signature (last field)
	signature := result.Fields[len(result.Fields)-1]
	messageFields := result.Fields[:len(result.Fields)-1]

	// Must contain exactly one field (not including the signature)
	if len(messageFields) != 1 {
		return fmt.Errorf("expected exactly 1 message field, got %d", len(messageFields))
	}

	// Convert message to UTF-8 and validate length
	message := string(messageFields[0])
	if len(message) < 2 {
		return fmt.Errorf("message too short: %d characters", len(message))
	}

	// Verify the signature against the locking public key
	if result.LockingPublicKey == nil {
		return fmt.Errorf("no locking public key found")
	}

	// Concatenate all message fields for signature verification
	dataToVerify := make([]byte, 0)
	for _, field := range messageFields {
		dataToVerify = append(dataToVerify, field...)
	}

	// Parse and verify signature
	sig, err := ec.ParseSignature(signature)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	if !sig.Verify(dataToVerify, result.LockingPublicKey) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for HelloWorld protocol)
func (tm *HelloWorldTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// HelloWorld protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation associated with this topic manager
func (tm *HelloWorldTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about the topic manager
func (tm *HelloWorldTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "HelloWorld Topic Manager",
		Description: "What's your message to the world?",
	}
}
