# DID Topic Manager and Lookup Service

Decentralized Identifiers (DID) - A protocol for storing DID serial numbers on-chain.

## Overview

The DID service enables storing and resolving decentralized identifier serial numbers on the blockchain using cryptographically signed PushDrop outputs. This creates a simple, on-chain registry for DID resolution.

### Components

1. **DIDTopicManager** - Validates DID serial number outputs
2. **DIDLookupService** - Provides DID resolution by serial number
3. **DIDStorage** - MongoDB-based storage for DID indexing

## Protocol Rules

Each valid output must satisfy the following:

1. It is a BRC-48 Pay-to-Push-Drop output
2. Contains exactly 2 fields:
   - **Serial Number**: UTF-8 encoded string (base64)
   - **Signature**: ECDSA signature field
3. The serial number must be a valid non-empty string

**Note**: The current implementation does not verify signatures in the topic manager. Additional validation could be added to:
- Link to a certifier
- Require serial numbers to be exactly 32 bytes

## Usage

### Topic Manager

```go
import "github.com/bsv-blockchain/go-overlay-fiber/example/services/did"

// Create topic manager
tm := did.NewDIDTopicManager()

// Use with overlay engine
overlayServer.ConfigureTopicManager("tm_did", tm)
```

### Lookup Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/did"
    "go.mongodb.org/mongo-driver/mongo"
)

// Create lookup service (requires MongoDB)
ls := did.NewDIDLookupService(mongoDatabase)

// Use with overlay engine
overlayServer.ConfigureLookupService("ls_did", ls)
```

## API

### Topic Manager Methods

- `IdentifyAdmissibleOutputs(beef, previousCoins)` - Validates PushDrop DID tokens
- `GetDocumentation()` - Returns topic manager documentation
- `GetMetaData()` - Returns topic manager metadata

### Lookup Service Methods

- `OutputAdmittedByTopic(payload)` - Extracts and stores DID serial number
- `OutputSpent(payload)` - Removes spent DID records from index
- `OutputEvicted(outpoint)` - Removes evicted DID records from index
- `Lookup(question)` - Queries stored DID records

### Query Parameters

```json
{
  "serialNumber": "abc123... (optional - base64 encoded string)",
  "outpoint": "txid.outputIndex (optional - exact match)"
}
```

**Note**: Must specify either `serialNumber` OR `outpoint`.

## Example Lookup Queries

### Find by Serial Number

```json
{
  "service": "ls_did",
  "query": {
    "serialNumber": "abc123def456..."
  }
}
```

### Find by Outpoint

```json
{
  "service": "ls_did",
  "query": {
    "outpoint": "abc123...def.0"
  }
}
```

## Storage

The service uses MongoDB with the following schema:

```go
type DIDRecord struct {
    Txid         string
    OutputIndex  int
    SerialNumber string    // UTF-8 encoded string
    CreatedAt    time.Time
}
```

### Query Response

Returns array of UTXO references:

```json
[
  { "txid": "abc...", "outputIndex": 0 },
  { "txid": "def...", "outputIndex": 1 }
]
```

## Protocol Specification

**Topic Name**: `tm_did`

**Lookup Service**: `ls_did`

**Encoding**: BRC-48 Pay-to-Push-Drop

**Validation**:
- PushDrop structure must be valid
- Exactly 2 fields (serial number + signature)
- Serial number must be non-empty UTF-8 string
- No signature verification (in current implementation)

**Query Responses**:
- UTXO references (txid + outputIndex)
- Can filter by serialNumber or outpoint

**Spend Handling**:
- When a UTXO is spent, the DID record is deleted
- Updates can be made by creating new DID outputs

## Use Case Flow

1. **DID Creation**:
   - Alice creates PushDrop output with DID serial number:
     - Serial number: "abc123..." (UTF-8 string)
     - Signature field

2. **Validation**:
   - Topic manager decodes PushDrop
   - Verifies exactly 2 fields present
   - Verifies serial number is non-empty
   - Admits output if all checks pass

3. **Indexing**:
   - Lookup service extracts serial number
   - Stores mapping:
     - serialNumber: "abc123..."
     - txid/outputIndex
     - createdAt timestamp

4. **Resolution**:
   - Bob wants to resolve DID with serial number "abc123..."
   - Bob queries ls_did with serialNumber
   - Gets UTXO references pointing to Alice's DID outputs
   - Bob retrieves full outputs to get complete DID information

5. **Update**:
   - Alice can create new DID outputs without spending old ones
   - Multiple outputs can reference the same serial number

6. **Removal**:
   - Spending a UTXO removes that DID record
   - Allows DID owners to revoke specific identifiers

## TypeScript Equivalent

This is a port of the DID service from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/did).

## Related Standards

- **BRC-48**: Pay-to-Push-Drop (The encoding standard used for DID outputs)
- **BRC-22**: Overlay Services (Data synchronization protocol)
- **BRC-24**: Overlay Lookup Services
