package ump

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUMPTopicManager_NewInstance(t *testing.T) {
	tm := NewUMPTopicManager()
	require.NotNil(t, tm)
}

func TestUMPTopicManager_GetDocumentation(t *testing.T) {
	tm := NewUMPTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "User Management Protocol")
	assert.Contains(t, docs, "PushDrop")
	assert.Contains(t, docs, "presentationHash")
	assert.Contains(t, docs, "recoveryHash")
}

func TestUMPTopicManager_GetMetaData(t *testing.T) {
	tm := NewUMPTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "User Management Protocol", meta.Name)
	assert.Contains(t, meta.Description, "wallet account descriptors")
}

func TestUMPTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewUMPTopicManager()

	// UMP protocol doesn't need any inputs for validation
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewUMPTopicManager()

	// Create a transaction with inputs but no outputs
	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
		SourceTxOutIndex: 0,
	})
	addSourceTransaction(tx)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewUMPTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_TooFewFields(t *testing.T) {
	tm := NewUMPTopicManager()

	// Create a transaction with a PushDrop output that has too few fields (need at least 11)
	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
		SourceTxOutIndex: 0,
	})
	addSourceTransaction(tx)

	// Create a PushDrop token with only 5 fields (invalid, needs at least 11)
	fields := [][]byte{
		[]byte("field0"),
		[]byte("field1"),
		[]byte("field2"),
		[]byte("field3"),
		[]byte("field4"),
	}

	lockingScript := createPushDropScript(t, fields)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	// Should not admit outputs with too few fields
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_InvalidPresentationHashLength(t *testing.T) {
	tm := NewUMPTopicManager()

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
		SourceTxOutIndex: 0,
	})
	addSourceTransaction(tx)

	// Create UMP token with invalid presentationHash length (field 6)
	fields := [][]byte{
		[]byte("passwordSalt"),
		[]byte("passwordPresentationPrimary"),
		[]byte("passwordRecoveryPrimary"),
		[]byte("presentationRecoveryPrimary"),
		[]byte("passwordPrimaryPrivileged"),
		[]byte("presentationRecoveryPrivileged"),
		[]byte("short-hash"), // Field 6: presentationHash - invalid length (not 32 bytes)
		make([]byte, 32),     // Field 7: recoveryHash - valid 32 bytes
		[]byte("presentationKeyEncrypted"),
		[]byte("passwordKeyEncrypted"),
		[]byte("recoveryKeyEncrypted"),
	}

	lockingScript := createPushDropScript(t, fields)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	// Should not admit outputs with invalid presentationHash length
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_InvalidRecoveryHashLength(t *testing.T) {
	tm := NewUMPTopicManager()

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
		SourceTxOutIndex: 0,
	})
	addSourceTransaction(tx)

	// Create UMP token with invalid recoveryHash length (field 7)
	fields := [][]byte{
		[]byte("passwordSalt"),
		[]byte("passwordPresentationPrimary"),
		[]byte("passwordRecoveryPrimary"),
		[]byte("presentationRecoveryPrimary"),
		[]byte("passwordPrimaryPrivileged"),
		[]byte("presentationRecoveryPrivileged"),
		make([]byte, 32),     // Field 6: presentationHash - valid 32 bytes
		[]byte("short-hash"), // Field 7: recoveryHash - invalid length (not 32 bytes)
		[]byte("presentationKeyEncrypted"),
		[]byte("passwordKeyEncrypted"),
		[]byte("recoveryKeyEncrypted"),
	}

	lockingScript := createPushDropScript(t, fields)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	// Should not admit outputs with invalid recoveryHash length
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_ValidToken(t *testing.T) {
	tm := NewUMPTopicManager()

	tx, err := createValidUMPTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_RetainsPreviousCoins(t *testing.T) {
	tm := NewUMPTopicManager()

	tx, err := createValidUMPTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.ElementsMatch(t, previousCoins, instructions.CoinsToRetain)
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_NoInputs(t *testing.T) {
	tm := NewUMPTopicManager()

	// Create a transaction with outputs but no inputs (invalid for UMP)
	tx := transaction.NewTransaction()

	// Create valid UMP fields
	fields := [][]byte{
		[]byte("passwordSalt"),
		[]byte("passwordPresentationPrimary"),
		[]byte("passwordRecoveryPrimary"),
		[]byte("presentationRecoveryPrimary"),
		[]byte("passwordPrimaryPrivileged"),
		[]byte("presentationRecoveryPrivileged"),
		make([]byte, 32), // Field 6: presentationHash - valid 32 bytes
		make([]byte, 32), // Field 7: recoveryHash - valid 32 bytes
		[]byte("presentationKeyEncrypted"),
		[]byte("passwordKeyEncrypted"),
		[]byte("recoveryKeyEncrypted"),
	}

	lockingScript := createPushDropScript(t, fields)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestUMPTopicManager_IdentifyAdmissibleOutputs_MultipleOutputs(t *testing.T) {
	tm := NewUMPTopicManager()

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
		SourceTxOutIndex: 0,
	})
	addSourceTransaction(tx)

	// Add valid UMP token
	validFields := [][]byte{
		[]byte("passwordSalt"),
		[]byte("passwordPresentationPrimary"),
		[]byte("passwordRecoveryPrimary"),
		[]byte("presentationRecoveryPrimary"),
		[]byte("passwordPrimaryPrivileged"),
		[]byte("presentationRecoveryPrivileged"),
		make([]byte, 32), // Field 6: presentationHash - valid 32 bytes
		make([]byte, 32), // Field 7: recoveryHash - valid 32 bytes
		[]byte("presentationKeyEncrypted"),
		[]byte("passwordKeyEncrypted"),
		[]byte("recoveryKeyEncrypted"),
	}

	validLockingScript := createPushDropScript(t, validFields)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: validLockingScript,
	})

	// Add invalid UMP token (too few fields)
	invalidFields := [][]byte{
		[]byte("field0"),
		[]byte("field1"),
	}

	invalidLockingScript := createPushDropScript(t, invalidFields)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: invalidLockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	// Should only admit the valid output (index 0)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

