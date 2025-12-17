package did

import (
	"bytes"
	"context"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDIDTopicManager_NewInstance(t *testing.T) {
	tm := NewDIDTopicManager()
	require.NotNil(t, tm)
}

func TestDIDTopicManager_GetDocumentation(t *testing.T) {
	tm := NewDIDTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "DID Topic Manager")
	assert.Contains(t, docs, "Admissibility Rules")
}

func TestDIDTopicManager_GetMetaData(t *testing.T) {
	tm := NewDIDTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "DID Topic Manager", meta.Name)
	assert.Equal(t, "DID Resolution Protocol", meta.Description)
}

func TestDIDTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewDIDTopicManager()

	// DID protocol doesn't need any inputs for validation
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewDIDTopicManager()

	// Create a transaction with an input but no outputs
	tx, err := createDIDTransactionWithInput(t, nil)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewDIDTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_WrongFieldCount(t *testing.T) {
	tm := NewDIDTopicManager()

	// Create a transaction with PushDrop output but wrong number of fields
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Create a PushDrop token with 2 input fields + signature = 3 fields (should be 2)
	// Note: createSignedDIDToken adds the signature field, so we need 2 input fields to get 3 total
	fields := [][]byte{
		[]byte("serial123"),
		[]byte("extra_field"), // This will cause 3 fields total (serial + extra + signature)
	}

	lockingScript, err := createSignedDIDToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createDIDTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Should fail because of wrong field count (3 instead of 2)
	// The topic manager returns an error when no outputs are admitted and no previous coins
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no outputs admitted")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_ValidTokenWithSerialNumber(t *testing.T) {
	tm := NewDIDTopicManager()

	// Create a valid DID token with serial number
	// Note: 1 input field + signature = 2 fields, which is exactly what's expected
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("serial123"), // valid serial number
	}

	lockingScript, err := createSignedDIDToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createDIDTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Should succeed - valid 2-field token with non-empty serial number
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_ValidToken(t *testing.T) {
	tm := NewDIDTopicManager()

	// Create a valid DID token
	tx, err := createValidDIDTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_NoOutputsAdmittedAndNoPreviousCoins(t *testing.T) {
	tm := NewDIDTopicManager()

	// Create a transaction with a non-PushDrop output
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
	output := &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	}

	tx, err := createDIDTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Should error because no outputs admitted AND no previous coins
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no outputs admitted")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_NoOutputsButHasPreviousCoins(t *testing.T) {
	tm := NewDIDTopicManager()

	// Create a transaction with a non-PushDrop output
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
	output := &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	}

	tx, err := createDIDTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Should succeed because we have previous coins even though no outputs admitted
	previousCoins := []uint32{0, 1}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
	assert.Empty(t, instructions.CoinsToRetain)
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_MissingInputs(t *testing.T) {
	tm := NewDIDTopicManager()

	// Create a transaction without inputs
	tx := transaction.NewTransaction()
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("data"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestDIDTopicManager_IdentifyAdmissibleOutputs_MultipleValidOutputs(t *testing.T) {
	tm := NewDIDTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Create the first DID token for the first output
	fields := [][]byte{
		[]byte("serial1"),
	}

	lockingScript, err := createSignedDIDToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createDIDTransactionWithInput(t, output)
	require.NoError(t, err)

	// Add remaining valid DID tokens
	for i := 1; i < 3; i++ {
		fields := [][]byte{
			[]byte("serial" + string(rune('1'+i))),
		}

		lockingScript, err := createSignedDIDToken(privateKey, fields)
		require.NoError(t, err)

		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      1,
			LockingScript: lockingScript,
		})
	}

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 3)
	assert.Equal(t, []uint32{0, 1, 2}, instructions.OutputsToAdmit)
}

// Helper functions

// createDIDTransactionWithInput creates a transaction with proper BEEF support
func createDIDTransactionWithInput(t *testing.T, output *transaction.TransactionOutput) (*transaction.Transaction, error) {
	// Create a funding transaction
	fundingTx := transaction.NewTransaction()
	fundingScript := &script.Script{}
	_ = fundingScript.AppendOpcodes(script.OpTRUE)
	fundingTx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: fundingScript,
	})

	// Create the main transaction that spends from the funding tx
	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:        fundingTx.TxID(),
		SourceTxOutIndex:  0,
		SourceTransaction: fundingTx,
		UnlockingScript:   &script.Script{},
		SequenceNumber:    0xFFFFFFFF,
	})

	// Add the output if provided
	if output != nil {
		tx.AddOutput(output)
	}

	return tx, nil
}

func createValidDIDTransaction(t *testing.T) (*transaction.Transaction, error) {
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("abc123serialnumber"),
	}

	lockingScript, err := createSignedDIDToken(privateKey, fields)
	if err != nil {
		return nil, err
	}

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	return createDIDTransactionWithInput(t, output)
}

func createSignedDIDToken(privateKey *ec.PrivateKey, fields [][]byte) (*script.Script, error) {
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

	// Sign using BRC-48 protocol for DID
	signArgs := wallet.CreateSignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 2,
				Protocol:      "did token",
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
