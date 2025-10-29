# UHRP Topic Manager and Lookup Service

Universal Hash Resolution Protocol (UHRP) - A protocol for advertising file hosting availability on the blockchain.

## Overview

The UHRP service enables hosts to publish cryptographically signed advertisements committing to host specific files at specific locations, with expiry times and file sizes. This creates a decentralized content delivery network where clients can discover where files are available.

### Components

1. **UHRPTopicManager** - Validates file hosting advertisements with signature verification
2. **UHRPLookupService** - Provides file availability resolution
3. **UHRPStorage** - MongoDB-based storage for advertisement indexing

## Protocol Rules

Each valid output must satisfy the following:

1. It is a BRC-48 Pay-to-Push-Drop output
2. Contains exactly 6 fields:
   - **Host Identity Key**: 33-byte public key
   - **File Hash**: 32-byte SHA-256 hash of the file
   - **Hosted File Location**: UTF-8 HTTPS URL
   - **Expiry Time**: Varint-encoded Unix timestamp (must be >= 1)
   - **File Size**: Varint-encoded size in bytes (must be >= 1)
   - **Signature**: ECDSA signature over fields 0-4
3. The file location must be a valid HTTPS URL
4. The signature must verify against the host identity key
5. All numeric values must be valid varints

## Usage

### Topic Manager

```go
import "github.com/bsv-blockchain/go-overlay-fiber/example/services/uhrp"

// Create topic manager
tm := uhrp.NewUHRPTopicManager()

// Use with overlay engine
overlayServer.ConfigureTopicManager("tm_uhrp", tm)
```

### Lookup Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/uhrp"
    "go.mongodb.org/mongo-driver/mongo"
)

// Create lookup service (requires MongoDB)
ls := uhrp.NewUHRPLookupService(mongoDatabase)

// Use with overlay engine
overlayServer.ConfigureLookupService("ls_uhrp", ls)
```

## API

### Topic Manager Methods

- `IdentifyAdmissibleOutputs(beef, previousCoins)` - Validates PushDrop advertisements with signature verification
- `GetDocumentation()` - Returns topic manager documentation
- `GetMetaData()` - Returns topic manager metadata

### Lookup Service Methods

- `OutputAdmittedByTopic(payload)` - Extracts and stores hosting advertisement
- `OutputSpent(payload)` - Removes spent advertisements from index
- `OutputEvicted(outpoint)` - Removes evicted advertisements from index
- `Lookup(question)` - Queries stored advertisements

### Query Parameters

```json
{
  "outpoint": "txid.outputIndex (optional - exact match)",
  "uhrpUrl": "uhrp://abc123... (optional - generated from file hash)",
  "expiryTime": 1735689600 (optional - Unix timestamp)",
  "hostIdentityKey": "02abc... (optional - hex-encoded public key)",
  "fileSize": 1048576 (optional - bytes)"
}
```

**Note**: Must specify either `outpoint`, or at least one of the other fields.

## Example Lookup Queries

### Find Hosts for a File

```json
{
  "service": "ls_uhrp",
  "query": {
    "uhrpUrl": "uhrp://abc123...def"
  }
}
```

### Find All Files Hosted by Identity

```json
{
  "service": "ls_uhrp",
  "query": {
    "hostIdentityKey": "02abc1234567890def1234567890abc1234567890def1234567890abc1234567890"
  }
}
```

### Find by Outpoint

```json
{
  "service": "ls_uhrp",
  "query": {
    "outpoint": "abc123...def.0"
  }
}
```

### Find by Expiry Time

```json
{
  "service": "ls_uhrp",
  "query": {
    "expiryTime": 1735689600
  }
}
```

## Storage

The service uses MongoDB with the following schema:

```go
type UHRPRecord struct {
    Txid               string
    OutputIndex        int
    UHRPUrl            string  // Generated from file hash
    HostIdentityKey    string  // hex-encoded public key
    HostedFileLocation string  // HTTPS URL
    ExpiryTime         uint64  // Unix timestamp
    FileSize           uint64  // bytes
}
```

### UHRP URL Format

The `uhrpUrl` field is generated from the file hash using base58check encoding:
- Format: `uhrp://[base58check_encoded_hash]`
- Example: `uhrp://9z3rJNRUQDVTpV8kZdQqZ...`
- This provides a standardized, human-readable identifier for files

### Query Response

Returns array of UTXO references:

```json
[
  { "txid": "abc...", "outputIndex": 0 },
  { "txid": "def...", "outputIndex": 1 }
]
```

## Protocol Specification

**Topic Name**: `tm_uhrp`

**Lookup Service**: `ls_uhrp`

**Encoding**: BRC-48 Pay-to-Push-Drop

**Validation**:
- PushDrop structure must be valid
- Exactly 6 fields (5 data + signature)
- Field 1 (hash) must be exactly 32 bytes
- Field 2 (location) must be valid HTTPS URL
- Fields 3-4 (expiry, size) must be >= 1 and valid varints
- Valid ECDSA signature over fields 0-4

**Query Responses**:
- UTXO references (txid + outputIndex)
- Can filter by uhrpUrl, hostIdentityKey, expiryTime, fileSize, or outpoint

**Spend Handling**:
- When a UTXO is spent, the advertisement is deleted
- Hosts can update by creating new advertisement (old remains until spent)

## Use Case Flow

1. **Advertisement Creation**:
   - Alice creates PushDrop output advertising file hosting:
     - Her public key (33 bytes)
     - File hash (32 bytes)
     - HTTPS location: "https://alice-cdn.example.com/files/"
     - Expiry: 1735689600 (Jan 1, 2025)
     - Size: 1048576 (1MB)
     - Signature over all above data

2. **Validation**:
   - Topic manager decodes PushDrop
   - Verifies hash is 32 bytes
   - Verifies location is HTTPS
   - Verifies expiry/size are valid
   - Verifies signature against Alice's public key
   - Admits output if all checks pass

3. **Indexing**:
   - Lookup service generates UHRP URL from file hash
   - Stores mapping:
     - uhrpUrl: "uhrp://abc123..." (from hash)
     - hostIdentityKey: "02abc..."
     - hostedFileLocation: "https://alice-cdn.example.com/files/"
     - expiryTime: 1735689600
     - fileSize: 1048576
     - txid/outputIndex

4. **Discovery**:
   - Bob wants to download file with hash abc123...
   - Bob queries ls_uhrp with uhrpUrl
   - Gets UTXO references pointing to Alice's hosting advertisements
   - Bob retrieves full outputs to get hosting locations

5. **Update**:
   - Alice can advertise updated availability without spending old one
   - Multiple hosts can advertise same file

6. **Removal**:
   - Spending a UTXO removes that advertisement
   - Allows hosts to revoke specific hosting commitments

## TypeScript Equivalent

This is a port of the UHRP service from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/uhrp).

## Related Standards

- **BRC-48**: Pay-to-Push-Drop (The encoding standard used for UHRP advertisements)
- **BRC-22**: Overlay Services (Data synchronization protocol)
- **BRC-24**: Overlay Lookup Services
