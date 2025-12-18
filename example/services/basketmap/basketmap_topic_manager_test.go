package basketmap

import (
	"context"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasketMapTopicManager_NewInstance(t *testing.T) {
	tm := NewBasketMapTopicManager()
	require.NotNil(t, tm)
}

func TestBasketMapTopicManager_GetDocumentation(t *testing.T) {
	tm := NewBasketMapTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "BasketMap Topic Manager")
	assert.Contains(t, docs, "PushDrop")
	assert.Contains(t, docs, "7 PushDrop fields")
}

func TestBasketMapTopicManager_GetMetaData(t *testing.T) {
	tm := NewBasketMapTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "BasketMap", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestBasketMapTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewBasketMapTopicManager()

	// BasketMap protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestBasketMapTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewBasketMapTopicManager()

	// Create a transaction with an input but no outputs
	tx, err := createBasketMapTransactionWithInput(t, nil)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestBasketMapTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewBasketMapTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestBasketMapTopicManager_IdentifyAdmissibleOutputs_NonPushDropOutput(t *testing.T) {
	tm := NewBasketMapTopicManager()

	// Create a transaction with a simple output (not PushDrop)
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
	output := &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	}

	tx, err := createBasketMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestBasketMapTopicManager_IdentifyAdmissibleOutputs_WrongFieldCount(t *testing.T) {
	tm := NewBasketMapTopicManager()

	// Create PushDrop with only 5 fields (should have 7)
	fields := [][]byte{
		[]byte("basket-1"),
		[]byte("Test Basket"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
	}

	lockingScript, err := createPushDropScript(fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createBasketMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestBasketMapTopicManager_IdentifyAdmissibleOutputs_ValidToken(t *testing.T) {
	tm := NewBasketMapTopicManager()

	// Create a valid BasketMap token with all 7 fields
	fields := [][]byte{
		[]byte("basket-1"),
		[]byte("Test Basket"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		[]byte("02abcdef1234567890"),
		[]byte("signature_data"),
	}

	lockingScript, err := createPushDropScript(fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createBasketMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestBasketMapTopicManager_IdentifyAdmissibleOutputs_RetainsPreviousCoins(t *testing.T) {
	tm := NewBasketMapTopicManager()

	// Create a valid token
	fields := [][]byte{
		[]byte("basket-1"),
		[]byte("Test Basket"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		[]byte("02abcdef1234567890"),
		[]byte("signature_data"),
	}

	lockingScript, err := createPushDropScript(fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createBasketMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.ElementsMatch(t, previousCoins, instructions.CoinsToRetain)
}

func TestBasketMapTopicManager_IdentifyAdmissibleOutputs_MissingInputs(t *testing.T) {
	tm := NewBasketMapTopicManager()

	// Create a transaction with no inputs (BasketMap requires at least 1 input)
	tx := transaction.NewTransaction()
	fields := [][]byte{
		[]byte("basket-1"),
		[]byte("Test Basket"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		[]byte("02abcdef1234567890"),
		[]byte("signature_data"),
	}

	lockingScript, err := createPushDropScript(fields)
	require.NoError(t, err)

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

// Helper functions

// createBasketMapTransactionWithInput creates a transaction with a valid funding input
// and optionally adds an output
func createBasketMapTransactionWithInput(t *testing.T, output *transaction.TransactionOutput) (*transaction.Transaction, error) {
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

func createPushDropScript(fields [][]byte) (*script.Script, error) {
	// PushDrop format: <pubkey_length> <pubkey> OP_CHECKSIG <fields...> <2DROP...> <DROP?>
	// Generate a valid public key
	privateKey, err := ec.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	pubKeyBytes := privateKey.PubKey().Compressed()

	// Start with locking key
	allChunks := []*script.ScriptChunk{
		{Op: byte(len(pubKeyBytes)), Data: pubKeyBytes},
		{Op: script.OpCHECKSIG},
	}

	// Add field data
	for _, field := range fields {
		allChunks = append(allChunks, pushdrop.CreateMinimallyEncodedScriptChunk(field))
	}

	// Add DROP operations
	notYetDropped := len(fields)
	for notYetDropped > 1 {
		allChunks = append(allChunks, &script.ScriptChunk{Op: script.Op2DROP})
		notYetDropped -= 2
	}
	if notYetDropped != 0 {
		allChunks = append(allChunks, &script.ScriptChunk{Op: script.OpDROP})
	}

	return script.NewScriptFromScriptOps(allChunks)
}
