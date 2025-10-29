package uhrp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net/url"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

const topicDocs = `
# Universal Hash Resolution Protocol Topic Manager Docs

To have outputs accepted into the UHRP overlay network, use the PushDrop template to create valid advertisements.

Submit transactions that advertise new files, or revoke (spend) existing advertisements already submitted.

The latest state of all advertisements will be tracked, and will be available through the corresponding UHRP Lookup Service.
`

// UHRPTopicManager implements a topic manager for the Universal Hash Resolution Protocol
type UHRPTopicManager struct{}

// NewUHRPTopicManager creates a new UHRPTopicManager instance
func NewUHRPTopicManager() *UHRPTopicManager {
	return &UHRPTopicManager{}
}

// Ensure UHRPTopicManager implements engine.TopicManager
var _ engine.TopicManager = (*UHRPTopicManager)(nil)

// IdentifyAdmissibleOutputs identifies which outputs in the supplied transaction are admissible.
func (tm *UHRPTopicManager) IdentifyAdmissibleOutputs(
	ctx context.Context,
	beef []byte,
	previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error) {
	outputsToAdmit := []uint32{}

	log.Printf("UHRP topic manager invoked with %d previous UTXOs", len(previousCoins))

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
		if err := tm.validateOutput(output, index); err == nil {
			log.Printf("[OUTPUT %d] PASSED validation", index)
			outputsToAdmit = append(outputsToAdmit, uint32(index))
		} else {
			log.Printf("[OUTPUT %d] FAILED validation: %v", index, err)
		}
	}

	// UHRP protocol retains previous coins
	coinsToRetain := []uint32{}
	for key := range previousCoins {
		coinsToRetain = append(coinsToRetain, key)
	}

	if len(outputsToAdmit) == 0 {
		log.Printf("No valid UHRP advertisements found, allowing transaction to pass without admitting outputs")
		return overlay.AdmittanceInstructions{
			OutputsToAdmit: []uint32{},
			CoinsToRetain:  coinsToRetain,
		}, nil
	}

	log.Printf("%d output(s) admitted as valid UHRP advertisement(s)", len(outputsToAdmit))

	return overlay.AdmittanceInstructions{
		OutputsToAdmit: outputsToAdmit,
		CoinsToRetain:  coinsToRetain,
	}, nil
}

// validateOutput validates a single output according to UHRP protocol rules
func (tm *UHRPTopicManager) validateOutput(output *transaction.TransactionOutput, index int) error {
	// Decode PushDrop
	result := pushdrop.Decode(output.LockingScript)
	if result == nil {
		return fmt.Errorf("not a valid PushDrop output")
	}

	// UHRP tokens have 6 fields (5 data + signature)
	if len(result.Fields) < 6 {
		return fmt.Errorf("invalid UHRP token: expected 6 fields, got %d", len(result.Fields))
	}

	// Extract fields
	hostIdentityKeyBuf := result.Fields[0]
	hashBuf := result.Fields[1]
	hostedFileLocationBuf := result.Fields[2]
	expiryTimeBuf := result.Fields[3]
	fileSizeBuf := result.Fields[4]
	signatureBuf := result.Fields[len(result.Fields)-1]

	// Validate hash length (must be 32 bytes)
	if len(hashBuf) != 32 {
		return fmt.Errorf("invalid hash length: expected 32 bytes, got %d", len(hashBuf))
	}

	// Validate file location (must be valid UTF-8 and HTTPS URL)
	fileLocationString := string(hostedFileLocationBuf)
	fileLocationURL, err := url.Parse(fileLocationString)
	if err != nil {
		return fmt.Errorf("invalid file location URL: %w", err)
	}
	if fileLocationURL.Scheme != "https" {
		return fmt.Errorf("advertisement must be on HTTPS, got %s", fileLocationURL.Scheme)
	}

	// Parse expiry time (varint)
	expiryTime, err := readVarInt(expiryTimeBuf)
	if err != nil {
		return fmt.Errorf("invalid expiry time: %w", err)
	}
	if expiryTime < 1 {
		return fmt.Errorf("invalid expiry time: must be >= 1")
	}

	// Parse file size (varint)
	fileSize, err := readVarInt(fileSizeBuf)
	if err != nil {
		return fmt.Errorf("invalid file size: %w", err)
	}
	if fileSize < 1 {
		return fmt.Errorf("invalid file size: must be >= 1")
	}

	log.Printf("[OUTPUT %d] Decoded UHRP advertisement: location=%s, expiryTime=%d, fileSize=%d",
		index, fileLocationString, expiryTime, fileSize)

	// Verify signature
	if err := tm.verifySignature(hostIdentityKeyBuf, result.Fields[:5], signatureBuf); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	log.Printf("[OUTPUT %d] Signature PASSED verification", index)

	return nil
}

// verifySignature verifies the UHRP advertisement signature
func (tm *UHRPTopicManager) verifySignature(identityKeyBytes []byte, dataFields [][]byte, signatureBytes []byte) error {
	// Parse identity key as public key
	pubKey, err := ec.ParsePubKey(identityKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid identity key: %w", err)
	}

	// Concatenate data fields for signature verification (fields 0-4, not including signature)
	var data bytes.Buffer
	for _, field := range dataFields {
		data.Write(field)
	}

	// Parse signature
	sig, err := ec.ParseSignature(signatureBytes)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	// Verify signature
	if !sig.Verify(data.Bytes(), pubKey) {
		return fmt.Errorf("signature does not match identity key")
	}

	return nil
}

// readVarInt reads a variable-length integer from the given byte slice
func readVarInt(data []byte) (uint64, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty data")
	}

	reader := bytes.NewReader(data)
	value, err := binary.ReadUvarint(reader)
	if err != nil {
		return 0, fmt.Errorf("failed to read varint: %w", err)
	}

	return value, nil
}

// IdentifyNeededInputs identifies inputs needed for validation (not used for UHRP protocol)
func (tm *UHRPTopicManager) IdentifyNeededInputs(ctx context.Context, beef []byte) ([]*transaction.Outpoint, error) {
	// UHRP protocol doesn't need any inputs for validation
	return nil, nil
}

// GetDocumentation returns the documentation associated with this topic manager
func (tm *UHRPTopicManager) GetDocumentation() string {
	return topicDocs
}

// GetMetaData returns metadata about the topic manager
func (tm *UHRPTopicManager) GetMetaData() *overlay.MetaData {
	return &overlay.MetaData{
		Name:        "Universal Hash Resolution Protocol",
		Description: "Manages UHRP content availability advertisements.",
	}
}
