# UMP (User Management Protocol) Service

The UMP service implements the User Management Protocol for managing CWI-style wallet account descriptors on the BSV blockchain.

## Overview

UMP tokens store encrypted wallet configuration data on-chain using BRC-48 PushDrop encoding. Each token contains presentation and recovery key hashes that allow users to look up their wallet configuration.

## Protocol Specification

### Topic Manager: `tm_users`

The UMP topic manager validates PushDrop outputs with the following structure:

**Required Fields (11 minimum):**
- Field 0: `passwordSalt`
- Field 1: `passwordPresentationPrimary`
- Field 2: `passwordRecoveryPrimary`
- Field 3: `presentationRecoveryPrimary`
- Field 4: `passwordPrimaryPrivileged`
- Field 5: `presentationRecoveryPrivileged`
- Field 6: `presentationHash` (32 bytes - used for lookups)
- Field 7: `recoveryHash` (32 bytes - used for lookups)
- Field 8: `presentationKeyEncrypted`
- Field 9: `passwordKeyEncrypted`
- Field 10: `recoveryKeyEncrypted`

**Optional Field:**
- Field 11: `profilesEncrypted`

### Validation Rules

1. Outputs must be valid PushDrop format
2. Must contain at least 11 fields
3. `presentationHash` (field 6) must be exactly 32 bytes
4. `recoveryHash` (field 7) must be exactly 32 bytes
5. Previous UMP tokens are retained when creating new ones (token renewal)

### Lookup Service: `ls_users`

The lookup service indexes UMP tokens and supports queries by:
- `presentationHash`: Find token by presentation key hash
- `recoveryHash`: Find token by recovery key hash
- `outpoint`: Find token by txid.outputIndex

**Returns:** The newest UMP token matching the query criteria

## Usage Example

### Configure UMP Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/ump"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
)

func main() {
    overlayServer := server.NewOverlayServer("mynode", privateKey, hostingURL)

    // Configure MongoDB
    overlayServer.ConfigureMongoDB(mongoURL)

    // Configure UMP topic manager
    tm := ump.NewUMPTopicManager()
    overlayServer.ConfigureTopicManager("tm_users", tm)

    // Configure UMP lookup service (requires MongoDB)
    ls := ump.NewUMPLookupService(overlayServer.MongoDB)
    overlayServer.ConfigureLookupService("ls_users", ls)

    overlayServer.Start()
}
```

### Submit UMP Token

```bash
# Create a transaction with UMP PushDrop output
# The output must contain the 11 required fields

curl -X POST http://localhost:8080/submit \
  -H "Content-Type: application/octet-stream" \
  -H "x-topics: [\"tm_users\"]" \
  --data-binary @ump-token.beef
```

### Lookup UMP Token

```bash
# By presentation hash
curl -X POST http://localhost:8080/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "service": "ls_users",
    "query": {
      "presentationHash": "abc123..."
    }
  }'

# By recovery hash
curl -X POST http://localhost:8080/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "service": "ls_users",
    "query": {
      "recoveryHash": "def456..."
    }
  }'

# By outpoint
curl -X POST http://localhost:8080/lookup \
  -H "Content-Type: application/json" \
  -d '{
    "service": "ls_users",
    "query": {
      "outpoint": "txid.0"
    }
  }'
```

## Use Cases

- **Wallet Recovery**: Users can recover their wallet configuration using presentation or recovery keys
- **Multi-Device Sync**: Store wallet descriptors on-chain for access from multiple devices
- **Account Management**: Update wallet configurations by consuming old tokens and creating new ones
- **CWI Implementation**: Foundation for implementing the CWI (Coherent Wallet Interface) protocol

## Token Lifecycle

1. **Creation**: A new UMP token is created with all required fields
2. **Indexing**: The token is indexed by presentationHash and recoveryHash
3. **Lookup**: Users can find their token using either hash
4. **Renewal**: Old tokens can be consumed and replaced with new ones (all previous coins are retained)
5. **Deletion**: When a token is spent, it's removed from the index

## References

- TypeScript Implementation: [overlay-express-examples/ump](https://github.com/bsv-blockchain/overlay-express-examples/tree/main/src/services/ump)
- CWI Wallet Manager: [wallet-toolbox](https://github.com/bsv-blockchain/wallet-toolbox)
- BRC-48 PushDrop: [PushDrop Specification](https://github.com/bitcoin-sv/BRCs/blob/master/peer-to-peer/0048.md)
