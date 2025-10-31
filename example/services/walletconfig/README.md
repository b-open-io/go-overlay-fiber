# WalletConfig Example Service

A protocol for registering wallet configuration options on-chain, enabling decentralized wallet service discovery.

## Overview

The WalletConfig service implements a registry system for wallet configuration options using PushDrop tokens. Registry operators can register wallet configurations on-chain with service URLs (WAB, storage, messagebox, legal), which can then be queried by configID, name, or specific service URLs.

**Topic Manager**: `tm_walletconfig`
**Lookup Service**: `ls_walletconfig`
**Complexity**: Medium

## Protocol Rules

### WalletConfig Token Structure

WalletConfig tokens use PushDrop format with exactly 9 fields:
- **Field 0**: Configuration ID
- **Field 1**: Configuration name
- **Field 2**: Icon URL
- **Field 3**: WAB (Wallet Authentication Backend) URL
- **Field 4**: Storage URL
- **Field 5**: Messagebox URL
- **Field 6**: Legal terms URL
- **Field 7**: Registry operator identity key
- **Field 8**: BRC-48 signature over fields 0-7

### Required Fields

Each wallet configuration registration must include:
- **configID**: Unique configuration identifier
- **name**: Human-readable wallet configuration name
- **icon**: Icon URL for the wallet configuration
- **wab**: Wallet Authentication Backend URL
- **storage**: Wallet storage service URL
- **messagebox**: Messagebox service URL
- **legal**: Legal terms and conditions URL
- **registryOperator**: Identity key of the registry operator

### Admissibility Rules

The topic manager validates that:
1. Output can be decoded as PushDrop with exactly 9 fields
2. All required fields are present and non-empty
3. The BRC-48 signature is valid for the claimed registry operator
4. The locking public key matches the expected derived child key

### Signature Verification

The protocol uses BRC-48 signatures with protocol ID `[1, 'wallet config option']` to verify:
- The signature is valid for the claimed registry operator identity key
- The locking public key matches the expected derived child key

This ensures that only the actual registry operator can create valid wallet configuration registrations for their identity key.

## Query Types

The lookup service supports multiple query patterns:

### 1. ConfigID Lookup
```go
Query: {
    "configID": "my-wallet-config",
    "registryOperators": ["operator_identity_key"]
}
```
Returns all wallet configuration registrations matching the specified configID from the given registry operators.

### 2. Name Fuzzy Search
```go
Query: {
    "name": "My Wallet",
    "registryOperators": ["operator_identity_key"]
}
```
Fuzzy-matches wallet configurations by name (case-insensitive) from the given registry operators.

### 3. WAB URL Lookup
```go
Query: {
    "wab": "https://wab.example.com",
    "registryOperators": ["operator_identity_key"]
}
```
Returns all wallet configurations using the specified WAB URL.

### 4. Storage URL Lookup
```go
Query: {
    "storage": "https://storage.example.com",
    "registryOperators": ["operator_identity_key"]
}
```
Returns all wallet configurations using the specified storage URL.

### 5. Messagebox URL Lookup
```go
Query: {
    "messagebox": "https://messagebox.example.com",
    "registryOperators": ["operator_identity_key"]
}
```
Returns all wallet configurations using the specified messagebox URL.

### 6. List All Configurations
```go
Query: {
    "registryOperators": ["operator_identity_key"]
}
```
Returns all wallet configurations from the specified registry operators.

**Note**: All queries require the `registryOperators` parameter to specify which registry operators to trust.

## Storage Schema

### MongoDB Collection: `walletConfigRecords`

```go
type WalletConfigRecord struct {
    Txid         string                    `bson:"txid"`
    OutputIndex  int                       `bson:"outputIndex"`
    Registration *WalletConfigRegistration `bson:"registration"`
    CreatedAt    time.Time                 `bson:"createdAt"`
}
```

### Duplicate Prevention

The storage layer prevents duplicate registrations by checking if a record with identical field values already exists before inserting.

## Use Cases

### Wallet Configuration Registration
1. Registry operator defines wallet configuration with service URLs
2. Configuration is encoded as PushDrop token with BRC-48 signature
3. Transaction is submitted to overlay network
4. Topic manager validates structure and signature
5. Lookup service indexes the configuration (preventing duplicates)

### Wallet Service Discovery
1. Wallet queries by configID or name with trusted registry operators
2. Lookup service returns matching wallet configurations
3. Results include txid and outputIndex for UTXO references
4. Wallet can fetch full configuration from the blockchain

### Service Provider Discovery
1. User queries by specific service URL (WAB, storage, messagebox)
2. Lookup service returns all wallet configurations using that service
3. Enables discovery of wallets using specific service providers
4. Supports multi-vendor wallet ecosystems

### Configuration Updates
1. Registry operator creates new version with updated service URLs
2. Old UTXO is spent (removes old configuration)
3. New UTXO is created (adds new configuration)
4. Registry reflects latest version

