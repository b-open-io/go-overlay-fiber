package walletconfig

import (
	"bytes"
	"context"
	"strings"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletConfigTopicManager_NewInstance(t *testing.T) {
	tm := NewWalletConfigTopicManager()
	require.NotNil(t, tm)
}

func TestWalletConfigTopicManager_GetDocumentation(t *testing.T) {
	tm := NewWalletConfigTopicManager()
	docs := tm.GetDocumentation()
	assert.Contains(t, docs, "WalletConfig Topic Manager")
	assert.Contains(t, docs, "PushDrop")
	assert.Contains(t, docs, "wallet configuration")
}

func TestWalletConfigTopicManager_GetMetaData(t *testing.T) {
	tm := NewWalletConfigTopicManager()
	meta := tm.GetMetaData()
	require.NotNil(t, meta)
	assert.Equal(t, "WalletConfig Topic Manager", meta.Name)
	assert.NotEmpty(t, meta.Description)
	assert.Equal(t, "0.1.0", meta.Version)
}

func TestWalletConfigTopicManager_IdentifyNeededInputs(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	// WalletConfig protocol doesn't need any inputs for validation
	inputs, err := tm.IdentifyNeededInputs(context.Background(), []byte{})
	require.NoError(t, err)
	assert.Nil(t, inputs)
}

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_EmptyOutputs(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	// Create a transaction with an input but no outputs
	tx, err := createWalletConfigTransactionWithInput(t, nil)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outputs")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_InvalidBEEF(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), []byte("invalid"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BEEF")
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_WrongFieldCount(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	// Create a PushDrop token with wrong number of fields (should be 9: 8 data + signature)
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Create token with only 3 fields (too few)
	fields := [][]byte{
		[]byte("config123"),
		[]byte("My Wallet"),
		[]byte("https://example.com/icon.png"),
	}

	lockingScript, err := createUnsignedPushDropToken(privateKey.PubKey(), fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createWalletConfigTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_MissingRequiredFields(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	testCases := []struct {
		name   string
		fields [][]byte
	}{
		{
			name: "missing configID",
			fields: [][]byte{
				[]byte(""),                               // configID - empty
				[]byte("My Wallet"),                      // name
				[]byte("https://example.com/icon.png"),   // icon
				[]byte("https://wab.example.com"),        // wab
				[]byte("https://storage.example.com"),    // storage
				[]byte("https://messagebox.example.com"), // messagebox
				[]byte("https://legal.example.com"),      // legal
				[]byte("02" + strings.Repeat("ab", 32)),  // registryOperator
			},
		},
		{
			name: "missing name",
			fields: [][]byte{
				[]byte("config123"),                      // configID
				[]byte(""),                               // name - empty
				[]byte("https://example.com/icon.png"),   // icon
				[]byte("https://wab.example.com"),        // wab
				[]byte("https://storage.example.com"),    // storage
				[]byte("https://messagebox.example.com"), // messagebox
				[]byte("https://legal.example.com"),      // legal
				[]byte("02" + strings.Repeat("ab", 32)),  // registryOperator
			},
		},
		{
			name: "missing icon",
			fields: [][]byte{
				[]byte("config123"),                      // configID
				[]byte("My Wallet"),                      // name
				[]byte(""),                               // icon - empty
				[]byte("https://wab.example.com"),        // wab
				[]byte("https://storage.example.com"),    // storage
				[]byte("https://messagebox.example.com"), // messagebox
				[]byte("https://legal.example.com"),      // legal
				[]byte("02" + strings.Repeat("ab", 32)),  // registryOperator
			},
		},
		{
			name: "missing wab",
			fields: [][]byte{
				[]byte("config123"),                      // configID
				[]byte("My Wallet"),                      // name
				[]byte("https://example.com/icon.png"),   // icon
				[]byte(""),                               // wab - empty
				[]byte("https://storage.example.com"),    // storage
				[]byte("https://messagebox.example.com"), // messagebox
				[]byte("https://legal.example.com"),      // legal
				[]byte("02" + strings.Repeat("ab", 32)),  // registryOperator
			},
		},
		{
			name: "missing storage",
			fields: [][]byte{
				[]byte("config123"),                      // configID
				[]byte("My Wallet"),                      // name
				[]byte("https://example.com/icon.png"),   // icon
				[]byte("https://wab.example.com"),        // wab
				[]byte(""),                               // storage - empty
				[]byte("https://messagebox.example.com"), // messagebox
				[]byte("https://legal.example.com"),      // legal
				[]byte("02" + strings.Repeat("ab", 32)),  // registryOperator
			},
		},
		{
			name: "missing messagebox",
			fields: [][]byte{
				[]byte("config123"),                     // configID
				[]byte("My Wallet"),                     // name
				[]byte("https://example.com/icon.png"),  // icon
				[]byte("https://wab.example.com"),       // wab
				[]byte("https://storage.example.com"),   // storage
				[]byte(""),                              // messagebox - empty
				[]byte("https://legal.example.com"),     // legal
				[]byte("02" + strings.Repeat("ab", 32)), // registryOperator
			},
		},
		{
			name: "missing legal",
			fields: [][]byte{
				[]byte("config123"),                      // configID
				[]byte("My Wallet"),                      // name
				[]byte("https://example.com/icon.png"),   // icon
				[]byte("https://wab.example.com"),        // wab
				[]byte("https://storage.example.com"),    // storage
				[]byte("https://messagebox.example.com"), // messagebox
				[]byte(""),                               // legal - empty
				[]byte("02" + strings.Repeat("ab", 32)),  // registryOperator
			},
		},
		{
			name: "missing registryOperator",
			fields: [][]byte{
				[]byte("config123"),                      // configID
				[]byte("My Wallet"),                      // name
				[]byte("https://example.com/icon.png"),   // icon
				[]byte("https://wab.example.com"),        // wab
				[]byte("https://storage.example.com"),    // storage
				[]byte("https://messagebox.example.com"), // messagebox
				[]byte("https://legal.example.com"),      // legal
				[]byte(""),                               // registryOperator - empty
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			privateKey, err := ec.NewPrivateKey()
			require.NoError(t, err)

			lockingScript, err := createSignedWalletConfigToken(privateKey, tc.fields)
			require.NoError(t, err)

			output := &transaction.TransactionOutput{
				Satoshis:      1,
				LockingScript: lockingScript,
			}

			tx, err := createWalletConfigTransactionWithInput(t, output)
			require.NoError(t, err)

			beef, err := tx.BEEF()
			require.NoError(t, err)

			instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
			require.NoError(t, err)
			assert.Empty(t, instructions.OutputsToAdmit, "output should not be admitted with %s", tc.name)
		})
	}
}

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_ValidWalletConfigToken(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	tx, err := createValidWalletConfigTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Len(t, instructions.OutputsToAdmit, 1)
	assert.Equal(t, uint32(0), instructions.OutputsToAdmit[0])
}

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_RetainsPreviousCoins(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	tx, err := createValidWalletConfigTransaction(t)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	previousCoins := []uint32{0, 1, 2}
	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, previousCoins)
	require.NoError(t, err)
	assert.ElementsMatch(t, previousCoins, instructions.CoinsToRetain)
}

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_NoInputs(t *testing.T) {
	tm := NewWalletConfigTopicManager()

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

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_InvalidSignature(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	differentKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	// Create fields with one key but sign with a different key
	fields := [][]byte{
		[]byte("config123"),                      // configID
		[]byte("My Wallet"),                      // name
		[]byte("https://example.com/icon.png"),   // icon
		[]byte("https://wab.example.com"),        // wab
		[]byte("https://storage.example.com"),    // storage
		[]byte("https://messagebox.example.com"), // messagebox
		[]byte("https://legal.example.com"),      // legal
		[]byte(privateKey.PubKey().ToDERHex()),   // registryOperator - use one key
	}

	// Sign with a different key
	lockingScript, err := createSignedWalletConfigToken(differentKey, fields)
	require.NoError(t, err)

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	tx, err := createWalletConfigTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

func TestWalletConfigTopicManager_IdentifyAdmissibleOutputs_NonPushDropOutput(t *testing.T) {
	tm := NewWalletConfigTopicManager()

	// Create a transaction with a simple output (not PushDrop)
	s := &script.Script{}
	_ = s.AppendOpcodes(script.OpRETURN)
	_ = s.AppendPushData([]byte("not a pushdrop"))
	output := &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: s,
	}

	tx, err := createWalletConfigTransactionWithInput(t, output)
	require.NoError(t, err)

	beef, err := tx.BEEF()
	require.NoError(t, err)

	instructions, err := tm.IdentifyAdmissibleOutputs(context.Background(), beef, nil)
	require.NoError(t, err)
	assert.Empty(t, instructions.OutputsToAdmit)
}

// Helper functions

// createWalletConfigTransactionWithInput creates a transaction with a valid funding input
// and optionally adds an output
func createWalletConfigTransactionWithInput(t *testing.T, output *transaction.TransactionOutput) (*transaction.Transaction, error) {
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

func createValidWalletConfigTransaction(t *testing.T) (*transaction.Transaction, error) {
	privateKey, err := ec.NewPrivateKey()
	require.NoError(t, err)

	fields := [][]byte{
		[]byte("config123"),                      // configID
		[]byte("My Wallet"),                      // name
		[]byte("https://example.com/icon.png"),   // icon
		[]byte("https://wab.example.com"),        // wab
		[]byte("https://storage.example.com"),    // storage
		[]byte("https://messagebox.example.com"), // messagebox
		[]byte("https://legal.example.com"),      // legal
		[]byte(privateKey.PubKey().ToDERHex()),   // registryOperator
	}

	lockingScript, err := createSignedWalletConfigToken(privateKey, fields)
	if err != nil {
		return nil, err
	}

	output := &transaction.TransactionOutput{
		Satoshis:      1,
		LockingScript: lockingScript,
	}

	return createWalletConfigTransactionWithInput(t, output)
}

func createSignedWalletConfigToken(privateKey *ec.PrivateKey, fields [][]byte) (*script.Script, error) {
	// Create wallet for the registry operator
	registryWallet, err := wallet.NewProtoWallet(wallet.ProtoWalletArgs{
		Type:       wallet.ProtoWalletArgsTypePrivateKey,
		PrivateKey: privateKey,
	})
	if err != nil {
		return nil, err
	}

	// Create "anyone" wallet for deriving the locking key
	anyoneWallet, err := wallet.NewProtoWallet(wallet.ProtoWalletArgs{
		Type: wallet.ProtoWalletArgsTypeAnyone,
	})
	if err != nil {
		return nil, err
	}

	// Derive the locking public key using the "anyone" wallet with registry operator as counterparty
	publicKeyArgs := wallet.GetPublicKeyArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "wallet config option",
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

	// Concatenate fields for signing
	var data bytes.Buffer
	for _, field := range fields {
		data.Write(field)
	}

	// Sign using BRC-48 protocol with [1, 'wallet config option']
	signArgs := wallet.CreateSignatureArgs{
		EncryptionArgs: wallet.EncryptionArgs{
			ProtocolID: wallet.Protocol{
				SecurityLevel: 1,
				Protocol:      "wallet config option",
			},
			KeyID: "1",
			Counterparty: wallet.Counterparty{
				Type: wallet.CounterpartyTypeAnyone,
			},
		},
		Data: data.Bytes(),
	}

	signResult, err := registryWallet.CreateSignature(context.Background(), signArgs, "")
	if err != nil {
		return nil, err
	}

	// Add signature to fields
	allFields := append(fields, signResult.Signature.Serialize())

	// Build the PushDrop script using the derived locking key
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
