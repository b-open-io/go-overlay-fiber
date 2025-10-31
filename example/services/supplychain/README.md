# SupplyChain Example Service

A protocol for supply chain tracking using simple PushDrop-like scripts with off-chain metadata storage.

## Overview

The SupplyChain service implements a file and integrity tracking system using a simplified PushDrop pattern combined with off-chain values. It enables supply chain tracking by storing metadata off-chain while anchoring proofs on-chain.

**Topic Manager**: `tm_supplychain`
**Lookup Service**: `ls_supplychain`
**Complexity**: Low-Medium

## Protocol Rules

### SupplyChain Output Structure

SupplyChain outputs use a PushDrop-like format with exactly 5 script chunks:
- **Chunk 0**: PushDrop metadata (data push)
- **Chunk 1**: Additional data (data push)
- **Chunk 2**: OP_2DROP
- **Chunk 3**: 33-byte public key
- **Chunk 4**: OP_CHECKSIG

### Off-Chain Values

The protocol relies on off-chain values (provided separately from the locking script) that contain supply chain metadata:
- **chainId** (required): Supply chain identifier
- Additional custom fields as needed for tracking

The off-chain values are stored as JSON and must be provided when submitting the transaction.

### Admissibility Rules

The topic manager validates that:
1. Locking script has exactly 5 chunks
2. Chunk 0 is a data push (metadata)
3. Chunk 1 is a data push (additional data)
4. Chunk 2 is OP_2DROP
5. Chunk 3 is a 33-byte public key
6. Chunk 4 is OP_CHECKSIG
7. Off-chain values contain a valid chainId

Outputs failing validation are ignored and **not** admitted.

## Query Types

The lookup service supports multiple query patterns:

### 1. ChainID Lookup
```go
Query: {
    "chainId": "supply-chain-id",
    "limit": 8,
    "skip": 0
}
```
Returns all records for the specified supply chain ID (default limit: 8).

### 2. Txid Lookup
```go
Query: {
    "txid": "transaction_id",
    "limit": 50,
    "skip": 0,
    "sortOrder": "desc"
}
```
Returns all records for the specified transaction ID.

### 3. Browse All
```go
Query: {
    "limit": 50,
    "skip": 0,
    "sortOrder": "desc"
}
```
Returns all records with pagination and sorting.

### 4. Date Range Query
```go
Query: {
    "startDate": "2025-01-01T00:00:00Z",
    "endDate": "2025-12-31T23:59:59Z",
    "limit": 100
}
```
Returns records created within the specified date range.

## Pagination and Sorting

All queries support:
- **limit**: Maximum results to return (default: 50 for txid/all, 8 for chainId)
- **skip**: Number of results to skip for pagination (default: 0)
- **sortOrder**: Sort direction - 'asc' or 'desc' (default: 'desc')

Results are sorted by `createdAt` timestamp.

## Storage Schema

### MongoDB Collection: `supplyChainRecords`

```go
type SupplyChainRecord struct {
    Txid           string                 `bson:"txid"`
    OutputIndex    int                    `bson:"outputIndex"`
    OffChainValues map[string]interface{} `bson:"offChainValues"`
    SpendingTxid   string                 `bson:"spendingTxid,omitempty"`
    CreatedAt      time.Time              `bson:"createdAt"`
}
```

### Indexes
- Index on `offChainValues.chainId` for efficient lookups

## Use Cases

### Supply Chain Tracking
1. Create transaction with PushDrop-like output
2. Provide off-chain values with chainId and metadata
3. Topic manager validates script structure
4. Lookup service indexes by chainId
5. Query to track items in supply chain

### File Integrity Tracking
1. Hash file and store in off-chain values
2. Create on-chain proof with chainId
3. Track file lineage through chain
4. Verify integrity at any point

### Multi-Item Tracking
1. Use chainId to group related items
2. Each transaction adds to the chain
3. Query by chainId to see complete history
4. Track spending to detect state changes

### Document Provenance
1. Store document metadata off-chain
2. Create on-chain anchors
3. Track document lifecycle via chainId
4. Audit trail maintained on blockchain

## API Methods

### Topic Manager (tm_supplychain)

```go
func (tm *SupplyChainTopicManager) IdentifyAdmissibleOutputs(
    ctx context.Context,
    beef []byte,
    previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error)
```

Validates script structure and determines which outputs should be admitted to the overlay network.

### Lookup Service (ls_supplychain)

```go
func (ls *SupplyChainLookupService) Lookup(
    ctx context.Context,
    question *lookup.LookupQuestion,
) (*lookup.LookupAnswer, error)
```

