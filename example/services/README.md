# Example Overlay Services

This directory contains example implementations of overlay services (topic managers and lookup services) that demonstrate how to build overlay functionality on top of the go-overlay-fiber framework.

## Available Services

### Any Service (`any/`)

The simplest possible overlay service that admits all transaction outputs with no validation rules.

- **Topic Manager**: `tm_anytx` - Admits all outputs
- **Lookup Service**: `ls_anytx` - Simple UTXO lookup by txid or date range
- **Complexity**: Very Low
- **Use Case**: Learning, testing, demonstrations

See [any/README.md](./any/README.md) for details.

### HelloWorld Service (`hello/`)

A messaging protocol using BRC-48 Pay-to-Push-Drop outputs for broadcasting UTF-8 messages.

- **Topic Manager**: `tm_helloworld` - Validates PushDrop messages with signatures
- **Lookup Service**: `ls_helloworld` - Full-text search and querying of messages
- **Complexity**: Low
- **Use Case**: Messaging, basic PushDrop implementation example

See [hello/README.md](./hello/README.md) for details.

### SlackThreads Service (`slackthreads/`)

A protocol for storing 32-byte hashes on-chain using simple script pattern validation.

- **Topic Manager**: `tm_slackthread` - Validates OP_SHA256 <hash> OP_EQUAL pattern
- **Lookup Service**: `ls_slackthread` - Query thread hashes by hash, txid, or date range
- **Complexity**: Low
- **Use Case**: Hash storage, simple script validation example (NOT PushDrop)

See [slackthreads/README.md](./slackthreads/README.md) for details.

### MessageBox Service (`messagebox/`)

An identity-based message routing protocol for advertising MessageBox hosts on-chain.

- **Topic Manager**: `tm_messagebox` - Validates PushDrop advertisements with identity key signatures
- **Lookup Service**: `ls_messagebox` - Query hosts by identity key
- **Complexity**: Medium
- **Use Case**: Identity-to-host resolution, message routing, host advertisement

See [messagebox/README.md](./messagebox/README.md) for details.

### UHRP Service (`uhrp/`)

Universal Hash Resolution Protocol - A protocol for advertising file hosting availability on the blockchain.

- **Topic Manager**: `tm_uhrp` - Validates file hosting advertisements with signature verification
- **Lookup Service**: `ls_uhrp` - Query by file hash, host, expiry time, or file size
- **Complexity**: Medium-High
- **Use Case**: Decentralized CDN, file availability discovery, content hosting advertisements

See [uhrp/README.md](./uhrp/README.md) for details.

## Using These Services

These are example implementations meant to demonstrate the patterns for building overlay services. You can:

1. **Use them as-is** in your overlay node for testing
2. **Reference them** when building your own custom services
3. **Extend them** to add more functionality

## Importing Services

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/examples/services/any"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
)

func main() {
    overlayServer := server.NewOverlayServer("mynode", privateKey, hostingURL)

    // Configure topic manager
    tm := any.NewAnyTopicManager()
    overlayServer.ConfigureTopicManager("tm_anytx", tm)

    // Configure lookup service (requires MongoDB)
    ls := any.NewAnyLookupService(mongoDatabase)
    overlayServer.ConfigureLookupService("ls_anytx", ls)

    overlayServer.Start()
}
```

## TypeScript Equivalents

These services are ports of the examples from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples). Each service maintains compatibility with its TypeScript counterpart.

## Future Services

Additional services to be ported:

- **DID** - Decentralized Identifiers
- **UMP** - Universal Messaging Protocol
- **ProtoMap** - Protocol metadata registration
- **BasketMap** - Basket type registration
- **CertMap** - Certificate type mapping

Contributions welcome!
