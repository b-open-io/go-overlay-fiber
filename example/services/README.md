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

### DID Service (`did/`)

A protocol for storing decentralized identifier (DID) serial numbers on-chain using BRC-48 PushDrop encoding.

- **Topic Manager**: `tm_did` - Validates PushDrop DID serial numbers
- **Lookup Service**: `ls_did` - Query by serial number or outpoint
- **Complexity**: Very Low
- **Use Case**: DID resolution, certificate serial number storage, identity references

See [did/README.md](./did/README.md) for details.

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

### UMP Service (`ump/`)

User Management Protocol - A protocol for managing CWI-style wallet account descriptors on-chain.

- **Topic Manager**: `tm_users` - Validates PushDrop UMP tokens with 11+ fields
- **Lookup Service**: `ls_users` - Query by presentationHash, recoveryHash, or outpoint
- **Complexity**: Medium
- **Use Case**: Wallet recovery, multi-device sync, account management, CWI implementation

See [ump/README.md](./ump/README.md) for details.

### ProtoMap Service (`protomap/`)

Protocol Registry - A protocol for storing protocol metadata and enabling UX enrichment for BSV applications.

- **Topic Manager**: `tm_protomap` - Validates PushDrop protocol registrations with 7 fields
- **Lookup Service**: `ls_protomap` - Query by protocol name or protocolID with registry operators
- **Complexity**: Medium
- **Use Case**: Protocol discovery, UX enrichment, registry management, application integration

See [protomap/README.md](./protomap/README.md) for details.

### BasketMap Service (`basketmap/`)

Basket Registry - A protocol for storing basket type metadata and enabling UX enrichment for BSV applications.

- **Topic Manager**: `tm_basketmap` - Validates PushDrop basket registrations with 7 fields
- **Lookup Service**: `ls_basketmap` - Query by basket ID or name (fuzzy search) with registry operators
- **Complexity**: Medium
- **Use Case**: Basket discovery, UX enrichment, registry management, application integration

See [basketmap/README.md](./basketmap/README.md) for details.

### Identity Service (`identity/`)

A protocol for managing BRC-48 identity certificates on-chain, enabling decentralized identity resolution and verification.

- **Topic Manager**: `tm_identity` - Validates BRC-48 VerifiableCertificate structure
- **Lookup Service**: `ls_identity` - Query by serial number, attributes, identity key, certificate type, or certifiers (with fuzzy search)
- **Complexity**: Medium-High
- **Use Case**: Identity certificate management, attribute-based discovery, certificate verification, decentralized identity

See [identity/README.md](./identity/README.md) for details.

### Apps Service (`apps/`)

A protocol for publishing Metanet App catalog entries on-chain, enabling decentralized app discovery and distribution.

- **Topic Manager**: `tm_apps` - Validates app metadata with required fields (version, name, description, icon, domain, publisher, release_date)
- **Lookup Service**: `ls_apps` - Query by domain, publisher, name (fuzzy), tags, category, outpoint, or browse all (with pagination and sorting)
- **Complexity**: Medium
- **Use Case**: App catalog, app discovery, publisher-based distribution, domain-based ecosystems, version management

See [apps/README.md](./apps/README.md) for details.

### CertMap Service (`certmap/`)

A protocol for registering certificate types on-chain, enabling decentralized certificate type discovery and standardization.

- **Topic Manager**: `tm_certmap` - Validates certificate type registrations with required fields (type, name, iconURL, description, documentationURL, certFields, registryOperator)
- **Lookup Service**: `ls_certmap` - Query by type or name (fuzzy search) with registry operators
- **Complexity**: Medium
- **Use Case**: Certificate type registry, certificate discovery, registry operator management, certificate standardization

See [certmap/README.md](./certmap/README.md) for details.

### Fractionalize Service (`fractionalize/`)

A proof-of-concept protocol for fractionalized ownership using BSV-20 style ordinal tokens with specialized locking mechanisms.

- **Topic Manager**: `tm_fractionalize` - Validates three output types: server tokens (ordinal+multisig), transfer tokens (ordinal only), and payment outputs (multisig only)
- **Lookup Service**: `ls_fractionalize` - Query by txid or date range with pagination and sorting
- **Complexity**: Medium-High
- **Use Case**: Fractionalized ownership PoC, BSV-20 token validation, ordinal inscription with custom locking, multi-pattern script validation

See [fractionalize/README.md](./fractionalize/README.md) for details.

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

- **TokenMap** - Token type mapping

Contributions welcome!
