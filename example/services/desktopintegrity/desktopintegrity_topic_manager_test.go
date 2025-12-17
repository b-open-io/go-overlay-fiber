package desktopintegrity

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDesktopIntegrityTopicManager_NewInstance(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()
	require.NotNil(t, tm)
}

func TestDesktopIntegrityTopicManager_GetDocumentation(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "DesktopIntegrity Topic Manager")
	assert.Contains(t, docs, "tm_desktopintegrity")
	assert.Contains(t, docs, "OP_FALSE OP_RETURN")
}

func TestDesktopIntegrityTopicManager_GetMetaData(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "DesktopIntegrity Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
	assert.Equal(t, "0.1.0", meta.Version)
}

func TestDesktopIntegrityTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()

	// DesktopIntegrity protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestDesktopIntegrityTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()

	// Create a transaction with no outputs
	tx := transaction.NewTransaction()
	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestDesktopIntegrityTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestDesktopIntegrityTopicManager_IdentifyAdmissibleOutputs_WrongChunkCount(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()

	// Create a transaction with wrong chunk count (3 chunks instead of 2)
	tx := transaction.NewTransaction()

	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpFALSE)
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("extra data"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)

	// Verify no outputs are admitted
	assert.Empty(t, instructions.OutputsToAdmit)
	assert.Empty(t, instructions.CoinsToRetain)
}

func TestDesktopIntegrityTopicManager_IdentifyAdmissibleOutputs_WrongOpcodes(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()

	testCases := []struct {
		name        string
		buildScript func() *script.Script
		description string
	}{
		{
			name: "Missing OP_FALSE",
			buildScript: func() *script.Script {
				s := &script.Script{}
				_ = s.AppendOpcodes(script.OpRETURN)
				return s
			},
			description: "Script with only OP_RETURN (missing OP_FALSE)",
		},
		{
			name: "Wrong first opcode",
			buildScript: func() *script.Script {
				s := &script.Script{}
				_ = s.AppendOpcodes(script.OpTRUE)
				_ = s.AppendOpcodes(script.OpRETURN)
				return s
			},
			description: "Script with OP_TRUE instead of OP_FALSE",
		},
		{
			name: "Wrong second opcode",
			buildScript: func() *script.Script {
				s := &script.Script{}
				_ = s.AppendOpcodes(script.OpFALSE)
				_ = s.AppendOpcodes(script.OpDROP)
				return s
			},
			description: "Script with OP_DROP instead of OP_RETURN",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := transaction.NewTransaction()
			tx.AddOutput(&transaction.TransactionOutput{
				Satoshis:      1000,
				LockingScript: tc.buildScript(),
			})

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)

			// Verify no outputs are admitted
			assert.Empty(t, instructions.OutputsToAdmit)
			assert.Empty(t, instructions.CoinsToRetain)
		})
	}
}

func TestDesktopIntegrityTopicManager_IdentifyAdmissibleOutputs_ValidOutput(t *testing.T) {
	tm := NewDesktopIntegrityTopicManager()

	// Create a transaction with a valid DesktopIntegrity output
	// Pattern: OP_FALSE OP_RETURN (exactly 2 chunks)
	tx := transaction.NewTransaction()

	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpFALSE)
	_ = s.AppendOpcodes(script.OpRETURN)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)

	// Verify output is admitted
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])

	// Verify no coins are retained (DesktopIntegrity protocol never retains)
	assert.Empty(t, instructions.CoinsToRetain)
}
