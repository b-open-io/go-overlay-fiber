package certmap

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertMapTopicManager_NewInstance(t *testing.T) {
	tm := NewCertMapTopicManager()
	require.NotNil(t, tm)
}

func TestCertMapTopicManager_GetDocumentation(t *testing.T) {
	tm := NewCertMapTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "CertMap Topic Manager")
	assert.Contains(t, docs, "PushDrop")
}

func TestCertMapTopicManager_GetMetaData(t *testing.T) {
	tm := NewCertMapTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "CertMap Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
}

func TestCertMapTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewCertMapTopicManager()

	// CertMap protocol doesn't need any inputs for validation
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewCertMapTopicManager()

	// Create a transaction with no outputs
	tx := transaction.NewTransaction()
	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	// CertMap checks for inputs first, so it will error on missing inputs
	assert.Contains(t, err.Error(), "inputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewCertMapTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_NonPushDropOutput(t *testing.T) {
	tm := NewCertMapTopicManager()

	// Create a transaction with a simple output (not PushDrop)
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
	output := &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	}

	tx, err := createCertMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_WrongFieldCount(t *testing.T) {
	tm := NewCertMapTopicManager()

	// Create a transaction with a PushDrop output but wrong field count (6 instead of 8)
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("test-type"),
		[]byte("Test Name"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		[]byte(`{"field1":"value1"}`),
		// Missing registryOperator field (should have 7 data fields + signature = 8 total)
	}

	lockingScript, err := createSignedCertMapToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createCertMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_InvalidCertFieldsJSON(t *testing.T) {
	tm := NewCertMapTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("test-type"),
		[]byte("Test Name"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		[]byte(`{invalid json}`), // Invalid JSON
		[]byte(privateKey.PubKey().ToDERHex()),
	}

	lockingScript, err := createSignedCertMapToken(privateKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createCertMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_MissingRequiredFields(t *testing.T) {
	tm := NewCertMapTopicManager()

	tests := []struct {
		name     string
		fields   [][]byte
		skipDesc string
	}{
		{
			name: "missing type",
			fields: [][]byte{
				[]byte(""),
				[]byte("Test Name"),
				[]byte("https://example.com/icon.png"),
				[]byte("Test description"),
				[]byte("https://example.com/docs"),
				[]byte(`{"field1":"value1"}`),
				nil, // Will be filled with registryOperator
			},
			skipDesc: "empty type field",
		},
		{
			name: "missing name",
			fields: [][]byte{
				[]byte("test-type"),
				[]byte(""),
				[]byte("https://example.com/icon.png"),
				[]byte("Test description"),
				[]byte("https://example.com/docs"),
				[]byte(`{"field1":"value1"}`),
				nil,
			},
			skipDesc: "empty name field",
		},
		{
			name: "missing iconURL",
			fields: [][]byte{
				[]byte("test-type"),
				[]byte("Test Name"),
				[]byte(""),
				[]byte("Test description"),
				[]byte("https://example.com/docs"),
				[]byte(`{"field1":"value1"}`),
				nil,
			},
			skipDesc: "empty iconURL field",
		},
		{
			name: "missing description",
			fields: [][]byte{
				[]byte("test-type"),
				[]byte("Test Name"),
				[]byte("https://example.com/icon.png"),
				[]byte(""),
				[]byte("https://example.com/docs"),
				[]byte(`{"field1":"value1"}`),
				nil,
			},
			skipDesc: "empty description field",
		},
		{
			name: "missing documentationURL",
			fields: [][]byte{
				[]byte("test-type"),
				[]byte("Test Name"),
				[]byte("https://example.com/icon.png"),
				[]byte("Test description"),
				[]byte(""),
				[]byte(`{"field1":"value1"}`),
				nil,
			},
			skipDesc: "empty documentationURL field",
		},
		{
			name: "missing registryOperator",
			fields: [][]byte{
				[]byte("test-type"),
				[]byte("Test Name"),
				[]byte("https://example.com/icon.png"),
				[]byte("Test description"),
				[]byte("https://example.com/docs"),
				[]byte(`{"field1":"value1"}`),
				[]byte(""),
			},
			skipDesc: "empty registryOperator field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privateKey, err := ec.NewPrivateKey()
			require.NoError(t, err)

			// Fill in registryOperator if nil
			fields := make([][]byte, len(tt.fields))
			copy(fields, tt.fields)
			if fields[6] == nil {
				fields[6] = []byte(privateKey.PubKey().ToDERHex())
			}

			lockingScript, err := createSignedCertMapToken(privateKey, fields)
			require.NoError(t, err)

			output := &transaction.TransactionOutput{
				Satoshis:      1,
				LockingScript: lockingScript,
			}

			tx, err := createCertMapTransactionWithInput(t, output)
			require.NoError(t, err)

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Empty(t, instructions.OutputsToAdmit)
		})
	}
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_ValidCertMapToken(t *testing.T) {
	tm := NewCertMapTopicManager()

	// Create a valid CertMap token
	tx, err := createValidCertMapTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_RetainsPreviousCoins(t *testing.T) {
	tm := NewCertMapTopicManager()

	tx, err := createValidCertMapTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.ElementsMatch(t, previousCoins, instructions.CoinsToRetain)
}

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_MissingInputs(t *testing.T) {
	tm := NewCertMapTopicManager()

	// Create a transaction with no inputs (CertMap requires at least one input)
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	certFields := map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	}
	certFieldsJSON, err := json.Marshal(certFields)
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("test-type"),
		[]byte("Test Name"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		certFieldsJSON,
		[]byte(privateKey.PubKey().ToDERHex()),
	}

	lockingScript, err := createSignedCertMapToken(privateKey, fields)
	require.NoError(t, err)

	tx := transaction.NewTransaction()
	// No inputs added
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

func TestCertMapTopicManager_IdentifyAdmissibleOutputs_InvalidSignature(t *testing.T) {
	tm := NewCertMapTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	differentKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	certFields := map[string]interface{}{
		"field1": "value1",
	}
	certFieldsJSON, err := json.Marshal(certFields)
	require.NoError(t, err)

	// Use one key for the registry operator field but sign with a different key
	fields := [][]byte{
		[]byte("test-type"),
		[]byte("Test Name"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		certFieldsJSON,
		[]byte(privateKey.PubKey().ToDERHex()), // registry operator identity key
	}

	// Sign with a different key
	lockingScript, err := createSignedCertMapTokenWithDifferentSigner(differentKey, privateKey.PubKey(), fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createCertMapTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

// Helper functions

func createValidCertMapTransaction(t *testing.T) (*transaction.Transaction, error) {
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	certFields := map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	}
	certFieldsJSON, err := json.Marshal(certFields)
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("test-type"),
		[]byte("Test Name"),
		[]byte("https://example.com/icon.png"),
		[]byte("Test description"),
		[]byte("https://example.com/docs"),
		certFieldsJSON,
		[]byte(privateKey.PubKey().ToDERHex()),
	}

	lockingScript, err := createSignedCertMapToken(privateKey, fields)
	if err != nil {
		return nil, err
	}

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	return createCertMapTransactionWithInput(t, output)
}

// createCertMapTransactionWithInput creates a transaction with a valid funding input
// and optionally adds an output
func createCertMapTransactionWithInput(t *testing.T, output *transaction.TransactionOutput) (*transaction.Transaction, error) {
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

func createSignedCertMapToken(privateKey *ec.PrivateKey, fields [][]byte) (*script.Script, error) {
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

	// Sign using BRC-48 protocol with anyone counterparty
	// The registry operator signs "for anyone" which means anyone can verify the signature
	signArgs := wallet.CreateSignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "certmap",
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

	// Get the expected locking public key (derived via BRC-48)
	// This should match what anyoneWallet.GetPublicKey would derive with the registry operator as counterparty
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
				Protocol:      "certmap",
			},
			KeyID: "1",
			Counterparty: wallet.Counterparty{
				Type:         wallet.CounterpartyTypeOther,
				Counterparty: privateKey.PubKey(),
			},
		},
	}

	derivedKey, err := anyoneWallet.GetPublicKey(context.Background(), publicKeyArgs, "")
	if err != nil {
		return nil, err
	}

	// Build the PushDrop script manually
	pubKeyBytes := derivedKey.PublicKey.Compressed()
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

func createSignedCertMapTokenWithDifferentSigner(signerKey *ec.PrivateKey, lockingPubKey *ec.PublicKey, fields [][]byte) (*script.Script, error) {
	// Create wallet for signing with a different key
	protoWallet, err := wallet.NewProtoWallet(wallet.ProtoWalletArgs{
		Type:       wallet.ProtoWalletArgsTypePrivateKey,
		PrivateKey: signerKey,
	})
	if err != nil {
		return nil, err
	}

	// Concatenate fields for signing
	var data bytes.Buffer
	for _, field := range fields {
		data.Write(field)
	}

	// Sign using BRC-48 protocol with anyone counterparty
	signArgs := wallet.CreateSignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "certmap",
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

	// Get derived key from anyone wallet's perspective with the claimed registry operator
	// This should create a valid locking key, but the signature won't verify with that key
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
				Protocol:      "certmap",
			},
			KeyID: "1",
			Counterparty: wallet.Counterparty{
				Type:         wallet.CounterpartyTypeOther,
				Counterparty: lockingPubKey,
			},
		},
	}

	derivedKey, err := anyoneWallet.GetPublicKey(context.Background(), publicKeyArgs, "")
	if err != nil {
		return nil, err
	}

	// Build the script with derived key (signature won't verify because it was signed by a different key)
	pubKeyBytes := derivedKey.PublicKey.Compressed()
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
