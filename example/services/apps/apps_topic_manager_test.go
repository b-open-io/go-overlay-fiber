package apps

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppsTopicManager_NewInstance(t *testing.T) {
	tm := NewAppsTopicManager()
	require.NotNil(t, tm)
}

func TestAppsTopicManager_GetDocumentation(t *testing.T) {
	tm := NewAppsTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "Apps Topic Manager")
	assert.Contains(t, docs, "PushDrop")
	assert.Contains(t, docs, "metanet apps")
}

func TestAppsTopicManager_GetMetaData(t *testing.T) {
	tm := NewAppsTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "Apps Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
	assert.Equal(t, "0.1.0", meta.Version)
}

func TestAppsTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewAppsTopicManager()

	// Apps protocol doesn't need any inputs for validation
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewAppsTopicManager()

	// Create a transaction with an input but no outputs
	sourceTx := transaction.NewTransaction()
	sourceTx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: &script.Script{},
	})

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:        sourceTx.TxID(),
		SourceTransaction: sourceTx,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewAppsTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_NonPushDropOutput(t *testing.T) {
	tm := NewAppsTopicManager()

	// Create a transaction with a simple output (not PushDrop)
	tx := transaction.NewTransaction()
	// Add a dummy input
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID: &chainhash.Hash{},
	})
	addSourceTransaction(tx)

	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
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

func TestAppsTopicManager_IdentifyAdmissibleOutputs_WrongFieldCount(t *testing.T) {
	tm := NewAppsTopicManager()

	// Create a PushDrop token with wrong number of fields (should be 2: metadata + signature)
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Create a single field (not enough)
	metadata := PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   privateKey.PubKey().ToDERHex(),
		ReleaseDate: "2025-01-01",
	}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)

	fields := [][]byte{
		metadataJSON,
		// Missing signature field - we'll add 3 fields total to make it wrong
		[]byte("extra field"),
		[]byte("another extra"),
	}

	lockingScript, err := createUnsignedPushDropToken(privateKey.PubKey(), fields)
	require.NoError(t, err)

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID: &chainhash.Hash{},
	})
	addSourceTransaction(tx)

	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_InvalidJSON(t *testing.T) {
	tm := NewAppsTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Create fields with invalid JSON in first field
	fields := [][]byte{
		[]byte("not valid json"),
	}

	lockingScript, err := createSignedAppToken(privateKey, fields)
	require.NoError(t, err)

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID: &chainhash.Hash{},
	})
	addSourceTransaction(tx)

	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_MissingRequiredFields(t *testing.T) {
	tm := NewAppsTopicManager()

	testCases := []struct {
		name     string
		metadata PublishedAppMetadata
	}{
		{
			name: "missing version",
			metadata: PublishedAppMetadata{
				Name:        "Test App",
				Description: "A test application",
				Icon:        "https://example.com/icon.png",
				HTTPURL:     "https://example.com",
				Domain:      "example.com",
				Publisher:   "02pubkey",
				ReleaseDate: "2025-01-01",
			},
		},
		{
			name: "missing name",
			metadata: PublishedAppMetadata{
				Version:     "1.0.0",
				Description: "A test application",
				Icon:        "https://example.com/icon.png",
				HTTPURL:     "https://example.com",
				Domain:      "example.com",
				Publisher:   "02pubkey",
				ReleaseDate: "2025-01-01",
			},
		},
		{
			name: "missing description",
			metadata: PublishedAppMetadata{
				Version:     "1.0.0",
				Name:        "Test App",
				Icon:        "https://example.com/icon.png",
				HTTPURL:     "https://example.com",
				Domain:      "example.com",
				Publisher:   "02pubkey",
				ReleaseDate: "2025-01-01",
			},
		},
		{
			name: "missing icon",
			metadata: PublishedAppMetadata{
				Version:     "1.0.0",
				Name:        "Test App",
				Description: "A test application",
				HTTPURL:     "https://example.com",
				Domain:      "example.com",
				Publisher:   "02pubkey",
				ReleaseDate: "2025-01-01",
			},
		},
		{
			name: "missing both httpURL and uhrpURL",
			metadata: PublishedAppMetadata{
				Version:     "1.0.0",
				Name:        "Test App",
				Description: "A test application",
				Icon:        "https://example.com/icon.png",
				Domain:      "example.com",
				Publisher:   "02pubkey",
				ReleaseDate: "2025-01-01",
			},
		},
		{
			name: "missing domain",
			metadata: PublishedAppMetadata{
				Version:     "1.0.0",
				Name:        "Test App",
				Description: "A test application",
				Icon:        "https://example.com/icon.png",
				HTTPURL:     "https://example.com",
				Publisher:   "02pubkey",
				ReleaseDate: "2025-01-01",
			},
		},
		{
			name: "missing publisher",
			metadata: PublishedAppMetadata{
				Version:     "1.0.0",
				Name:        "Test App",
				Description: "A test application",
				Icon:        "https://example.com/icon.png",
				HTTPURL:     "https://example.com",
				Domain:      "example.com",
				ReleaseDate: "2025-01-01",
			},
		},
		{
			name: "missing release_date",
			metadata: PublishedAppMetadata{
				Version:     "1.0.0",
				Name:        "Test App",
				Description: "A test application",
				Icon:        "https://example.com/icon.png",
				HTTPURL:     "https://example.com",
				Domain:      "example.com",
				Publisher:   "02pubkey",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			privateKey, err := ec.NewPrivateKey()
			require.NoError(t, err)

			metadataJSON, err := json.Marshal(tc.metadata)
			require.NoError(t, err)

			fields := [][]byte{metadataJSON}
			lockingScript, err := createSignedAppToken(privateKey, fields)
			require.NoError(t, err)

			tx := transaction.NewTransaction()
			tx.AddInput(&transaction.TransactionInput{
				SourceTXID: &chainhash.Hash{},
			})
			addSourceTransaction(tx)

			tx.AddOutput(&transaction.TransactionOutput{
				Satoshis:      1,
				LockingScript: lockingScript,
			})

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Empty(t, instructions.OutputsToAdmit, "output should not be admitted with %s", tc.name)
		})
	}
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_ValidAppToken(t *testing.T) {
	tm := NewAppsTopicManager()

	tx, err := createValidAppTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_ValidAppTokenWithUHRPURL(t *testing.T) {
	tm := NewAppsTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	metadata := PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		UHRPURL:     "uhrp://example",
		Domain:      "example.com",
		Publisher:   privateKey.PubKey().ToDERHex(),
		ReleaseDate: "2025-01-01",
	}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)

	fields := [][]byte{metadataJSON}
	lockingScript, err := createSignedAppToken(privateKey, fields)
	require.NoError(t, err)

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID: &chainhash.Hash{},
	})
	addSourceTransaction(tx)

	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_RetainsPreviousCoins(t *testing.T) {
	tm := NewAppsTopicManager()

	tx, err := createValidAppTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.ElementsMatch(t, previousCoins, instructions.CoinsToRetain)
}

