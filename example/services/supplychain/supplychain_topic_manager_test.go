package supplychain

import (
	"context"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupplyChainTopicManager_NewInstance(t *testing.T) {
	tm := NewSupplyChainTopicManager()
	require.NotNil(t, tm)
}

func TestSupplyChainTopicManager_GetDocumentation(t *testing.T) {
	tm := NewSupplyChainTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "SupplyChain Topic Manager")
	assert.Contains(t, docs, "supply chain")
	assert.Contains(t, docs, "5 chunks")
}

func TestSupplyChainTopicManager_GetMetaData(t *testing.T) {
	tm := NewSupplyChainTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "SupplyChain Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
	assert.Equal(t, "0.1.0", meta.Version)
}

func TestSupplyChainTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	// SupplyChain protocol doesn't need any inputs
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestSupplyChainTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	// Create a transaction with no outputs
	tx := transaction.NewTransaction()
	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestSupplyChainTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestSupplyChainTopicManager_IdentifyAdmissibleOutputs_WrongChunkCount(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	// Create a transaction with wrong number of chunks (not 5)
	tx := transaction.NewTransaction()
	s := &script.Script{}
	_ = s.AppendPushData([]byte("metadata"))
	_ = s.AppendPushData([]byte("additional data"))
	// Missing OP_2DROP, public key, and OP_CHECKSIG
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

func TestSupplyChainTopicManager_IdentifyAdmissibleOutputs_WrongPublicKeyLength(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	// Create a transaction with wrong public key length (not 33 bytes)
	tx := transaction.NewTransaction()
	s := &script.Script{}
	_ = s.AppendPushData([]byte("metadata"))
	_ = s.AppendPushData([]byte("additional data"))
	_ = s.AppendOpcodes(script.Op2DROP)
	_ = s.AppendPushData([]byte("short-pubkey")) // Wrong length
	_ = s.AppendOpcodes(script.OpCHECKSIG)
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

func TestSupplyChainTopicManager_IdentifyAdmissibleOutputs_WrongOpcodes(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	// Create a valid private key and public key
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)
	pubKeyBytes := privateKey.PubKey().Compressed()

	tests := []struct {
		name        string
		buildScript func() *script.Script
		description string
	}{
		{
			name: "wrong_opcode_at_chunk_2",
			buildScript: func() *script.Script {
				s := &script.Script{}
				_ = s.AppendPushData([]byte("metadata"))
				_ = s.AppendPushData([]byte("additional data"))
				_ = s.AppendOpcodes(script.OpDROP) // Should be OP_2DROP
				_ = s.AppendPushData(pubKeyBytes)
				_ = s.AppendOpcodes(script.OpCHECKSIG)
				return s
			},
			description: "chunk 2 should be OP_2DROP",
		},
		{
			name: "wrong_opcode_at_chunk_4",
			buildScript: func() *script.Script {
				s := &script.Script{}
				_ = s.AppendPushData([]byte("metadata"))
				_ = s.AppendPushData([]byte("additional data"))
				_ = s.AppendOpcodes(script.Op2DROP)
				_ = s.AppendPushData(pubKeyBytes)
				_ = s.AppendOpcodes(script.OpCHECKSIGVERIFY) // Should be OP_CHECKSIG
				return s
			},
			description: "chunk 4 should be OP_CHECKSIG",
		},
		{
			name: "chunk_0_not_data_push",
			buildScript: func() *script.Script {
				s := &script.Script{}
				_ = s.AppendOpcodes(script.OpRETURN) // Should be data push
				_ = s.AppendPushData([]byte("additional data"))
				_ = s.AppendOpcodes(script.Op2DROP)
				_ = s.AppendPushData(pubKeyBytes)
				_ = s.AppendOpcodes(script.OpCHECKSIG)
				return s
			},
			description: "chunk 0 should be data push",
		},
		{
			name: "chunk_1_not_data_push",
			buildScript: func() *script.Script {
				s := &script.Script{}
				_ = s.AppendPushData([]byte("metadata"))
				_ = s.AppendOpcodes(script.OpRETURN) // Should be data push
				_ = s.AppendOpcodes(script.Op2DROP)
				_ = s.AppendPushData(pubKeyBytes)
				_ = s.AppendOpcodes(script.OpCHECKSIG)
				return s
			},
			description: "chunk 1 should be data push",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := transaction.NewTransaction()
			tx.AddOutput(&transaction.TransactionOutput{
				Satoshis:      1000,
				LockingScript: tt.buildScript(),
			})

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Empty(t, instructions.OutputsToAdmit, tt.description)
		})
	}
}

func TestSupplyChainTopicManager_IdentifyAdmissibleOutputs_ValidSupplyChainOutput(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	// Create a valid SupplyChain transaction
	tx, err := createValidSupplyChainTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
	// SupplyChain never retains previous coins
	assert.Empty(t, instructions.CoinsToRetain)
}

func TestSupplyChainTopicManager_IdentifyAdmissibleOutputs_MultipleOutputs(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	// Create a transaction with multiple outputs (valid and invalid)
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)
	pubKeyBytes := privateKey.PubKey().Compressed()

	tx := transaction.NewTransaction()

	// Output 0: Valid
	validScript := &script.Script{}
	_ = validScript.AppendPushData([]byte("metadata1"))
	_ = validScript.AppendPushData([]byte("additional data1"))
	_ = validScript.AppendOpcodes(script.Op2DROP)
	_ = validScript.AppendPushData(pubKeyBytes)
	_ = validScript.AppendOpcodes(script.OpCHECKSIG)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: validScript,
	})

	// Output 1: Invalid (wrong chunk count)
	invalidScript := &script.Script{}
	_ = invalidScript.AppendPushData([]byte("metadata2"))
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: invalidScript,
	})

	// Output 2: Valid
	validScript2 := &script.Script{}
	_ = validScript2.AppendPushData([]byte("metadata3"))
	_ = validScript2.AppendPushData([]byte("additional data3"))
	_ = validScript2.AppendOpcodes(script.Op2DROP)
	_ = validScript2.AppendPushData(pubKeyBytes)
	_ = validScript2.AppendOpcodes(script.OpCHECKSIG)
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: validScript2,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 2)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
	assert.Equal(t, uint32(2), instructions.OutputsToAdmit[1])
	assert.Empty(t, instructions.CoinsToRetain)
}

func TestSupplyChainTopicManager_IdentifyAdmissibleOutputs_NoPreviousCoinsRetained(t *testing.T) {
	tm := NewSupplyChainTopicManager()

	tx, err := createValidSupplyChainTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	// Even with previous coins, SupplyChain never retains them
	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Empty(t, instructions.CoinsToRetain)
}

// Helper functions

func createValidSupplyChainTransaction(t *testing.T) (*transaction.Transaction, error) {
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)
	pubKeyBytes := privateKey.PubKey().Compressed()

	// Build valid SupplyChain locking script with exactly 5 chunks
	s := &script.Script{}
	_ = s.AppendPushData([]byte("metadata"))
	_ = s.AppendPushData([]byte("additional data"))
	_ = s.AppendOpcodes(script.Op2DROP)
	_ = s.AppendPushData(pubKeyBytes)
	_ = s.AppendOpcodes(script.OpCHECKSIG)

	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: s,
	})

	return tx, nil
}
