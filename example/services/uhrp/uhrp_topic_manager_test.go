package uhrp

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadVarInt(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected uint64
		wantErr  bool
	}{
		{
			name:     "single byte value 0",
			input:    []byte{0x00},
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "single byte value 1",
			input:    []byte{0x01},
			expected: 1,
			wantErr:  false,
		},
		{
			name:     "single byte max (127)",
			input:    []byte{0x7f},
			expected: 127,
			wantErr:  false,
		},
		{
			name:     "two byte value 128",
			input:    []byte{0x80, 0x01},
			expected: 128,
			wantErr:  false,
		},
		{
			name:     "two byte value 300",
			input:    []byte{0xac, 0x02},
			expected: 300,
			wantErr:  false,
		},
		{
			name:     "larger value 16384",
			input:    []byte{0x80, 0x80, 0x01},
			expected: 16384,
			wantErr:  false,
		},
		{
			name:    "empty data",
			input:   []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := readVarInt(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUHRPTopicManager_NewInstance(t *testing.T) {
	tm := NewUHRPTopicManager()
	require.NotNil(t, tm)
}

func TestUHRPTopicManager_GetDocumentation(t *testing.T) {
	tm := NewUHRPTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "Universal Hash Resolution Protocol")
	assert.Contains(t, docs, "PushDrop")
}

func TestUHRPTopicManager_GetMetaData(t *testing.T) {
	tm := NewUHRPTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Universal Hash Resolution Protocol", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestUHRPTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewUHRPTopicManager()

	// UHRP protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestUHRPTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewUHRPTopicManager()

	// Create a transaction with no outputs
	tx := transaction.NewTransaction()
	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUHRPTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewUHRPTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUHRPTopicManager_IdentifyAdmissibleOutputs_NonPushDropOutput(t *testing.T) {
	tm := NewUHRPTopicManager()

	// Create a transaction with a simple output (not PushDrop)
	tx := transaction.NewTransaction()
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUHRPTopicManager_IdentifyAdmissibleOutputs_ValidUHRPToken(t *testing.T) {
	tm := NewUHRPTopicManager()

	// Create a valid UHRP token
	tx, err := createValidUHRPTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestUHRPTopicManager_IdentifyAdmissibleOutputs_RetainsPreviousCoins(t *testing.T) {
	tm := NewUHRPTopicManager()

	tx, err := createValidUHRPTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.ElementsMatch(t, previousCoins, instructions.CoinsToRetain)
}

func TestUHRPTopicManager_ValidateOutput_InvalidHashLength(t *testing.T) {
	tm := NewUHRPTopicManager()

	// Create token with invalid hash length (not 32 bytes)
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		privateKey.PubKey().Compressed(), // identity key
		[]byte("short-hash"),             // invalid: should be 32 bytes
		[]byte("https://example.com/file.dat"),
		encodeVarInt(uint64(1735689600)), // expiry time
		encodeVarInt(uint64(1024)),       // file size
	}

	lockingScript, err := createSignedPushDropToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	err = tm.validateOutput(output, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hash length")
}

func TestUHRPTopicManager_ValidateOutput_InvalidURLScheme(t *testing.T) {
	tm := NewUHRPTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		privateKey.PubKey().Compressed(),
		make([]byte, 32),                      // valid 32-byte hash
		[]byte("http://example.com/file.dat"), // invalid: must be https
		encodeVarInt(uint64(1735689600)),
		encodeVarInt(uint64(1024)),
	}

	lockingScript, err := createSignedPushDropToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	err = tm.validateOutput(output, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestUHRPTopicManager_ValidateOutput_InvalidExpiryTime(t *testing.T) {
	tm := NewUHRPTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		privateKey.PubKey().Compressed(),
		make([]byte, 32),
		[]byte("https://example.com/file.dat"),
		encodeVarInt(uint64(0)), // invalid: expiry time must be >= 1
		encodeVarInt(uint64(1024)),
	}

	lockingScript, err := createSignedPushDropToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	err = tm.validateOutput(output, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expiry time")
}

func TestUHRPTopicManager_ValidateOutput_InvalidFileSize(t *testing.T) {
	tm := NewUHRPTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		privateKey.PubKey().Compressed(),
		make([]byte, 32),
		[]byte("https://example.com/file.dat"),
		encodeVarInt(uint64(1735689600)),
		encodeVarInt(uint64(0)), // invalid: file size must be >= 1
	}

	lockingScript, err := createSignedPushDropToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	err = tm.validateOutput(output, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file size")
}

func TestUHRPTopicManager_ValidateOutput_InvalidSignature(t *testing.T) {
	tm := NewUHRPTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	differentKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Use one key for the identity field but sign with a different key
	fields := [][]byte{
		privateKey.PubKey().Compressed(), // identity key
		make([]byte, 32),
		[]byte("https://example.com/file.dat"),
		encodeVarInt(uint64(1735689600)),
		encodeVarInt(uint64(1024)),
	}

	// Sign with a different key
	lockingScript, err := createSignedPushDropTokenWithDifferentSigner(differentKey, privateKey.PubKey(), fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	err = tm.validateOutput(output, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}

// Helper functions

func createValidUHRPTransaction(t *testing.T) (*transaction.Transaction, error) {
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		privateKey.PubKey().Compressed(),
		make([]byte, 32), // 32-byte hash
		[]byte("https://example.com/file.dat"),
		encodeVarInt(uint64(1735689600)), // expiry time (future timestamp)
		encodeVarInt(uint64(1024)),       // file size
	}

	lockingScript, err := createSignedPushDropToken(privateKey, fields)
	if err != nil {
		return nil, err
	}

	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	return tx, nil
}

func createSignedPushDropToken(privateKey *ec.PrivateKey, fields [][]byte) (*script.Script, error) {
	// Create wallet for signing
	protoWallet, err := wallet.NewProtoWallet(wallet.ProtoWalletArgs{
		Type:       wallet.ProtoWalletArgsTypePrivateKey,
		PrivateKey: privateKey,
	})
	if err != nil {
		return nil, err
	}

	// Concatenate fields for signing
	var data bytes.Buffer
	for _, field := range fields {
		data.Write(field)
	}

	// Sign using BRC-48 protocol
	signArgs := wallet.CreateSignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 2,
				Protocol:      "uhrp advertisement",
			},
			KeyID: "1",
			Counterparty: wallet.Counterparty{
				Type: wallet.CounterpartyTypeAnyone,
			},
		},
		Data: data.Bytes(),
	}

	signResult, err := protoWallet.CreateSignature(context.Background(), signArgs, "")
	if err != nil {
		return nil, err
	}

	// Add signature to fields
	allFields := append(fields, signResult.Signature.Serialize())

	// Build the PushDrop script manually
	pubKeyBytes := privateKey.PubKey().Compressed()
	lockChunks := []*script.ScriptChunk{
		{Op: byte(len(pubKeyBytes)), Data: pubKeyBytes},
		{Op: script.OpCHECKSIG},
	}

	pushDropChunks := make([]*script.ScriptChunk, 0)
	for _, field := range allFields {
		pushDropChunks = append(pushDropChunks, pushdrop.CreateMinimallyEncodedScriptChunk(field))
	}

	notYetDropped := len(allFields)
	for notYetDropped > 1 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.Op2DROP})
		notYetDropped -= 2
	}
	if notYetDropped != 0 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.OpDROP})
	}

	return script.NewScriptFromScriptOps(append(lockChunks, pushDropChunks...))
}

