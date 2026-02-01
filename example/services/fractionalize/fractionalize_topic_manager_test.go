package fractionalize

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFractionalizeTopicManager_NewInstance(t *testing.T) {
	tm := NewFractionalizeTopicManager()
	require.NotNil(t, tm)
}

func TestFractionalizeTopicManager_GetDocumentation(t *testing.T) {
	tm := NewFractionalizeTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "Fractionalize Topic Manager")
	assert.Contains(t, docs, "Server Token")
	assert.Contains(t, docs, "Transfer Token")
	assert.Contains(t, docs, "Payment")
	assert.Contains(t, docs, "OP_IF")
	assert.Contains(t, docs, "OP_CHECKMULTISIG")
}

func TestFractionalizeTopicManager_GetMetaData(t *testing.T) {
	tm := NewFractionalizeTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Fractionalize Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
	assert.Equal(t, "0.1.0", meta.Version)
}

func TestFractionalizeTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Fractionalize protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create a transaction with an input but no outputs
	tx, err := createFractionalizeTransactionWithInput(t, nil)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_NoMatchingOutputs(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create a transaction with a simple output (not a valid fractionalize output)
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a fractionalize output"))
	output := &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	}

	tx, err := createFractionalizeTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
	assert.Empty(t, instructions.CoinsToRetain)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_ServerToken(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create a valid server token (has both OP_IF and OP_CHECKMULTISIG)
	serverTokenScript, err := createServerTokenScript()
	require.NoError(t, err)
	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: serverTokenScript,
	}

	tx, err := createFractionalizeTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
	assert.Empty(t, instructions.CoinsToRetain) // Fractionalize never retains coins
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_TransferToken(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create a valid transfer token (has OP_IF but not OP_CHECKMULTISIG)
	transferTokenScript, err := createTransferTokenScript()
	require.NoError(t, err)
	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: transferTokenScript,
	}

	tx, err := createFractionalizeTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
	assert.Empty(t, instructions.CoinsToRetain)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_Payment(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create a valid payment output (has OP_CHECKMULTISIG but not OP_IF)
	paymentScript, err := createPaymentScript()
	require.NoError(t, err)
	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: paymentScript,
	}

	tx, err := createFractionalizeTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
	assert.Empty(t, instructions.CoinsToRetain)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_MultipleOutputTypes(t *testing.T) {
	tm := NewFractionalizeTopicManager()

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

	// Add server token
	serverTokenScript, err := createServerTokenScript()
	require.NoError(t, err)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: serverTokenScript,
	})

	// Add transfer token
	transferTokenScript, err := createTransferTokenScript()
	require.NoError(t, err)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: transferTokenScript,
	})

	// Add payment
	paymentScript, err := createPaymentScript()
	require.NoError(t, err)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: paymentScript,
	})

	// Add invalid output
	invalidScript := &script.Script{}
	_ = invalidScript.AppendOpcodes(script.OpRETURN)
	_ = invalidScript.AppendPushData([]byte("invalid"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: invalidScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 3)
	assert.Contains(t, instructions.OutputsToAdmit, uint32(0)) // server token
	assert.Contains(t, instructions.OutputsToAdmit, uint32(1)) // transfer token
	assert.Contains(t, instructions.OutputsToAdmit, uint32(2)) // payment
	assert.Empty(t, instructions.CoinsToRetain)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_InvalidServerToken(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create an invalid server token (has both OP_IF and OP_CHECKMULTISIG but wrong template)
	invalidScript := &script.Script{}
	_ = invalidScript.AppendOpcodes(script.OpIF)
	_ = invalidScript.AppendPushData([]byte("wrong template"))
	_ = invalidScript.AppendOpcodes(script.OpCHECKMULTISIG)
	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: invalidScript,
	}

	tx, err := createFractionalizeTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	// With template validation enabled, this should NOT be admitted
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_InvalidTransferToken(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create an invalid transfer token (has OP_IF but wrong template)
	invalidScript := &script.Script{}
	_ = invalidScript.AppendOpcodes(script.OpIF)
	_ = invalidScript.AppendPushData([]byte("wrong template"))
	_ = invalidScript.AppendOpcodes(script.OpENDIF)
	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: invalidScript,
	}

	tx, err := createFractionalizeTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	// With template validation enabled, this should NOT be admitted
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_InvalidPayment(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create an invalid payment (has OP_CHECKMULTISIG but wrong template)
	invalidScript := &script.Script{}
	_ = invalidScript.AppendPushData([]byte("wrong template"))
	_ = invalidScript.AppendOpcodes(script.OpCHECKMULTISIG)
	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: invalidScript,
	}

	tx, err := createFractionalizeTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	// With template validation enabled, this should NOT be admitted
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestFractionalizeTopicManager_IdentifyAdmissibleOutputs_NilLockingScript(t *testing.T) {
	tm := NewFractionalizeTopicManager()

	// Create a transaction with empty locking script (nil would crash BEEF generation)
	emptyScript := &script.Script{}
	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: emptyScript,
	}

	tx, err := createFractionalizeTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

// Helper functions

// createFractionalizeTransactionWithInput creates a transaction with a valid funding input
// and optionally adds an output
func createFractionalizeTransactionWithInput(t *testing.T, output *transaction.TransactionOutput) (*transaction.Transaction, error) {
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

// createServerTokenScript creates a valid server token script that matches the template
// Template expects: OP_0 OP_IF <3 bytes> OP_1 <18 bytes> OP_0 <36 bytes> OP_ENDIF
// OP_2DUP OP_CAT OP_HASH160 <20 bytes> OP_EQUALVERIFY OP_TOALTSTACK OP_TOALTSTACK
// OP_1 OP_FROMALTSTACK OP_FROMALTSTACK OP_2 OP_CHECKMULTISIG OP_RETURN <32 bytes>
func createServerTokenScript() (*script.Script, error) {
	s := &script.Script{}
	// Ordinal inscription
	_ = s.AppendOpcodes(script.OpFALSE)
	_ = s.AppendOpcodes(script.OpIF)
	_ = s.AppendPushData([]byte("ord")) // 3 bytes
	_ = s.AppendOpcodes(script.Op1)
	_ = s.AppendPushData([]byte("application/bsv-20")) // 18 bytes
	_ = s.AppendOpcodes(script.OpFALSE)
	_ = s.AppendPushData([]byte(`{"p":"bsv-20","op":"mint","amt":"1"}`)) // 36 bytes
	_ = s.AppendOpcodes(script.OpENDIF)
	// 1-of-2 multisig locking script
	_ = s.AppendOpcodes(script.Op2DUP)
	_ = s.AppendOpcodes(script.OpCAT)
	_ = s.AppendOpcodes(script.OpHASH160)
	_ = s.AppendPushData(make([]byte, 20)) // 20 bytes hash160
	_ = s.AppendOpcodes(script.OpEQUALVERIFY)
	_ = s.AppendOpcodes(script.OpTOALTSTACK)
	_ = s.AppendOpcodes(script.OpTOALTSTACK)
	_ = s.AppendOpcodes(script.Op1)
	_ = s.AppendOpcodes(script.OpFROMALTSTACK)
	_ = s.AppendOpcodes(script.OpFROMALTSTACK)
	_ = s.AppendOpcodes(script.Op2)
	_ = s.AppendOpcodes(script.OpCHECKMULTISIG)
	// OP_RETURN with txid
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData(make([]byte, 32)) // 32 bytes txid
	return s, nil
}

// createTransferTokenScript creates a valid transfer token script that matches the template
// Template expects: OP_0 OP_IF <3 bytes> OP_1 <18 bytes> OP_0 <36 bytes> OP_ENDIF
// OP_DUP OP_HASH160 <20 bytes> OP_EQUALVERIFY OP_CHECKSIG OP_RETURN <32 bytes>
func createTransferTokenScript() (*script.Script, error) {
	s := &script.Script{}
	// Ordinal inscription
	_ = s.AppendOpcodes(script.OpFALSE)
	_ = s.AppendOpcodes(script.OpIF)
	_ = s.AppendPushData([]byte("ord")) // 3 bytes
	_ = s.AppendOpcodes(script.Op1)
	_ = s.AppendPushData([]byte("application/bsv-20")) // 18 bytes
	_ = s.AppendOpcodes(script.OpFALSE)
	_ = s.AppendPushData([]byte(`{"p":"bsv-20","op":"mint","amt":"1"}`)) // 36 bytes
	_ = s.AppendOpcodes(script.OpENDIF)
	// P2PKH locking script
	_ = s.AppendOpcodes(script.OpDUP)
	_ = s.AppendOpcodes(script.OpHASH160)
	_ = s.AppendPushData(make([]byte, 20)) // 20 bytes pubkeyhash
	_ = s.AppendOpcodes(script.OpEQUALVERIFY)
	_ = s.AppendOpcodes(script.OpCHECKSIG)
	// OP_RETURN with txid
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData(make([]byte, 32)) // 32 bytes txid
	return s, nil
}

// createPaymentScript creates a valid payment script that matches the template
// Template expects: OP_2DUP OP_CAT OP_HASH160 <20 bytes> OP_EQUALVERIFY
// OP_TOALTSTACK OP_TOALTSTACK OP_1 OP_FROMALTSTACK OP_FROMALTSTACK OP_2 OP_CHECKMULTISIG
func createPaymentScript() (*script.Script, error) {
	s := &script.Script{}
	// 1-of-2 multisig locking script (no ordinal/OP_IF)
	_ = s.AppendOpcodes(script.Op2DUP)
	_ = s.AppendOpcodes(script.OpCAT)
	_ = s.AppendOpcodes(script.OpHASH160)
	_ = s.AppendPushData(make([]byte, 20)) // 20 bytes hash160
	_ = s.AppendOpcodes(script.OpEQUALVERIFY)
	_ = s.AppendOpcodes(script.OpTOALTSTACK)
	_ = s.AppendOpcodes(script.OpTOALTSTACK)
	_ = s.AppendOpcodes(script.Op1)
	_ = s.AppendOpcodes(script.OpFROMALTSTACK)
	_ = s.AppendOpcodes(script.OpFROMALTSTACK)
	_ = s.AppendOpcodes(script.Op2)
	_ = s.AppendOpcodes(script.OpCHECKMULTISIG)
	return s, nil
}
