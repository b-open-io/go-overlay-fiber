package slackthreads

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackThreadsTopicManager_NewInstance(t *testing.T) {
	tm := NewSlackThreadsTopicManager()
	require.NotNil(t, tm)
}

func TestSlackThreadsTopicManager_GetDocumentation(t *testing.T) {
	tm := NewSlackThreadsTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "SlackThread Topic Manager")
	assert.Contains(t, docs, "tm_slackthread")
	assert.Contains(t, docs, "OP_SHA256")
	assert.Contains(t, docs, "OP_EQUAL")
}

func TestSlackThreadsTopicManager_GetMetaData(t *testing.T) {
	tm := NewSlackThreadsTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "SlackThreads Topic Manager", meta.Name)
	assert.Equal(t, "Saves hashes of slack threads", meta.Description)
}

func TestSlackThreadsTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewSlackThreadsTopicManager()

	// SlackThreads protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestSlackThreadsTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewSlackThreadsTopicManager()

	// Create a transaction with no outputs
	tx := transaction.NewTransaction()
	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestSlackThreadsTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewSlackThreadsTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestSlackThreadsTopicManager_IdentifyAdmissibleOutputs_WrongChunkCount(t *testing.T) {
	tm := NewSlackThreadsTopicManager()

	// Create a transaction with wrong number of chunks (not 3)
	tx := transaction.NewTransaction()
	s := &script.Script{}
	_ = s.AppendOpcodes(OP_SHA256)
	// Only 1 chunk instead of 3
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)

	// Test with 2 chunks
	tx2 := transaction.NewTransaction()
	s2 := &script.Script{}
	_ = s2.AppendOpcodes(OP_SHA256)
	_ = s2.AppendPushData(make([]byte, 32))
	tx2.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s2,
	})

	beef2, err := tx2.BEEF()
	require.NoError(t, err)

	instructions2, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef2, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions2.OutputsToAdmit)

	// Test with 4 chunks
	tx3 := transaction.NewTransaction()
	s3 := &script.Script{}
	_ = s3.AppendOpcodes(OP_SHA256)
	_ = s3.AppendPushData(make([]byte, 32))
	_ = s3.AppendOpcodes(OP_EQUAL)
	_ = s3.AppendOpcodes(script.OpRETURN) // Extra chunk
	tx3.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s3,
	})

	beef3, err := tx3.BEEF()
	require.NoError(t, err)

	instructions3, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef3, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions3.OutputsToAdmit)
}

func TestSlackThreadsTopicManager_IdentifyAdmissibleOutputs_WrongHashLength(t *testing.T) {
	tm := NewSlackThreadsTopicManager()

	// Test with hash too short (31 bytes)
	tx := transaction.NewTransaction()
	s := &script.Script{}
	_ = s.AppendOpcodes(OP_SHA256)
	_ = s.AppendPushData(make([]byte, 31)) // 31 bytes instead of 32
	_ = s.AppendOpcodes(OP_EQUAL)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)

	// Test with hash too long (33 bytes)
	tx2 := transaction.NewTransaction()
	s2 := &script.Script{}
	_ = s2.AppendOpcodes(OP_SHA256)
	_ = s2.AppendPushData(make([]byte, 33)) // 33 bytes instead of 32
	_ = s2.AppendOpcodes(OP_EQUAL)
	tx2.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s2,
	})

	beef2, err := tx2.BEEF()
	require.NoError(t, err)

	instructions2, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef2, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions2.OutputsToAdmit)
}