// Helper functions

func createValidUMPTransaction(t *testing.T) (*transaction.Transaction, error) {
	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
		SourceTxOutIndex: 0,
	})
	addSourceTransaction(tx)

	// Create valid UMP fields (at least 11 fields with valid hash lengths)
	fields := [][]byte{
		[]byte("passwordSalt"),
		[]byte("passwordPresentationPrimary"),
		[]byte("passwordRecoveryPrimary"),
		[]byte("presentationRecoveryPrimary"),
		[]byte("passwordPrimaryPrivileged"),
		[]byte("presentationRecoveryPrivileged"),
		make([]byte, 32), // Field 6: presentationHash - valid 32 bytes
		make([]byte, 32), // Field 7: recoveryHash - valid 32 bytes
		[]byte("presentationKeyEncrypted"),
		[]byte("passwordKeyEncrypted"),
		[]byte("recoveryKeyEncrypted"),
	}

	lockingScript := createPushDropScript(t, fields)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	return tx, nil
}

func createPushDropScript(t *testing.T, fields [][]byte) *script.Script {
	// Create a simple pubkey lock (similar to DID/UHRP but without signature validation)
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	pubKeyBytes := privateKey.PubKey().Compressed()
	lockChunks := []*script.ScriptChunk{
		{Op: byte(len(pubKeyBytes)), Data: pubKeyBytes},
		{Op: script.OpCHECKSIG},
	}

	// Create the PushDrop chunks
	pushDropChunks := make([]*script.ScriptChunk, 0)
	for _, field := range fields {
		pushDropChunks = append(pushDropChunks, pushdrop.CreateMinimallyEncodedScriptChunk(field))
	}

	// Add DROP operations
	notYetDropped := len(fields)
	for notYetDropped > 1 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.Op2DROP})
		notYetDropped -= 2
	}
	if notYetDropped != 0 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.OpDROP})
	}

	s, err := script.NewScriptFromScriptOps(append(lockChunks, pushDropChunks...))
	require.NoError(t, err)
	return s
}

func addSourceTransaction(tx *transaction.Transaction) {
	sourceTx := transaction.NewTransaction()
	sourceTx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: &script.Script{},
	})

	// Update all inputs to reference the source transaction
	for i := range tx.Inputs {
		tx.Inputs[i].SourceTXID = sourceTx.TxID()
		tx.Inputs[i].SourceTransaction = sourceTx
	}
}