## API Methods

### Topic Manager (tm_walletconfig)

```go
func (tm *WalletConfigTopicManager) IdentifyAdmissibleOutputs(
    ctx context.Context,
    beef []byte,
    previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error)
```

Validates wallet configuration data and determines which outputs should be admitted to the overlay network.

### Lookup Service (ls_walletconfig)

```go
func (ls *WalletConfigLookupService) Lookup(
    ctx context.Context,
    question *lookup.LookupQuestion,
) (*lookup.LookupAnswer, error)
```

Queries wallet configurations based on the provided query parameters.

## Example Usage

```go
import (
    "context"
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/walletconfig"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
    "go.mongodb.org/mongo-driver/mongo"
)

func main() {
    // Create MongoDB client
    mongoClient, _ := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    db := mongoClient.Database("overlay")

    // Create overlay server
    overlayServer := server.NewOverlayServer("mynode", privateKey, "https://myhost.com")

    // Configure WalletConfig topic manager
    tm := walletconfig.NewWalletConfigTopicManager()
    overlayServer.ConfigureTopicManager("tm_walletconfig", tm)

    // Configure WalletConfig lookup service
    ls := walletconfig.NewWalletConfigLookupService(db)
    overlayServer.ConfigureLookupService("ls_walletconfig", ls)

    // Start server
    overlayServer.Start()
}
```

### Querying with LookupResolver

```go
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by configID
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_walletconfig",
    Query: map[string]interface{}{
        "configID": "my-wallet-config",
        "registryOperators": []string{"registry_operator_identity_key"},
    },
}, 10000)

// Find by name (fuzzy)
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_walletconfig",
    Query: map[string]interface{}{
        "name": "My Wallet",
        "registryOperators": []string{"registry_operator_identity_key"},
    },
}, 10000)

// Find by WAB URL
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_walletconfig",
    Query: map[string]interface{}{
        "wab": "https://wab.example.com",
        "registryOperators": []string{"registry_operator_identity_key"},
    },
}, 10000)

// List all configs from trusted operators
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_walletconfig",
    Query: map[string]interface{}{
        "registryOperators": []string{"registry_operator_identity_key"},
    },
}, 10000)
```

### Registering a Wallet Configuration

```go
import (
    "github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

fields := [][]byte{
    []byte("my-wallet-config"),                          // configID
    []byte("My Awesome Wallet"),                         // name
    []byte("https://example.com/icons/wallet.png"),      // icon
    []byte("https://wab.example.com"),                   // wab
    []byte("https://storage.example.com"),               // storage
    []byte("https://messagebox.example.com"),            // messagebox
    []byte("https://example.com/legal.html"),            // legal
    []byte(registryOperatorIdentityKey),                 // registryOperator
    signature,                                            // signature (field 8)
}

// Create PushDrop token
lockingScript := pushdrop.Encode(fields, lockingPublicKey)

// Add output to transaction and broadcast
```

## Security Considerations

### Registry Operator Verification
- BRC-48 signatures link tokens to registry operator identity keys
- Prevents impersonation of registry operators
- Enables trust verification

### Multi-Operator Trust Model
- Clients specify which registry operators they trust
- Enables competitive registries or cooperative standards
- Prevents single point of failure

### Configuration Collision
- Same configID can be registered by different operators
- Clients filter by trusted operators to resolve conflicts
- Operators should coordinate on standards

### Duplicate Prevention
- Storage layer checks for duplicate registrations
- Prevents storage bloat from repeated registrations
- Only stores unique configurations

### Service URL Validation
- URLs are stored but not validated on-chain
- Applications should validate URL accessibility
- Off-chain validation recommended for service availability

## Use Case Examples

### Multi-Wallet Ecosystem
1. Multiple wallet providers register their configurations
2. Users query by name to discover available wallets
3. Each wallet specifies its own service providers
4. Users choose wallet based on features and trust

### Service Provider Migration
1. Wallet operator changes service providers
2. New configuration registered with updated URLs
3. Old configuration spent and removed
4. Users automatically discover new service URLs

### Wallet Discovery by Services
1. Developer queries by WAB URL
2. Discovers all wallets using specific WAB
3. Enables integration testing and compatibility checks
4. Supports service provider analytics

## Future Enhancements

Potential future enhancements include:

1. **Service Health Monitoring**: Track service availability and performance
2. **Version Management**: Support versioned configurations
3. **Operator Reputation**: Reputation system for registry operators
4. **Configuration Templates**: Pre-defined templates for common setups
5. **Service Discovery Protocol**: Standardized protocol for service capabilities

## TypeScript Equivalent

This service is a port of the WalletConfig example from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/walletconfig).

## References

- [BRC-48: PushDrop and Key Derivation](https://brc.dev/48)
- [PushDrop Protocol](https://github.com/bsv-blockchain/go-sdk/tree/master/transaction/template/pushdrop)
- [Wallet Authentication Backend](https://projectbabbage.com)
