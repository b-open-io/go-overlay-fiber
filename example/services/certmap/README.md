# CertMap Example Service

A protocol for registering certificate types on-chain, enabling decentralized certificate type discovery and standardization.

## Overview

The CertMap service implements a certificate type registration system using PushDrop tokens. Registry operators can register certificate types on-chain with rich metadata, which can then be queried by type, name, or registry operator.

**Topic Manager**: `tm_certmap`
**Lookup Service**: `ls_certmap`
**Complexity**: Medium

## Protocol Rules

### CertMap Token Structure

CertMap tokens use PushDrop format with exactly 8 fields:
- **Field 0**: Certificate type identifier
- **Field 1**: Certificate name
- **Field 2**: Icon URL
- **Field 3**: Description
- **Field 4**: Documentation URL
- **Field 5**: JSON-encoded certificate fields schema
- **Field 6**: Registry operator identity key
- **Field 7**: BRC-48 signature over fields 0-6

### Required Fields

Each certificate type registration must include:
- **type**: Unique certificate type identifier
- **name**: Human-readable certificate name
- **iconURL**: Icon URL for the certificate type
- **description**: Description of the certificate type
- **documentationURL**: URL to documentation about this certificate type
- **certFields**: JSON object defining the schema for this certificate type
- **registryOperator**: Identity key of the registry operator

### Admissibility Rules

The topic manager validates that:
1. Output can be decoded as PushDrop with exactly 8 fields
2. All required fields are present and non-empty
3. Field 5 (certFields) contains valid JSON
4. The BRC-48 signature is valid for the claimed registry operator
5. The locking public key matches the expected derived child key

### Signature Verification

The protocol uses BRC-48 signatures with protocol ID `[1, 'certmap']` to verify:
- The signature is valid for the claimed registry operator identity key
- The locking public key matches the expected derived child key

This ensures that only the actual registry operator can create valid certificate type registrations for their identity key.

## Query Types

The lookup service supports multiple query patterns:

### 1. Type Lookup
```go
Query: {
    "type": "Certificate Type ID",
    "registryOperators": ["operator_identity_key"]
}
```
Returns all certificate type registrations matching the specified type from the given registry operators.

### 2. Name Fuzzy Search
```go
Query: {
    "name": "Certificate Name",
    "registryOperators": ["operator_identity_key"]
}
```
Fuzzy-matches certificate types by name (case-insensitive) from the given registry operators.

**Note**: All queries require the `registryOperators` parameter to specify which registry operators to trust.

## Storage Schema

### MongoDB Collection: `certmapRecords`

```go
type CertMapRecord struct {
    Txid         string               `bson:"txid"`
    OutputIndex  int                  `bson:"outputIndex"`
    Registration *CertMapRegistration `bson:"registration"`
    CreatedAt    time.Time            `bson:"createdAt"`
}
```

## Use Cases

### Certificate Type Registration
1. Registry operator defines a new certificate type with its schema
2. Metadata is encoded as PushDrop token with BRC-48 signature
3. Transaction is submitted to overlay network
4. Topic manager validates structure and signature
5. Lookup service indexes the certificate type registration

### Certificate Type Discovery
1. User queries by type or name with trusted registry operators
2. Lookup service returns matching certificate types
3. Results include txid and outputIndex for UTXO references
4. Client can fetch full metadata from the blockchain

### Certificate Type Updates
1. Registry operator creates new version with updated metadata
2. Old UTXO is spent (removes old registration)
3. New UTXO is created (adds new registration)
4. Registry reflects latest version

### Multi-Operator Registries
1. Multiple registry operators can register certificate types
2. Clients specify which operators they trust in queries
3. Enables decentralized certificate type standards
4. Operators can coordinate on shared type definitions

## API Methods

### Topic Manager (tm_certmap)

```go
func (tm *CertMapTopicManager) IdentifyAdmissibleOutputs(
    ctx context.Context,
    beef []byte,
    previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error)
```

Validates certificate type registration data and determines which outputs should be admitted to the overlay network.

### Lookup Service (ls_certmap)

```go
func (ls *CertMapLookupService) Lookup(
    ctx context.Context,
    question *lookup.LookupQuestion,
) (*lookup.LookupAnswer, error)
```

Queries certificate type registrations based on the provided query parameters.

## Example Usage

```go
import (
    "context"
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/certmap"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
    "go.mongodb.org/mongo-driver/mongo"
)

func main() {
    // Create MongoDB client
    mongoClient, _ := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    db := mongoClient.Database("overlay")

    // Create overlay server
    overlayServer := server.NewOverlayServer("mynode", privateKey, "https://myhost.com")

    // Configure CertMap topic manager
    tm := certmap.NewCertMapTopicManager()
    overlayServer.ConfigureTopicManager("tm_certmap", tm)

    // Configure CertMap lookup service
    ls := certmap.NewCertMapLookupService(db)
    overlayServer.ConfigureLookupService("ls_certmap", ls)

    // Start server
    overlayServer.Start()
}
```

### Querying with LookupResolver

```go
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by type
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_certmap",
    Query: map[string]interface{}{
        "type": "employment",
        "registryOperators": []string{"registry_operator_identity_key"},
    },
}, 10000)

// Find by name (fuzzy)
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_certmap",
    Query: map[string]interface{}{
        "name": "Employment Certificate",
        "registryOperators": []string{"registry_operator_identity_key"},
    },
}, 10000)
```

### Registering a Certificate Type

```go
import (
    "encoding/json"
    "github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

certFields := map[string]interface{}{
    "employer": "string",
    "position": "string",
    "startDate": "date",
    "endDate": "date",
}

certFieldsJSON, _ := json.Marshal(certFields)

fields := [][]byte{
    []byte("employment"),                                    // type
    []byte("Employment Certificate"),                        // name
    []byte("https://example.com/icons/employment.png"),      // iconURL
    []byte("Certifies employment history and position"),     // description
    []byte("https://example.com/docs/employment-cert.html"), // documentationURL
    certFieldsJSON,                                          // certFields
    []byte(registryOperatorIdentityKey),                     // registryOperator
    signature,                                                // signature (field 7)
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

### Type Collision
- Same type identifier can be registered by different operators
- Clients filter by trusted operators to resolve conflicts
- Operators should coordinate on standards

### Content Validation
- certFields schema is stored but not validated on-chain
- Applications must validate certificate data against schemas
- Off-chain validation recommended for field types and structure

## Future Enhancements

Potential future enhancements include:

1. **Schema Validation**: On-chain or off-chain validation of certFields schemas
2. **Type Versioning**: Support for versioned certificate type schemas
3. **Operator Reputation**: Reputation system for registry operators
4. **Cross-Reference**: Link related certificate types
5. **Certificate Templates**: Pre-defined templates for common certificate types

## TypeScript Equivalent

This service is a port of the CertMap example from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/certmap).

## References

- [BRC-48: PushDrop and Key Derivation](https://brc.dev/48)
- [PushDrop Protocol](https://github.com/bsv-blockchain/go-sdk/tree/master/transaction/template/pushdrop)
