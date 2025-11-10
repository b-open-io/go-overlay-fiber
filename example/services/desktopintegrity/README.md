# DesktopIntegrity Example Service

A protocol for desktop application integrity verification using 32-byte file hashes stored on-chain with off-chain metadata.

## Overview

The DesktopIntegrity service implements a simple file integrity tracking system for desktop applications. It stores 32-byte hashes on-chain in OP_RETURN outputs while maintaining off-chain metadata for additional context.

**Topic Manager**: `tm_desktopintegrity`
**Lookup Service**: `ls_desktopintegrity`
**Complexity**: Very Low

## Protocol Rules

### DesktopIntegrity Output Structure

DesktopIntegrity outputs use a simple OP_RETURN pattern:
- **OP_FALSE**: Indicates unspendable output
- **OP_RETURN**: Followed by exactly 32 bytes of file hash data

**Locking Script Pattern**:
```
OP_FALSE OP_RETURN <32 byte hash>
```

### Admissibility Rules

The topic manager validates that:
1. Locking script has exactly 2 chunks
2. Chunk 0 is OP_FALSE
3. Chunk 1 is OP_RETURN with 32-byte hash data

Outputs failing validation are ignored and **not** admitted.

## Query Types

The lookup service supports multiple query patterns:

### 1. File Hash Lookup
```go
Query: {
    "fileHash": "abc123...",
    "limit": 50,
    "skip": 0,
    "sortOrder": "desc"
}
```
Returns all records for the specified file hash (hex-encoded 32 bytes).

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
- **limit**: Maximum results to return (default: 50)
- **skip**: Number of results to skip for pagination (default: 0)
- **sortOrder**: Sort direction - 'asc' or 'desc' (default: 'desc')

Results are sorted by `createdAt` timestamp.

## Storage Schema

### MongoDB Collection: `desktopIntegrityRecords`

```go
type DesktopIntegrityRecord struct {
    Txid           string    `bson:"txid"`
    OutputIndex    int       `bson:"outputIndex"`
    FileHash       string    `bson:"fileHash"`
    OffChainValues []byte    `bson:"offChainValues"`
    CreatedAt      time.Time `bson:"createdAt"`
}
```

### Indexes
- Index on `fileHash` for efficient lookups

## Use Cases

### Desktop Application Integrity
1. Hash desktop application files
2. Create on-chain proof with OP_RETURN
3. Store metadata off-chain (version, signature, etc.)
4. Query by file hash to verify integrity
5. Detect tampering or unauthorized modifications

### Software Distribution
1. Publisher hashes software releases
2. Creates on-chain record per release
3. Off-chain values store version, platform, etc.
4. Users query by hash to verify authenticity
5. Audit trail maintained on blockchain

### Update Verification
1. Hash each software update
2. Create sequential on-chain records
3. Track update history via txid queries
4. Verify update integrity before installation
5. Roll back to previous verified versions

### Code Signing Alternative
1. Hash signed executables
2. Store signing metadata off-chain
3. Create verifiable on-chain proof
4. Query to validate code signatures
5. Transparent signature verification

## API Methods

### Topic Manager (tm_desktopintegrity)

```go
func (tm *DesktopIntegrityTopicManager) IdentifyAdmissibleOutputs(
    ctx context.Context,
    beef []byte,
    previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error)
```

Validates OP_FALSE OP_RETURN pattern and determines which outputs should be admitted to the overlay network.

### Lookup Service (ls_desktopintegrity)

```go
func (ls *DesktopIntegrityLookupService) Lookup(
    ctx context.Context,
    question *lookup.LookupQuestion,
) (*lookup.LookupAnswer, error)
```

Queries desktop integrity records based on fileHash, txid, or date range parameters.

## Example Usage

