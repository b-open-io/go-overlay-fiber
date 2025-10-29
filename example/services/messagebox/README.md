# MessageBox Topic Manager and Lookup Service

An identity-based message routing protocol using BRC-48 Pay-to-Push-Drop outputs for advertising MessageBox hosts on-chain.

## Overview

The MessageBox service enables identity-based message routing in the BSV ecosystem. Users advertise their MessageBox host through cryptographically signed PushDrop outputs, proving they have authorized a specific host to receive their messages. Clients can then discover where to send messages for any given identity key.

### Components

1. **MessageBoxTopicManager** - Validates host advertisements with identity key signature verification
2. **MessageBoxLookupService** - Provides identity-to-host resolution
3. **MessageBoxStorage** - MongoDB-based storage for advertisement indexing

## Protocol Rules

Each valid output must satisfy the following:

1. It is a BRC-48 Pay-to-Push-Drop output
2. Contains exactly 3 fields:
   - **Identity Key**: Raw public key bytes (33 bytes)
   - **Host**: UTF-8 string containing the host URL
   - **Signature**: ECDSA signature over (identityKey + host)
3. The signature must verify against the identity key
4. Both identity key and host must be non-empty

## Usage

### Topic Manager

```go
import "github.com/bsv-blockchain/go-overlay-fiber/example/services/messagebox"

// Create topic manager
tm := messagebox.NewMessageBoxTopicManager()

// Use with overlay engine
overlayServer.ConfigureTopicManager("tm_messagebox", tm)
```

### Lookup Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/messagebox"
    "go.mongodb.org/mongo-driver/mongo"
)

// Create lookup service (requires MongoDB)
ls := messagebox.NewMessageBoxLookupService(mongoDatabase)

// Use with overlay engine
overlayServer.ConfigureLookupService("ls_messagebox", ls)
```

## API

### Topic Manager Methods

- `IdentifyAdmissibleOutputs(beef, previousCoins)` - Validates PushDrop outputs with signature verification
- `GetDocumentation()` - Returns topic manager documentation
- `GetMetaData()` - Returns topic manager metadata

### Lookup Service Methods

- `OutputAdmittedByTopic(payload)` - Extracts and stores identity key and host from admitted output
- `OutputSpent(payload)` - Removes spent advertisements from index
- `OutputEvicted(outpoint)` - Removes evicted advertisements from index
- `Lookup(question)` - Queries stored advertisements by identity key

### Query Parameters

```json
{
  "identityKey": "02abc...def (required - hex-encoded public key)",
  "host": "https://messagebox.example.com (optional - filter by specific host)"
}
```

## Example Lookup Queries

### Find All Hosts for Identity Key

```json
{
  "service": "ls_messagebox",
  "query": {
    "identityKey": "02abc1234567890def1234567890abc1234567890def1234567890abc1234567890"
  }
}
```

### Find Specific Host for Identity Key

```json
{
  "service": "ls_messagebox",
  "query": {
    "identityKey": "02abc1234567890def1234567890abc1234567890def1234567890abc1234567890",
    "host": "https://alice-messagebox.example.com"
  }
}
```

## Storage

The service uses MongoDB with the following schema:

```go
type MessageBoxAdvertisement struct {
    IdentityKey string    // hex-encoded public key
    Host        string    // UTF-8 host URL
    Txid        string
    OutputIndex int
    CreatedAt   time.Time
}
```

### Query Response

Returns array of UTXO references ordered by recency (newest first):

```json
[
  { "txid": "abc...", "outputIndex": 0 },
  { "txid": "def...", "outputIndex": 1 }
]
```

## Protocol Specification

**Topic Name**: `tm_messagebox`

**Lookup Service**: `ls_messagebox`

**Encoding**: BRC-48 Pay-to-Push-Drop

**Validation**:
- PushDrop structure must be valid
- Exactly 3 fields (identityKey + host + signature)
- Identity key must be valid 33-byte public key
- Host must be non-empty UTF-8 string
- Valid ECDSA signature over concat(identityKey, host)

**Query Responses**:
- UTXO references (txid + outputIndex)
- Sorted by creation time (newest first)
- Optionally filtered by host

**Spend Handling**:
- When a UTXO is spent, the advertisement is deleted
- Users can update by creating new advertisement (old remains until spent)

## Use Case Flow

1. **Advertisement Creation**:
   - Alice creates PushDrop output with:
     - Her public key (33 bytes)
     - Host URL: "https://alice-mb.example.com"
     - Signature over (pubkey + host)

2. **Validation**:
   - Topic manager decodes PushDrop
   - Verifies signature against Alice's public key
   - Admits output if valid

3. **Indexing**:
   - Lookup service stores mapping:
     - identityKey: "02abc..." → host: "https://alice-mb.example.com"

4. **Discovery**:
   - Bob wants to send Alice a message
   - Bob queries ls_messagebox with Alice's identity key
   - Gets UTXO references pointing to her host advertisements

5. **Update**:
   - Alice can advertise new host without spending old one
   - Multiple hosts can be active simultaneously

6. **Removal**:
   - Spending a UTXO removes that advertisement
   - Allows Alice to revoke specific hosts

## TypeScript Equivalent

This is a port of the MessageBox service from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/message-box).

## Related Standards

- **BRC-48**: Pay-to-Push-Drop (The encoding standard used for MessageBox advertisements)
- **BRC-22**: Overlay Services (Data synchronization protocol)
- **BRC-24**: Overlay Lookup Services
