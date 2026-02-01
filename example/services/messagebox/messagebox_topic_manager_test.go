package messagebox

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

func TestMessageBoxTopicManager_NewInstance(t *testing.T) {
	tm := NewMessageBoxTopicManager()
	require.NotNil(t, tm)
}

func TestMessageBoxTopicManager_GetDocumentation(t *testing.T) {
	tm := NewMessageBoxTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "MessageBox Topic Manager")
	assert.Contains(t, docs, "tm_messagebox")
	assert.Contains(t, docs, "PushDrop")
}

func TestMessageBoxTopicManager_GetMetaData(t *testing.T) {
	tm := NewMessageBoxTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "MessageBox Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestMessageBoxTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	// MessageBox protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestMessageBoxTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	// Create a transaction with an input but no outputs
	tx, err := createMessageBoxTransactionWithInput(t, nil)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestMessageBoxTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestMessageBoxTopicManager_IdentifyAdmissibleOutputs_NonPushDropOutput(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	// Create a transaction with a simple output (not PushDrop)
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
	output := &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	}

	tx, err := createMessageBoxTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestMessageBoxTopicManager_IdentifyAdmissibleOutputs_WrongFieldCount(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Create PushDrop with only 2 fields (need 3: identityKey, host, signature)
	fields := [][]byte{
		privateKey.PubKey().Compressed(),
		[]byte("https://example.com"),
	}

	lockingScript, err := createMessageBoxPushDropScript(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createMessageBoxTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestMessageBoxTopicManager_IdentifyAdmissibleOutputs_EmptyFields(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	testCases := []struct {
		name        string
		identityKey []byte
		host        []byte
	}{
		{
			name:        "empty identity key",
			identityKey: []byte{},
			host:        []byte("https://example.com"),
		},
		{
			name:        "empty host",
			identityKey: nil, // Will be set to a valid key
			host:        []byte{},
		},
		{
			name:        "both empty",
			identityKey: []byte{},
			host:        []byte{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			privateKey, err := ec.NewPrivateKey()
			require.NoError(t, err)

			identityKey := tc.identityKey
			if identityKey == nil {
				identityKey = privateKey.PubKey().Compressed()
			}

			// Create data to sign
			dataToSign := append(identityKey, tc.host...)
			signature, err := privateKey.Sign(dataToSign)
			require.NoError(t, err)

			fields := [][]byte{
				identityKey,
				tc.host,
				signature.Serialize(),
			}

			lockingScript, err := createMessageBoxPushDropScript(privateKey, fields)
			require.NoError(t, err)

			output := &transaction.TransactionOutput{
				Satoshis:      1,
				LockingScript: lockingScript,
			}

			tx, err := createMessageBoxTransactionWithInput(t, output)
			require.NoError(t, err)

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Empty(t, instructions.OutputsToAdmit)
		})
	}
}

func TestMessageBoxTopicManager_IdentifyAdmissibleOutputs_ValidAdvertisement(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	tx, err := createValidMessageBoxTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestMessageBoxTopicManager_IdentifyAdmissibleOutputs_InvalidSignature(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	identityKey := privateKey.PubKey().Compressed()
	host := []byte("https://example.com")

	// Sign with different data than what will be verified
	wrongData := []byte("wrong data")
	signature, err := privateKey.Sign(wrongData)
	require.NoError(t, err)

	fields := [][]byte{
		identityKey,
		host,
		signature.Serialize(),
	}

	lockingScript, err := createMessageBoxPushDropScript(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createMessageBoxTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestMessageBoxTopicManager_IdentifyAdmissibleOutputs_RetainsPreviousCoins(t *testing.T) {
	tm := NewMessageBoxTopicManager()

	tx, err := createValidMessageBoxTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.ElementsMatch(t, previousCoins, instructions.CoinsToRetain)
}

// Helper functions

// createMessageBoxTransactionWithInput creates a transaction with a valid funding input
// and optionally adds an output
func createMessageBoxTransactionWithInput(t *testing.T, output *transaction.TransactionOutput) (*transaction.Transaction, error) {
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

func createValidMessageBoxTransaction(t *testing.T) (*transaction.Transaction, error) {
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	identityKey := privateKey.PubKey().Compressed()
	host := []byte("https://example-messagebox.com")

	// Create data to sign: identityKey + host
	dataToSign := append(identityKey, host...)
	signature, err := privateKey.Sign(dataToSign)
	if err != nil {
		return nil, err
	}

	fields := [][]byte{
		identityKey,
		host,
		signature.Serialize(),
	}

	lockingScript, err := createMessageBoxPushDropScript(privateKey, fields)
	if err != nil {
		return nil, err
	}

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	return createMessageBoxTransactionWithInput(t, output)
}

func createMessageBoxPushDropScript(privateKey *ec.PrivateKey, fields [][]byte) (*script.Script, error) {
	// Build the PushDrop script manually
	pubKeyBytes := privateKey.PubKey().Compressed()
	lockChunks := []*script.ScriptChunk{
		{Op: byte(len(pubKeyBytes)), Data: pubKeyBytes},
		{Op: script.OpCHECKSIG},
	}

	pushDropChunks := make([]*script.ScriptChunk, 0)
	for _, field := range fields {
		pushDropChunks = append(pushDropChunks, pushdrop.CreateMinimallyEncodedScriptChunk(field))
	}

	notYetDropped := len(fields)
	for notYetDropped > 1 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.Op2DROP})
		notYetDropped -= 2
	}
	if notYetDropped != 0 {
		pushDropChunks = append(pushDropChunks, &script.ScriptChunk{Op: script.OpDROP})
	}

	return script.NewScriptFromScriptOps(append(lockChunks, pushDropChunks...))
}