func TestSlackThreadsTopicManager_IdentifyAdmissibleOutputs_WrongOpcodes(t *testing.T) {
	tm := NewSlackThreadsTopicManager()

	// Test with wrong first opcode (not OP_SHA256)
	tx := transaction.NewTransaction()
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN) // Wrong opcode
	_ = s.AppendPushData(make([]byte, 32))
	_ = s.AppendOpcodes(OP_EQUAL)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)

	// Test with wrong third opcode (not OP_EQUAL)
	tx2 := transaction.NewTransaction()
	s2 := &script.Script{}
	_ = s2.AppendOpcodes(OP_SHA256)
	_ = s2.AppendPushData(make([]byte, 32))
	_ = s2.AppendOpcodes(script.OpRETURN) // Wrong opcode
	tx2.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s2,
	})

	beef2, err := tx2.BEEF()
	require.NoError(t, err)

	instructions2, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef2, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions2.OutputsToAdmit)

	// Test with both opcodes wrong
	tx3 := transaction.NewTransaction()
	s3 := &script.Script{}
	_ = s3.AppendOpcodes(script.OpRETURN) // Wrong opcode
	_ = s3.AppendPushData(make([]byte, 32))
	_ = s3.AppendOpcodes(script.Op1) // Wrong opcode
	tx3.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s3,
	})

	beef3, err := tx3.BEEF()
	require.NoError(t, err)

	instructions3, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef3, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions3.OutputsToAdmit)
}

func TestSlackThreadsTopicManager_IdentifyAdmissibleOutputs_ValidOutput(t *testing.T) {
	tm := NewSlackThreadsTopicManager()

	// Create a valid SlackThreads output
	tx := transaction.NewTransaction()

	// Create the locking script: OP_SHA256 <32-byte hash> OP_EQUAL
	s := &script.Script{}
	_ = s.AppendOpcodes(OP_SHA256)
	threadHash := make([]byte, 32)
	// Use a recognizable pattern
	for i := range threadHash {
		threadHash[i] = byte(i)
	}
	_ = s.AppendPushData(threadHash)
	_ = s.AppendOpcodes(OP_EQUAL)

	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
	assert.Empty(t, instructions.CoinsToRetain)

	// Test with multiple valid outputs
	tx2 := transaction.NewTransaction()

	// First valid output
	s1 := &script.Script{}
	_ = s1.AppendOpcodes(OP_SHA256)
	hash1 := make([]byte, 32)
	for i := range hash1 {
		hash1[i] = 0xAA
	}
	_ = s1.AppendPushData(hash1)
	_ = s1.AppendOpcodes(OP_EQUAL)
	tx2.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s1,
	})

	// Second valid output
	s2 := &script.Script{}
	_ = s2.AppendOpcodes(OP_SHA256)
	hash2 := make([]byte, 32)
	for i := range hash2 {
		hash2[i] = 0xBB
	}
	_ = s2.AppendPushData(hash2)
	_ = s2.AppendOpcodes(OP_EQUAL)
	tx2.AddOutput(&transaction.TransactionOutput{
		Satoshis:      2000,
		LockingScript: s2,
	})

	beef2, err := tx2.BEEF()
	require.NoError(t, err)

	instructions2, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef2, nil)
	require.NoError(t, err)
	assert.Len(t, instructions2.OutputsToAdmit, 2)
	assert.Equal(t, uint32(0), instructions2.OutputsToAdmit[0])
	assert.Equal(t, uint32(1), instructions2.OutputsToAdmit[1])
	assert.Empty(t, instructions2.CoinsToRetain)

	// Test with mix of valid and invalid outputs
	tx3 := transaction.NewTransaction()

	// Invalid output
	sInvalid := &script.Script{}
	_ = sInvalid.AppendOpcodes(script.OpRETURN)
	tx3.AddOutput(&transaction.TransactionOutput{
		Satoshis:      500,
		LockingScript: sInvalid,
	})

	// Valid output
	sValid := &script.Script{}
	_ = sValid.AppendOpcodes(OP_SHA256)
	hashValid := make([]byte, 32)
	for i := range hashValid {
		hashValid[i] = 0xCC
	}
	_ = sValid.AppendPushData(hashValid)
	_ = sValid.AppendOpcodes(OP_EQUAL)
	tx3.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1500,
		LockingScript: sValid,
	})

	beef3, err := tx3.BEEF()
	require.NoError(t, err)

	instructions3, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef3, nil)
	require.NoError(t, err)
	assert.Len(t, instructions3.OutputsToAdmit, 1)
	assert.Equal(t, uint32(1), instructions3.OutputsToAdmit[0])
	assert.Empty(t, instructions3.CoinsToRetain)
}
