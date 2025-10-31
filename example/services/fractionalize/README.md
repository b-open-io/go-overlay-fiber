# Fractionalize Example Service

A proof-of-concept protocol for fractionalized ownership using BSV-20 style ordinal tokens with specialized locking mechanisms.

## Overview

The Fractionalize service implements a token protocol that validates three distinct types of outputs for fractionalized ownership. It uses BSV-20 inscription patterns combined with different locking script mechanisms to enable server-managed tokens, user transfers, and payment outputs.

**Topic Manager**: `tm_fractionalize`
**Lookup Service**: `ls_fractionalize`
**Complexity**: Medium-High

## Protocol Rules

### Output Types

The Fractionalize protocol validates three distinct types of outputs based on their locking script patterns:

#### 1. Server Token (Ordinal + MultiSig)
- **Pattern**: Contains both `OP_IF` (ordinal inscription) and `OP_CHECKMULTISIG`
- **Purpose**: Token mint or server change output
- **Locking Mechanism**: 1-of-2 multisig with ordinal inscription
- **Use Case**: Server maintains control of token supply

#### 2. Transfer Token (Ordinal only)
- **Pattern**: Contains `OP_IF` but not `OP_CHECKMULTISIG`
- **Purpose**: Token transfer to end user
- **Locking Mechanism**: Standard P2PKH with ordinal inscription
- **Use Case**: User receives fractional ownership token

#### 3. Payment (MultiSig only)
- **Pattern**: Contains `OP_CHECKMULTISIG` but not `OP_IF`
- **Purpose**: Payment output (no token content)
- **Locking Mechanism**: 1-of-2 multisig
- **Use Case**: Payment for token transactions

### Admissibility Rules

The topic manager validates that:
1. Output locking script matches one of the three expected patterns
2. Script structure conforms to the hardcoded template for that type
3. Variable data (hashes) can differ but structure must match exactly

Outputs that don't match any of the three templates are ignored and **not** admitted.

### Script Structure

Each output type has a specific script structure:

#### Server Token Script
```
OP_0 OP_IF
  "ord" OP_1 "application/bsv-20" OP_0 <json_inscription>
OP_ENDIF
OP_2DUP OP_CAT OP_HASH160 <hash160> OP_EQUALVERIFY
OP_TOALTSTACK OP_TOALTSTACK OP_1 OP_FROMALTSTACK OP_FROMALTSTACK OP_2
OP_CHECKMULTISIG
OP_RETURN <token_txid>
```

#### Transfer Token Script
```
OP_0 OP_IF
  "ord" OP_1 "application/bsv-20" OP_0 <json_inscription>
OP_ENDIF
OP_DUP OP_HASH160 <pubkey_hash> OP_EQUALVERIFY OP_CHECKSIG
OP_RETURN <token_txid>
```

#### Payment Script
```
OP_2DUP OP_CAT OP_HASH160 <hash160> OP_EQUALVERIFY
OP_TOALTSTACK OP_TOALTSTACK OP_1 OP_FROMALTSTACK OP_FROMALTSTACK OP_2
OP_CHECKMULTISIG
```

### BSV-20 Inscription Format

Token outputs include BSV-20 inscription metadata:
```json
{
  "p": "bsv-20",
  "op": "mint",
  "amt": "1"
}
```

## Query Types

The lookup service supports simple query patterns:

### 1. Txid Lookup
```go
Query: {
    "txid": "transaction_id"
}
```
Returns the specific UTXO for the given transaction ID.

### 2. Browse All
```go
Query: {
    "limit": 50,
    "skip": 0,
    "sortOrder": "desc"
}
```
Returns all UTXOs with pagination and sorting.

### 3. Date Range Query
```go
Query: {
    "startDate": "2025-01-01T00:00:00Z",
    "endDate": "2025-12-31T23:59:59Z",
    "limit": 100
}
```
Returns UTXOs created within the specified date range.

## Pagination and Sorting

All queries support:
- **limit**: Maximum results to return (default: 50)
- **skip**: Number of results to skip for pagination (default: 0)
- **sortOrder**: Sort direction - 'asc' or 'desc' (default: 'desc')

Results are sorted by `createdAt` timestamp.

## Storage Schema

### MongoDB Collection: `fractionalizeRecords`

```go
type FractionalizeRecord struct {
    Txid         string    `bson:"txid"`
    OutputIndex  int       `bson:"outputIndex"`
    SpendingTxid string    `bson:"spendingTxid,omitempty"`
    CreatedAt    time.Time `bson:"createdAt"`
}
```