Queries supply chain records based on chainId, txid, or date range parameters.

## Example Usage

```go
import (
    "context"
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/supplychain"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
    "go.mongodb.org/mongo-driver/mongo"
)

func main() {
    // Create MongoDB client
    mongoClient, _ := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    db := mongoClient.Database("overlay")

    // Create overlay server
    overlayServer := server.NewOverlayServer("mynode", privateKey, "https://myhost.com")

    // Configure SupplyChain topic manager
    tm := supplychain.NewSupplyChainTopicManager()
    overlayServer.ConfigureTopicManager("tm_supplychain", tm)

    // Configure SupplyChain lookup service
    ls := supplychain.NewSupplyChainLookupService(db)
    overlayServer.ConfigureLookupService("ls_supplychain", ls)

    // Start server
    overlayServer.Start()
}
```

### Querying with LookupResolver

```go
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by chainId
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_supplychain",
    Query: map[string]interface{}{
        "chainId": "my-supply-chain",
        "limit": 10,
    },
}, 10000)

// Find by txid
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_supplychain",
    Query: map[string]interface{}{
        "txid": "transaction_id",
    },
}, 10000)

// Find all with date range
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_supplychain",
    Query: map[string]interface{}{
        "startDate": "2025-01-01T00:00:00Z",
        "endDate": "2025-12-31T23:59:59Z",
        "limit": 100,
        "sortOrder": "desc",
    },
}, 10000)
```

### Creating a SupplyChain Record

```go
import (
    "encoding/json"
    "github.com/bsv-blockchain/go-sdk/script"
)

// Prepare off-chain values
offChainValues := map[string]interface{}{
    "chainId": "my-supply-chain",
    "itemName": "Widget A",
    "timestamp": "2025-10-31T12:00:00Z",
    "location": "Warehouse 1",
    "hash": "abc123...",
}

// Create locking script with PushDrop-like pattern
lockingScript := &script.Script{}
lockingScript.AppendPushData([]byte("metadata")) // Chunk 0
lockingScript.AppendPushData([]byte("data"))     // Chunk 1
lockingScript.AppendOpcodes(script.Op2DROP)      // Chunk 2
lockingScript.AppendPushData(publicKey)          // Chunk 3 (33 bytes)
lockingScript.AppendOpcodes(script.OpCHECKSIG)   // Chunk 4

// Add output to transaction
// Submit with off-chain values as JSON
```

## Security Considerations

### Off-Chain Data Integrity
- Off-chain values are not part of the blockchain
- Relies on overlay network to propagate and store
- Verify data integrity independently
- Consider redundancy for critical data

### ChainID Uniqueness
- ChainID should be unique for each supply chain
- No built-in collision detection
- Applications must manage chainId namespaces
- Consider using hash-based identifiers

### Script Validation
- Simple pattern matching only
- No cryptographic signature verification
- Relies on standard P2PK for spending
- Additional validation needed at application layer

### Data Privacy
- Off-chain values stored in overlay database
- Not encrypted by default
- Consider encryption for sensitive data
- Access control at application layer

## Limitations

### Off-Chain Dependency
This protocol relies heavily on off-chain data:
- ChainId and metadata not on blockchain
- Overlay network must preserve off-chain values
- Risk of data loss if overlay node fails
- Not suitable for applications requiring full on-chain verification

### No Built-in Verification
- No signature verification on off-chain data
- No merkle proofs or commitments
- Trust overlay operator for data integrity
- Applications must implement additional verification

### Simple Pattern
- Basic script pattern only
- No complex validation rules
- Limited to specific use cases
- Consider more robust protocols for critical applications

## Future Enhancements

Potential improvements for production use:

1. **Merkle Commitments**: Hash off-chain data into on-chain commitments
2. **Signature Verification**: Sign off-chain values for integrity
3. **Schema Validation**: Enforce structure on off-chain values
4. **Data Encryption**: Encrypt sensitive supply chain data
5. **Multi-Chain Support**: Track items across multiple chains
6. **Enhanced Query**: Full-text search on off-chain values

## TypeScript Equivalent

This service is a port of the SupplyChain example from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/supplychain).

## References

- [Bitcoin Script](https://wiki.bitcoinsv.io/index.php/Script)
- [PushDrop Protocol](https://github.com/bsv-blockchain/go-sdk/tree/master/transaction/template/pushdrop)
- [Supply Chain on Blockchain](https://en.wikipedia.org/wiki/Blockchain#Supply_chain)
