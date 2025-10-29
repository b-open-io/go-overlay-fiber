# SlackThreads Topic Manager and Lookup Service

A simple protocol for storing 32-byte hashes on-chain using a direct script pattern validation approach.

## Overview

The SlackThreads service allows users to push 32-byte hashes to the BSV blockchain using a simple OP_SHA256 locking script pattern. Unlike PushDrop-based protocols, SlackThreads uses direct script validation for maximum simplicity.

### Components

1. **SlackThreadsTopicManager** - Validates outputs matching the OP_SHA256 pattern
2. **SlackThreadsLookupService** - Provides querying of thread hashes
3. **SlackThreadsStorage** - MongoDB-based storage with thread hash indexing

## Protocol Rules

Each valid output must have a locking script matching this exact pattern:

```asm
OP_SHA256 <32-byte hash> OP_EQUAL
```

This means:
1. Exactly 3 script chunks
2. Chunk[0]: OP_SHA256 opcode (0xA8)
3. Chunk[1]: 32-byte data push
4. Chunk[2]: OP_EQUAL opcode (0x87)

## Key Differences

**NOT PushDrop Encoding**: Unlike other overlay protocols, SlackThreads does NOT use BRC-48 PushDrop encoding. The hash is embedded directly in the locking script as raw bytes.

## Usage

### Topic Manager

```go
import "github.com/bsv-blockchain/go-overlay-fiber/example/services/slackthreads"

// Create topic manager
tm := slackthreads.NewSlackThreadsTopicManager()

// Use with overlay engine
overlayServer.ConfigureTopicManager("tm_slackthread", tm)
```

### Lookup Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/slackthreads"
    "go.mongodb.org/mongo-driver/mongo"
)

// Create lookup service (requires MongoDB)
ls := slackthreads.NewSlackThreadLookupService(mongoDatabase)

// Use with overlay engine
overlayServer.ConfigureLookupService("ls_slackthread", ls)
```

## API

### Topic Manager Methods

- `IdentifyAdmissibleOutputs(beef, previousCoins)` - Validates script pattern
- `GetDocumentation()` - Returns topic manager documentation
- `GetMetaData()` - Returns topic manager metadata

### Lookup Service Methods

- `OutputAdmittedByTopic(payload)` - Extracts and stores hash from admitted output
- `OutputSpent(payload)` - Removes spent outputs from index
- `OutputEvicted(outpoint)` - Removes evicted outputs from index
- `Lookup(question)` - Queries stored thread hashes

### Query Parameters

```json
{
  "threadHash": "64-character hex string (optional - exact match)",
  "txid": "transaction id (optional)",
  "limit": 50,
  "skip": 0,
  "startDate": "2024-01-01T00:00:00Z (optional)",
  "endDate": "2024-12-31T23:59:59Z (optional)",
  "sortOrder": "desc"
}
```

## Example Lookup Queries

### Find by Thread Hash

```json
{
  "service": "ls_slackthread",
  "query": {
    "threadHash": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "limit": 10
  }
}
```

### Find by Transaction ID

```json
{
  "service": "ls_slackthread",
  "query": {
    "txid": "a1b2c3d4e5f6...",
    "limit": 10
  }
}
```

### List All Thread Hashes

```json
{
  "service": "ls_slackthread",
  "query": {
    "limit": 20,
    "skip": 0,
    "sortOrder": "desc"
  }
}
```

### Filter by Date Range

```json
{
  "service": "ls_slackthread",
  "query": {
    "startDate": "2024-01-01T00:00:00Z",
    "endDate": "2024-12-31T23:59:59Z",
    "limit": 50
  }
}
```

## Storage

The service uses MongoDB with the following schema:

```go
type SlackThreadRecord struct {
    Txid        string
    OutputIndex int
    ThreadHash  string    // 64-character hex string
    CreatedAt   time.Time
}
```

### Indexes

- `threadHash` - Regular index for exact hash matching (not full-text)

## Hash Format

The 32-byte hash from the script is stored as a 64-character hexadecimal string:

- **On-chain**: 32 bytes (e.g., `[0x12, 0x34, ..., 0xef]`)
- **In MongoDB**: "1234...ef" (64 hex characters)
- **Queries**: Must provide exact 64-character hex string

## Protocol Specification

**Topic Name**: `tm_slackthread`

**Lookup Service**: `ls_slackthread`

**Encoding**: Direct script pattern (NOT PushDrop)

**Validation**:
- Script must have exactly 3 chunks
- Chunk[0] = OP_SHA256
- Chunk[1] = 32-byte data push
- Chunk[2] = OP_EQUAL

**Query Responses**:
- Only UTXO references returned (txid + outputIndex)
- Hash and creation date NOT returned to clients

**Spend Handling**:
- When a UTXO is spent, the record is deleted
- The hash is no longer queryable

## TypeScript Equivalent

This is a port of the SlackThreads service from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples).

## Related Standards

- **BRC-22**: Overlay Services (Data synchronization protocol)
- **BRC-24**: Overlay Lookup Services