func TestAppsTopicManager_IdentifyAdmissibleOutputs_NoInputs(t *testing.T) {
	tm := NewAppsTopicManager()

	// Create a transaction without inputs
	tx := transaction.NewTransaction()
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
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

func TestAppsTopicManager_IdentifyAdmissibleOutputs_InvalidSignature(t *testing.T) {
	tm := NewAppsTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	differentKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Create metadata with one key but sign with a different key
	metadata := PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   privateKey.PubKey().ToDERHex(), // Use one key's pubkey
		ReleaseDate: "2025-01-01",
	}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)

	fields := [][]byte{metadataJSON}

	// Sign with a different key
	lockingScript, err := createSignedAppToken(differentKey, fields)
	require.NoError(t, err)

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID: &chainhash.Hash{},
	})
	addSourceTransaction(tx)

	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

// Helper functions

// addSourceTransaction creates a simple source transaction and adds it to the input
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

func createValidAppTransaction(t *testing.T) (*transaction.Transaction, error) {
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	metadata := PublishedAppMetadata{
		Version:     "1.0.0",
		Name:        "Test App",
		Description: "A test application",
		Icon:        "https://example.com/icon.png",
		HTTPURL:     "https://example.com",
		Domain:      "example.com",
		Publisher:   privateKey.PubKey().ToDERHex(),
		ReleaseDate: "2025-01-01",
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	fields := [][]byte{metadataJSON}
	lockingScript, err := createSignedAppToken(privateKey, fields)
	if err != nil {
		return nil, err
	}

	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID: &chainhash.Hash{},
	})
	addSourceTransaction(tx)

	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	})

	return tx, nil
}

func createSignedAppToken(privateKey *ec.PrivateKey, fields [][]byte) (*script.Script, error) {
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

	// Sign using BRC-48 protocol with [1, 'metanet apps']
	signArgs := wallet.CreateSignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "metanet apps",
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

	// Get the locking public key from the "anyone" wallet's perspective
	// This matches how verification will derive the key
	anyoneWallet, err := wallet.NewProtoWallet(wallet.ProtoWalletArgs{
		Type: wallet.ProtoWalletArgsTypeAnyone,
	})
	if err != nil {
		return nil, err
	}

	publicKeyArgs := wallet.GetPublicKeyArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "metanet apps",
			},
			KeyID: "1",
			Counterparty: wallet.Counterparty{
				Type:         wallet.CounterpartyTypeOther,
				Counterparty: privateKey.PubKey(),
			},
		},
	}

	derivedKeyResult, err := anyoneWallet.GetPublicKey(context.Background(), publicKeyArgs, "")
	if err != nil {
		return nil, err
	}

	// Build the PushDrop script with the derived locking key
	pubKeyBytes := derivedKeyResult.PublicKey.Compressed()
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

func createUnsignedPushDropToken(pubKey *ec.PublicKey, fields [][]byte) (*script.Script, error) {
	// Build the PushDrop script manually without signature
	pubKeyBytes := pubKey.Compressed()
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
