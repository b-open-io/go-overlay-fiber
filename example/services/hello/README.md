# HelloWorld Topic Manager and Lookup Service

A simple messaging protocol using BRC-48 Pay-to-Push-Drop outputs for broadcasting UTF-8 messages on-chain.

## Overview

The HelloWorld service allows users to broadcast messages on the BSV blockchain using the BRC-48 PushDrop standard. Each message is validated for structure and signature before being admitted to the overlay.

### Components

1. **HelloWorldTopicManager** - Validates BRC-48 PushDrop outputs containing messages
2. **HelloWorldLookupService** - Provides full-text search and querying of messages
3. **HelloWorldStorage** - MongoDB-based storage with full-text indexing

## Protocol Rules

Each valid output must satisfy the following:

1. It is a BRC-48 Pay-to-Push-Drop output
2. The drop contains exactly one field - the UTF-8 message
3. The message is at least two characters long
4. The signature inside the drop must verify against the locking public key over the concatenated field data

## Usage

### Topic Manager

```go
import "github.com/bsv-blockchain/go-overlay-fiber/example/services/hello"

// Create topic manager
tm := hello.NewHelloWorldTopicManager()

// Use with overlay engine
overlayServer.ConfigureTopicManager("tm_helloworld", tm)
```

### Lookup Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/hello"
    "go.mongodb.org/mongo-driver/mongo"
)

// Create lookup service (requires MongoDB)
ls := hello.NewHelloWorldLookupService(mongoDatabase)

// Use with overlay engine
overlayServer.ConfigureLookupService("ls_helloworld", ls)
```

## API

### Topic Manager Methods

- `IdentifyAdmissibleOutputs(beef, previousCoins)` - Validates and admits PushDrop outputs
- `GetDocumentation()` - Returns topic manager documentation
- `GetMetaData()` - Returns topic manager metadata

### Lookup Service Methods

- `OutputAdmittedByTopic(payload)` - Extracts and stores message from admitted output
- `OutputSpent(payload)` - Removes spent outputs from index
- `OutputEvicted(outpoint)` - Removes evicted outputs from index
- `Lookup(question)` - Queries stored messages

### Query Parameters

```json
{
  "message": "hello (optional - enables full-text search)",
  "limit": 50,
  "skip": 0,
  "startDate": "2024-01-01T00:00:00Z (optional)",
  "endDate": "2024-12-31T23:59:59Z (optional)",
  "sortOrder": "desc"
}
```

## Example Lookup Query

### Search by Message Text

```json
{
  "service": "ls_helloworld",
  "query": {
    "message": "hello world",
    "limit": 10,
    "sortOrder": "desc"
  }
}
```

### List All Messages

```json
{
  "service": "ls_helloworld",
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
  "service": "ls_helloworld",
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
type HelloWorldRecord struct {
    Txid        string
    OutputIndex int
    Message     string
    CreatedAt   time.Time
}
```

### Indexes

- `message` - Full-text index for efficient message searching

## Protocol Specification

**Topic Name**: `tm_helloworld`

**Lookup Service**: `ls_helloworld`

**Encoding**: BRC-48 Pay-to-Push-Drop

**Validation**:
- PushDrop structure must be valid
- Exactly one message field (plus signature)
- Message length >= 2 characters
- Valid ECDSA signature over message data

## TypeScript Equivalent

This is a port of the HelloWorld service from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/hello).

## Related Standards

- **[BRC-48: Pay-to-Push-Drop](https://github.com/bitcoin-sv/BRCs/blob/master/brc-48/0048.md)** - The encoding standard used for HelloWorld messages
