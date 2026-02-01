package any

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnyTopicManager_NewInstance(t *testing.T) {
	tm := NewAnyTopicManager()
	require.NotNil(t, tm)
}

func TestAnyTopicManager_GetDocumentation(t *testing.T) {
	tm := NewAnyTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "Any Topic Manager")
	assert.Contains(t, docs, "Literally any transaction")
}

func TestAnyTopicManager_GetMetaData(t *testing.T) {
	tm := NewAnyTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Any Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestAnyTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewAnyTopicManager()

	// Any protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestAnyTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewAnyTopicManager()

	// Create a transaction with no outputs
	tx := transaction.NewTransaction()
	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestAnyTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewAnyTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestAnyTopicManager_IdentifyAdmissibleOutputs_AdmitsAllOutputs(t *testing.T) {
	tm := NewAnyTopicManager()

	// Create a transaction with multiple outputs of different types
	tx := transaction.NewTransaction()

	// Add a simple OP_RETURN output
	s1 := &script.Script{}
	_ = s1.AppendOpcodes(script.OpRETURN)
	_ = s1.AppendPushData([]byte("output 1"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s1,
	})

	// Add another OP_RETURN output
	s2 := &script.Script{}
	_ = s2.AppendOpcodes(script.OpRETURN)
	_ = s2.AppendPushData([]byte("output 2"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      2000,
		LockingScript: s2,
	})

	// Add a third OP_RETURN output
	s3 := &script.Script{}
	_ = s3.AppendOpcodes(script.OpRETURN)
	_ = s3.AppendPushData([]byte("output 3"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      3000,
		LockingScript: s3,
	})

	// Add a fourth OP_RETURN output
	s4 := &script.Script{}
	_ = s4.AppendOpcodes(script.OpRETURN)
	_ = s4.AppendPushData([]byte("output 4"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      4000,
		LockingScript: s4,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)

	// Verify all outputs are admitted
	assert.Len(t, instructions.OutputsToAdmit, 4)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
	assert.Equal(t, uint32(1), instructions.OutputsToAdmit[1])
	assert.Equal(t, uint32(2), instructions.OutputsToAdmit[2])
	assert.Equal(t, uint32(3), instructions.OutputsToAdmit[3])

	// Verify no coins are retained (Any protocol never retains)
	assert.Empty(t, instructions.CoinsToRetain)
}