func createSignedPushDropTokenWithDifferentSigner(signerKey *ec.PrivateKey, lockingPubKey *ec.PublicKey, fields [][]byte) (*script.Script, error) {
	// Create wallet for signing with a different key
	protoWallet, err := wallet.NewProtoWallet(wallet.ProtoWalletArgs{
		Type:       wallet.ProtoWalletArgsTypePrivateKey,
		PrivateKey: signerKey,
	})
	if err != nil {
		return nil, err
	}

	// Concatenate fields for signing
	var data bytes.Buffer
	for _, field := range fields {
		data.Write(field)
	}

	// Sign using BRC-48 protocol
	signArgs := wallet.CreateSignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 2,
				Protocol:      "uhrp advertisement",
			},
			KeyID: "1",
			Counterparty: wallet.Counterparty{
				Type: wallet.CounterpartyTypeAnyone,
			},
		},
		Data: data.Bytes(),
	}

	signResult, err := protoWallet.CreateSignature(context.Background(), signArgs, "")
	if err != nil {
		return nil, err
	}

	// Add signature to fields
	allFields := append(fields, signResult.Signature.Serialize())

	// Build the script manually with mismatched locking key
	pubKeyBytes := lockingPubKey.Compressed()
	lockChunks := []*script.ScriptChunk{
		{Op: byte(len(pubKeyBytes)), Data: pubKeyBytes},
		{Op: script.OpCHECKSIG},
	}

	pushDropChunks := make([]*script.ScriptChunk, 0)
	for _, field := range allFields {
		pushDropChunks = append(pushDropChunks, pushdrop.CreateMinimallyEncodedScriptChunk(field))
	}

	notYetDropped := len(allFields)
	for notYetDropped > 1 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.Op2DROP})
		notYetDropped -= 2
	}
	if notYetDropped != 0 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.OpDROP})
	}

	return script.NewScriptFromScriptOps(append(lockChunks, pushDropChunks...))
}

func encodeVarInt(value uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, value)
	return buf[:n]
}
