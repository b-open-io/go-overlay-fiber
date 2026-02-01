package hello

import (
	"bytes"
	"context"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelloWorldTopicManager_NewInstance(t *testing.T) {
	tm := NewHelloWorldTopicManager()
	require.NotNil(t, tm)
}

func TestHelloWorldTopicManager_GetDocumentation(t *testing.T) {
	tm := NewHelloWorldTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "HelloWorld Topic Manager")
	assert.Contains(t, docs, "Push")
}

func TestHelloWorldTopicManager_GetMetaData(t *testing.T) {
	tm := NewHelloWorldTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "HelloWorld Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestHelloWorldTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewHelloWorldTopicManager()

	// HelloWorld protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestHelloWorldTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewHelloWorldTopicManager()

	// Create a transaction with no outputs
	tx := transaction.NewTransaction()
	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestHelloWorldTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewHelloWorldTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestHelloWorldTopicManager_IdentifyAdmissibleOutputs_NonPushDropOutput(t *testing.T) {
	tm := NewHelloWorldTopicManager()

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

func TestHelloWorldTopicManager_IdentifyAdmissibleOutputs_ValidToken(t *testing.T) {
	tm := NewHelloWorldTopicManager()

	// Create a valid HelloWorld token
	tx, err := createValidHelloWorldTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestHelloWorldTopicManager_ValidateOutput_MessageTooShort(t *testing.T) {
	tm := NewHelloWorldTopicManager()

	// Create token with message that's too short (< 2 characters)
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("H"), // Too short - only 1 character
	}

	lockingScript, err := createSignedHelloWorldToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	err = tm.validateOutput(output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message too short")
}

func TestHelloWorldTopicManager_ValidateOutput_InvalidSignature(t *testing.T) {
	tm := NewHelloWorldTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	differentKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Use one key for locking but sign with a different key
	fields := [][]byte{
		[]byte("Hello World"),
	}

	// Sign with a different key
	lockingScript, err := createSignedHelloWorldTokenWithDifferentSigner(differentKey, privateKey.PubKey(), fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	err = tm.validateOutput(output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}

// Helper functions

func createValidHelloWorldTransaction(t *testing.T) (*transaction.Transaction, error) {
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("Hello World"),
	}

	lockingScript, err := createSignedHelloWorldToken(privateKey, fields)
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

func createSignedHelloWorldToken(privateKey *ec.PrivateKey, fields [][]byte) (*script.Script, error) {
	// Concatenate fields for signing
	var data bytes.Buffer
	for _, field := range fields {
		data.Write(field)
	}

	// Sign with the private key using direct ECDSA
	hash := data.Bytes()
	signature, err := privateKey.Sign(hash)
	if err != nil {
		return nil, err
	}

	// Add signature to fields
	allFields := append(fields, signature.Serialize())

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

func createSignedHelloWorldTokenWithDifferentSigner(signerKey *ec.PrivateKey, lockingPubKey *ec.PublicKey, fields [][]byte) (*script.Script, error) {
	// Concatenate fields for signing
	var data bytes.Buffer
	for _, field := range fields {
		data.Write(field)
	}

	// Sign with the signer key (different from locking key)
	hash := data.Bytes()
	signature, err := signerKey.Sign(hash)
	if err != nil {
		return nil, err
	}

	// Add signature to fields
	allFields := append(fields, signature.Serialize())

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
