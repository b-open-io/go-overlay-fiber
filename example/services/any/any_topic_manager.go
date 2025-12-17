package any

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

const topicDocs = `
# Any Topic Manager Documentation

Literally any transaction is admitted by the Any Topic Manager.
`

// AnyTopicManager implements a topic manager for the simple "Any" protocol.
//
// Each valid output must satisfy the following rules:
// 1. There are no rules.
type AnyTopicManager struct{}

// NewAnyTopicManager creates a new AnyTopicManager instance
func NewAnyTopicManager() *AnyTopicManager {
	return &AnyTopicManager{}
}

// Ensure AnyTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*AnyTopicManager)(nil)

// IdentifyAdmissibleOutputs identifies which outputs in the supplied transaction are admissible.
//
// For the Any protocol, all outputs are admitted with no validation.
func (tm *AnyTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins []uint32,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	slog.Info("Any topic manager invoked")

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

	// Admit all outputs
	for i := range tx.Outputs {
		outputsToAdmit = append(outputsToAdmit, uint32(i))
	}

	// The Any protocol never retains previous coins
	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  []uint32{},
	}, nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for Any protocol)
func (tm *AnyTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// Any protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation associated with this topic manager
func (tm *AnyTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about the topic manager
func (tm *AnyTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "Any Topic Manager",
		Description: "Any transaction is admitted by the Any Topic Manager.",
	}
}
