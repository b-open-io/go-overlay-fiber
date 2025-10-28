# Any Topic Manager and Lookup Service

A simple overlay service that admits all transaction outputs with no validation.

## Overview

The Any service is the simplest possible overlay implementation. It demonstrates the basic structure of an overlay service without any complex validation logic.

### Components

1. **AnyTopicManager** - Admits all outputs from any transaction
2. **AnyLookupService** - Provides lookup capabilities for admitted outputs
3. **AnyStorage** - MongoDB-based storage for UTXO records

## Usage

### Topic Manager

```go
import "github.com/bsv-blockchain/go-overlay-fiber/examples/services/any"

// Create topic manager
tm := any.NewAnyTopicManager()

// Use with overlay engine
engine.ConfigureTopicManager("tm_anytx", tm)
```

### Lookup Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/examples/services/any"
    "go.mongodb.org/mongo-driver/mongo"
)

// Create lookup service (requires MongoDB)
ls := any.NewAnyLookupService(mongoDatabase)

// Use with overlay engine
engine.ConfigureLookupService("ls_anytx", ls)
```

## API

### Topic Manager Methods

- `IdentifyAdmissibleOutputs(beef, previousCoins)` - Returns all outputs as admissible
- `GetDocumentation()` - Returns topic manager documentation
- `GetMetaData()` - Returns topic manager metadata

### Lookup Service Methods

- `OutputAdmittedByTopic(payload)` - Stores admitted output
- `OutputSpent(payload)` - Marks output as spent
- `OutputEvicted(txid, outputIndex)` - Removes output from index
- `Lookup(question)` - Queries stored outputs

### Query Parameters

```json
{
  "txid": "string (optional)",
  "limit": 50,
  "skip": 0,
  "startDate": "2024-01-01T00:00:00Z (optional)",
  "endDate": "2024-12-31T23:59:59Z (optional)",
  "sortOrder": "desc"
}
```

## Example Lookup Query

```json
{
  "service": "ls_anytx",
  "query": {
    "limit": 10,
    "skip": 0,
    "sortOrder": "desc"
  }
}
```

## Storage

The service uses MongoDB with the following schema:

```go
type AnyRecord struct {
    Txid         string
    OutputIndex  int
    CreatedAt    time.Time
    SpendingTxid *string (optional)
}
```

### Indexes

- `txid` - For efficient transaction lookups

## Protocol Specification

**Admission Rules**: None - all outputs are admitted

**Topic Name**: `tm_anytx`

**Lookup Service**: `ls_anytx`

## TypeScript Equivalent

This is a port of the Any service from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/any).
