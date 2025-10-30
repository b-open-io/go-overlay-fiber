# ProtoMap (Protocol Registry) Service

The ProtoMap service implements a protocol registry for storing protocol metadata and enabling UX enrichment for BSV applications.

## Overview

ProtoMap tokens store protocol registration information on-chain using BRC-48 PushDrop encoding. Each token contains protocol metadata including name, description, icon URL, and documentation that can be looked up by protocol ID or name.

## Protocol Specification

### Topic Manager: `tm_protomap`

The ProtoMap topic manager validates PushDrop outputs with the following structure:

**Required Fields (7 total):**
- Field 0: `protocolID` (JSON array: `[securityLevel, "protocol-name"]`)
  - Security level: 0, 1, or 2
  - Protocol: string identifier
- Field 1: `name` (protocol display name)
- Field 2: `iconURL` (URL to protocol icon)
- Field 3: `description` (protocol description)
- Field 4: `documentationURL` (URL to protocol documentation)
- Field 5: `registryOperator` (identity key of registry operator)
- Field 6: `signature` (signature over fields 0-5)

### Validation Rules

1. Outputs must be valid PushDrop format
2. Must contain exactly 7 fields
3. `protocolID` must be valid JSON array with 2 elements
4. Security level must be 0, 1, or 2
5. Protocol string must be non-empty
6. All required fields (name, iconURL, description, documentationURL, registryOperator) must be non-empty
7. Signature field must be present (signature verification pending implementation)
8. Previous ProtoMap tokens are retained when creating new ones

### Lookup Service: `ls_protomap`

The lookup service indexes ProtoMap registrations and supports queries by:
- `name` + `registryOperators`: Find protocol by display name
- `protocolID` + `registryOperators`: Find protocol by ID

**Query Parameters:**
- `name` (string): Protocol display name
- `registryOperators` (string[]): Array of trusted registry operator identity keys
- `protocolID` (object): Protocol identifier with `securityLevel` and `protocol` fields

**Returns:** Array of UTXO references for all matching protocol registrations

## Usage Example

### Configure ProtoMap Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/protomap"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
)

func main() {
    overlayServer := server.NewOverlayServer("mynode", privateKey, hostingURL)

    // Configure MongoDB
    overlayServer.ConfigureMongoDB(mongoURL)

    // Configure ProtoMap topic manager
    tm := protomap.NewProtoMapTopicManager()
    overlayServer.ConfigureTopicManager("tm_protomap", tm)

    // Configure ProtoMap lookup service (requires MongoDB)
    ls := protomap.NewProtoMapLookupService(overlayServer.MongoDB)
    overlayServer.ConfigureLookupService("ls_protomap", ls)

    overlayServer.Start()
}
```

### Submit Protocol Registration

```bash
# Create a transaction with ProtoMap PushDrop output
# The output must contain the 7 required fields

curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/octet-stream" \
  -H "x-topics: [\"tm_protomap\"]" \
  --data-binary @protocol-registration.beef
```

### Lookup Protocol

```bash
# By name and registry operators
curl -X POST http://localhost:8080/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "service": "ls_protomap",
    "query": {
      "name": "my-protocol",
      "registryOperators": ["operator1", "operator2"]
    }
  }'

# By protocolID and registry operators
curl -X POST http://localhost:8080/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "service": "ls_protomap",
    "query": {
      "protocolID": {
        "securityLevel": 1,
        "protocol": "my-protocol"
      },
      "registryOperators": ["operator1"]
    }
  }'
```

## Use Cases

- **UX Enrichment**: Display protocol names, icons, and descriptions in wallet interfaces
- **Protocol Discovery**: Find protocols by name or ID
- **Registry Management**: Trusted operators can register and update protocol metadata
- **Application Integration**: Applications can look up protocol information to enhance user experience

## Token Lifecycle

1. **Registration**: A registry operator creates a ProtoMap token with protocol metadata
2. **Indexing**: The token is indexed by protocol name and protocolID
3. **Lookup**: Applications can find protocol information using name or ID
4. **Update**: Old registrations can be consumed and replaced with updated ones
5. **Deletion**: When a token is spent, it's removed from the index

## Security

- **Registry Operators**: Only trusted registry operators should be queried to prevent malicious registrations
- **Signature Verification**: The signature field ensures registrations are authentic (verification pending implementation)
- **Multiple Operators**: Query multiple operators to ensure consensus on protocol metadata

## Data Structure

### ProtoMap Record (MongoDB)
```go
type ProtoMapRecord struct {
    Txid         string               // Transaction ID
    OutputIndex  int                  // Output index
    Registration ProtoMapRegistration // Registration details
    CreatedAt    time.Time           // Timestamp
}

type ProtoMapRegistration struct {
    RegistryOperator string     // Identity key of operator
    ProtocolID       ProtocolID // Protocol identifier
    Name             string     // Display name
}

type ProtocolID struct {
    SecurityLevel int    // 0, 1, or 2
    Protocol      string // Protocol identifier
}
```

## References

- TypeScript Implementation: [overlay-express-examples/protomap](https://github.com/bsv-blockchain/overlay-express-examples/tree/main/src/services/protomap)
- BRC-48 PushDrop: [PushDrop Specification](https://github.com/bitcoin-sv/BRCs/blob/master/peer-to-peer/0048.md)
- Protocol Registry Concept: [overlay-services documentation](https://github.com/bsv-blockchain/overlay-services)
