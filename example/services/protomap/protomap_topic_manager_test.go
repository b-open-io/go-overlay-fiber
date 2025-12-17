package protomap

import (
	"context"
	"encoding/json"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtoMapTopicManager_NewInstance(t *testing.T) {
	tm := NewProtoMapTopicManager()
	require.NotNil(t, tm)
}

func TestProtoMapTopicManager_GetDocumentation(t *testing.T) {
	tm := NewProtoMapTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "ProtoMap Topic Manager")
	assert.Contains(t, docs, "PushDrop")
	assert.Contains(t, docs, "7 PushDrop fields")
}

func TestProtoMapTopicManager_GetMetaData(t *testing.T) {
	tm := NewProtoMapTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "ProtoMap Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestProtoMapTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewProtoMapTopicManager()

	// ProtoMap protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewProtoMapTopicManager()

	// Create a transaction with an input but no outputs
	tx, err := createProtoMapTransactionWithInput(t, nil)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewProtoMapTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_WrongFieldCount(t *testing.T) {
	tm := NewProtoMapTopicManager()

	// Create PushDrop with only 5 fields (should have 7)
	fields := [][]byte{
		[]byte(`[1, "test-protocol"]`),
		[]byte("Test Protocol"),
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

	tx, err := createProtoMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_InvalidProtocolIDJSON(t *testing.T) {
	tm := NewProtoMapTopicManager()

	testCases := []struct {
		name        string
		protocolID  string
		description string
	}{
		{
			name:        "not JSON",
			protocolID:  "not-json",
			description: "Invalid JSON string",
		},
		{
			name:        "wrong array length",
			protocolID:  `[1]`,
			description: "Array with only one element",
		},
		{
			name:        "wrong array length three",
			protocolID:  `[1, "test", "extra"]`,
			description: "Array with three elements",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fields := [][]byte{
				[]byte(tc.protocolID),
				[]byte("Test Protocol"),
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

			tx, err := createProtoMapTransactionWithInput(t, output)
			require.NoError(t, err)

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Empty(t, instructions.OutputsToAdmit)
		})
	}
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_InvalidSecurityLevel(t *testing.T) {
	tm := NewProtoMapTopicManager()

	testCases := []struct {
		name       string
		protocolID string
	}{
		{"security level -1", `[-1, "test-protocol"]`},
		{"security level 3", `[3, "test-protocol"]`},
		{"security level 10", `[10, "test-protocol"]`},
		{"security level string", `["0", "test-protocol"]`},
		{"security level null", `[null, "test-protocol"]`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fields := [][]byte{
				[]byte(tc.protocolID),
				[]byte("Test Protocol"),
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

			tx, err := createProtoMapTransactionWithInput(t, output)
			require.NoError(t, err)

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Empty(t, instructions.OutputsToAdmit)
		})
	}
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_EmptyFields(t *testing.T) {
	tm := NewProtoMapTopicManager()

	testCases := []struct {
		name       string
		fieldIndex int
	}{
		{"empty protocolID", 0},
		{"empty name", 1},
		{"empty iconURL", 2},
		{"empty description", 3},
		{"empty documentationURL", 4},
		{"empty registryOperator", 5},
		{"empty signature", 6},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create 7 fields, but make one empty
			fields := [][]byte{
				[]byte(`[1, "test-protocol"]`),
				[]byte("Test Protocol"),
				[]byte("https://example.com/icon.png"),
				[]byte("Test description"),
				[]byte("https://example.com/docs"),
				[]byte("02abcdef1234567890"),
				[]byte("signature_data"),
			}
			fields[tc.fieldIndex] = []byte{}

			lockingScript, err := createPushDropScript(fields)
			require.NoError(t, err)

			output := &transaction.TransactionOutput{
				Satoshis:      1,
				LockingScript: lockingScript,
			}

			tx, err := createProtoMapTransactionWithInput(t, output)
			require.NoError(t, err)

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Empty(t, instructions.OutputsToAdmit)
		})
	}
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_ValidToken(t *testing.T) {
	tm := NewProtoMapTopicManager()

	// Test all valid security levels: 0, 1, 2
	testCases := []struct {
		name          string
		securityLevel int
	}{
		{"security level 0", 0},
		{"security level 1", 1},
		{"security level 2", 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			protocolID := map[string]interface{}{
				"securityLevel": tc.securityLevel,
				"protocol":      "test-protocol",
			}
			protocolIDJSON, err := json.Marshal([]interface{}{tc.securityLevel, "test-protocol"})
			require.NoError(t, err)

			fields := [][]byte{
				protocolIDJSON,
				[]byte("Test Protocol"),
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

			tx, err := createProtoMapTransactionWithInput(t, output)
			require.NoError(t, err)

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Len(t, instructions.OutputsToAdmit, 1)
			assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])

			// Suppress unused variable warning
			_ = protocolID
		})
	}
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_RetainsPreviousCoins(t *testing.T) {
	tm := NewProtoMapTopicManager()

	// Create a valid token
	fields := [][]byte{
		[]byte(`[1, "test-protocol"]`),
		[]byte("Test Protocol"),
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

	tx, err := createProtoMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.ElementsMatch(t, previousCoins, instructions.CoinsToRetain)
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_MissingInputs(t *testing.T) {
	tm := NewProtoMapTopicManager()

	// Create a transaction with no inputs (ProtoMap requires at least 1 input)
	tx := transaction.NewTransaction()
	fields := [][]byte{
		[]byte(`[1, "test-protocol"]`),
		[]byte("Test Protocol"),
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

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_NonPushDropOutput(t *testing.T) {
	tm := NewProtoMapTopicManager()

	// Create a transaction with a simple output (not PushDrop)
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
	output := &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	}

	tx, err := createProtoMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestProtoMapTopicManager_IdentifyAdmissibleOutputs_InvalidProtocolType(t *testing.T) {
	tm := NewProtoMapTopicManager()

	// Protocol name should be a string, not a number
	fields := [][]byte{
		[]byte(`[1, 123]`), // protocol is a number instead of string
		[]byte("Test Protocol"),
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

	tx, err := createProtoMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

// Helper functions

// createProtoMapTransactionWithInput creates a transaction with a valid funding input
// and optionally adds an output
func createProtoMapTransactionWithInput(t *testing.T, output *transaction.TransactionOutput) (*transaction.Transaction, error) {
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