### Indexes
- Index on `txid` for efficient lookups

## Use Cases

### Token Minting
1. Server creates token with BSV-20 inscription
2. Output uses server token pattern (ordinal + multisig)
3. Topic manager validates script structure
4. Lookup service indexes the UTXO

### Token Transfer
1. User receives token via transfer token pattern
2. Output uses P2PKH with ordinal inscription
3. User can verify ownership and token content
4. Lookup service tracks the transfer

### Token Spending
1. Token UTXO is spent in new transaction
2. Lookup service records spending txid
3. Historical record maintained for auditing
4. New outputs admitted if valid

### Server Change Management
1. Server maintains token supply via server token outputs
2. Multisig control enables secure key management
3. Ordinal inscription preserves token lineage
4. Change outputs remain under server control

## API Methods

### Topic Manager (tm_fractionalize)

```go
func (tm *FractionalizeTopicManager) IdentifyAdmissibleOutputs(
    ctx context.Context,
    beef []byte,
    previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error)
```

Validates output scripts against the three templates and determines which outputs should be admitted to the overlay network.

### Lookup Service (ls_fractionalize)

```go
func (ls *FractionalizeLookupService) Lookup(
    ctx context.Context,
    question *lookup.LookupQuestion,
) (*lookup.LookupAnswer, error)
```

Queries fractionalize records based on txid or date range parameters.

## Example Usage

```go
import (
    "context"
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/fractionalize"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
    "go.mongodb.org/mongo-driver/mongo"
)

func main() {
    // Create MongoDB client
    mongoClient, _ := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    db := mongoClient.Database("overlay")

    // Create overlay server
    overlayServer := server.NewOverlayServer("mynode", privateKey, "https://myhost.com")

    // Configure Fractionalize topic manager
    tm := fractionalize.NewFractionalizeTopicManager()
    overlayServer.ConfigureTopicManager("tm_fractionalize", tm)

    // Configure Fractionalize lookup service
    ls := fractionalize.NewFractionalizeLookupService(db)
    overlayServer.ConfigureLookupService("ls_fractionalize", ls)

    // Start server
    overlayServer.Start()
}
```

### Querying with LookupResolver

```go
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by txid
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_fractionalize",
    Query: map[string]interface{}{
        "txid": "transaction_id",
    },
}, 10000)

// Find all with date range
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_fractionalize",
    Query: map[string]interface{}{
        "startDate": "2025-01-01T00:00:00Z",
        "endDate": "2025-12-31T23:59:59Z",
        "limit": 100,
        "sortOrder": "desc",
    },
}, 10000)
```

## Security Considerations

### Script Template Validation
- Hardcoded templates prevent script injection
- Only specific patterns are admitted
- Variable data (hashes) allowed but structure fixed

### Server Key Management
- Multisig provides additional security for server tokens
- Requires coordination of multiple keys for spending
- Protects token supply from single key compromise

### Token Lineage
- OP_RETURN includes token txid for provenance
- Enables tracking of token ancestry
- Supports audit and verification

### Spending Tracking
- Lookup service records spending transactions
- Historical record maintained
- Enables double-spend detection at application layer

## Limitations

### Proof-of-Concept Status
This is a demonstration protocol showing one approach to fractionalized ownership. It is not production-ready and should be considered experimental.

### Hardcoded Templates
Script validation uses hardcoded templates which limits flexibility:
- New patterns require code changes
- Cannot adapt to protocol evolution without updates
- Template maintenance required

### Limited Inscription Validation
- Does not validate BSV-20 inscription content
- Application must verify token amounts and operations
- No built-in token accounting

## Future Enhancements

Potential improvements for production use:

1. **Dynamic Template Configuration**: Support configurable script patterns
2. **Inscription Validation**: Parse and validate BSV-20 content
3. **Token Accounting**: Track token balances and supply
4. **Multi-Token Support**: Handle different token types
5. **Enhanced Query Capabilities**: Search by token properties

## TypeScript Equivalent

This service is a port of the Fractionalize example from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/fractionalize).

## References

- [BSV-20 Protocol](https://docs.1satordinals.com/bsv20)
- [Bitcoin Script](https://wiki.bitcoinsv.io/index.php/Script)
- [Ordinal Theory](https://docs.1satordinals.com/)