```go
import (
    "context"
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/desktopintegrity"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
    "go.mongodb.org/mongo-driver/mongo"
)

func main() {
    // Create MongoDB client
    mongoClient, _ := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    db := mongoClient.Database("overlay")

    // Create overlay server
    overlayServer := server.NewOverlayServer("mynode", privateKey, "https://myhost.com")

    // Configure DesktopIntegrity topic manager
    tm := desktopintegrity.NewDesktopIntegrityTopicManager()
    overlayServer.ConfigureTopicManager("tm_desktopintegrity", tm)

    // Configure DesktopIntegrity lookup service
    ls := desktopintegrity.NewDesktopIntegrityLookupService(db)
    overlayServer.ConfigureLookupService("ls_desktopintegrity", ls)

    // Start server
    overlayServer.Start()
}
```

### Querying with LookupResolver

```go
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by file hash
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_desktopintegrity",
    Query: map[string]interface{}{
        "fileHash": "abc123def456...",
        "limit": 10,
    },
}, 10000)

// Find by txid
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_desktopintegrity",
    Query: map[string]interface{}{
        "txid": "transaction_id",
    },
}, 10000)

// Find all with date range
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_desktopintegrity",
    Query: map[string]interface{}{
        "startDate": "2025-01-01T00:00:00Z",
        "endDate": "2025-12-31T23:59:59Z",
        "limit": 100,
        "sortOrder": "desc",
    },
}, 10000)
```

### Creating a DesktopIntegrity Record

```go
import (
    "crypto/sha256"
    "github.com/bsv-blockchain/go-sdk/script"
)

// Hash a file
fileData := []byte("application binary data...")
hash := sha256.Sum256(fileData)

// Create locking script with OP_FALSE OP_RETURN pattern
lockingScript := &script.Script{}
lockingScript.AppendOpcodes(script.OpFALSE)    // OP_FALSE
lockingScript.AppendOpcodes(script.OpRETURN)   // OP_RETURN

// Append 32-byte hash with length prefix
hashData := make([]byte, 33)
hashData[0] = 32  // Length byte
copy(hashData[1:], hash[:])
lockingScript.AppendPushData(hashData)

// Add output to transaction
// Submit with off-chain values as needed (version info, signatures, etc.)
```

## Security Considerations

### Hash Collision
- SHA-256 provides strong collision resistance
- 32-byte hashes are sufficient for file integrity
- Consider additional metadata for disambiguation
- Multiple records can have same hash (valid use case)

### Off-Chain Data
- Off-chain values not cryptographically bound to hash
- Overlay network must preserve off-chain metadata
- Consider redundancy for critical metadata
- Application layer should verify off-chain data

### Data Privacy
- File hashes are public on blockchain
- Off-chain values stored in overlay database
- Not encrypted by default
- Consider encryption for sensitive metadata
- Access control at application layer

### Integrity Verification
- On-chain hash provides tamper evidence
- Cannot prove file existed before hash
- Timestamp via blockchain provides ordering
- Trust overlay operator for off-chain data

## Limitations

### Simple Hash Storage
This protocol provides basic hash storage:
- No merkle trees or proof structures
- No built-in versioning or lineage tracking
- Single hash per output (no multi-file support)
- Limited to 32-byte hashes (SHA-256)

### Off-Chain Dependency
- Relies on overlay network for metadata
- Risk of metadata loss if overlay node fails
- No on-chain binding of off-chain values
- Consider redundant storage for critical data

### No Signature Verification
- No built-in cryptographic signatures
- Trust model depends on who creates records
- Applications must implement additional verification
- Consider combining with BRC-48 for signatures

## Future Enhancements

Potential improvements for production use:

1. **Merkle Tree Support**: Hash multiple files in single output
2. **Version Tracking**: Link updates in chain-of-custody
3. **Signature Integration**: Add BRC-48 signature verification
4. **Multi-Hash Support**: Support multiple hash algorithms
5. **Metadata Binding**: Cryptographically bind off-chain values
6. **Expiry Mechanism**: Time-limited integrity proofs

## TypeScript Equivalent

This service is a port of the DesktopIntegrity example from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/desktopintegrity).

## References

- [OP_RETURN](https://wiki.bitcoinsv.io/index.php/OP_RETURN)
- [Bitcoin Script](https://wiki.bitcoinsv.io/index.php/Script)
- [SHA-256](https://en.wikipedia.org/wiki/SHA-2)
- [Code Signing](https://en.wikipedia.org/wiki/Code_signing)
