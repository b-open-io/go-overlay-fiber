# Identity Example Service

A protocol for managing BRC-48 identity certificates on-chain, enabling decentralized identity resolution and verification.

## Overview

The Identity service implements a complete identity certificate management system using the BRC-48 standard. It allows users to publish verifiable certificates on-chain, which can then be queried by various criteria including attributes, identity keys, certificate types, and certifiers.

**Topic Manager**: `tm_identity`
**Lookup Service**: `ls_identity`
**Complexity**: Medium-High

## Protocol Rules

### Identity Token Structure

Identity tokens use BRC-48 certificates encoded in PushDrop format:
- **Field 0**: JSON-encoded VerifiableCertificate
- **Field 1**: Signature over the certificate data

### Certificate Requirements

Each certificate must include:
- **type**: Certificate type identifier
- **serialNumber**: Unique certificate serial number
- **subject**: Public identity key (subject of the certificate)
- **certifier**: Public key of the issuing certifier
- **fields**: Key-value pairs of certified attributes

### Admissibility Rules

The topic manager validates that:
1. Output can be decoded as PushDrop with at least 1 field
2. Field 0 contains valid JSON certificate data
3. Certificate has required fields (type, serialNumber, subject, certifier)
4. Certificate contains at least one attribute field
5. Subject and certifier public keys are valid

### Field Encryption

Certificates support selective field revelation through encryption:
- Fields can be encrypted to specific public keys
- The keyring allows authorized verifiers to decrypt
- Public revelation uses the "anyone" wallet derivation

## Query Types

The lookup service supports five query patterns (in priority order):

### 1. Serial Number Lookup (Unique)
```go
Query: {
    "serialNumber": "abc123..."
}
```
Returns the single certificate with this serial number.

### 2. Attribute Search with Certifiers
```go
Query: {
    "attributes": {
        "name": "John",
        "country": "USA"
    },
    "certifiers": ["certifier_pubkey1", "certifier_pubkey2"]
}
```
Searches for certificates matching the specified attributes from trusted certifiers. Supports fuzzy matching.

Special key `"any"` searches across all fields:
```go
Query: {
    "attributes": {
        "any": "John"
    },
    "certifiers": ["certifier_pubkey"]
}
```

### 3. Identity Key with Certificate Types
```go
Query: {
    "identityKey": "subject_pubkey",
    "certificateTypes": ["type1", "type2"],
    "certifiers": ["certifier_pubkey"]
}
```
Finds certificates for a specific identity with specific types from trusted certifiers.

### 4. Identity Key with Certifiers
```go
Query: {
    "identityKey": "subject_pubkey",
    "certifiers": ["certifier_pubkey"]
}
```
Finds all certificates for a specific identity from trusted certifiers.

### 5. Certifiers Only
```go
Query: {
    "certifiers": ["certifier_pubkey1", "certifier_pubkey2"]
}
```
Returns all certificates issued by the specified certifiers.

## Fuzzy Attribute Search

The service implements fuzzy search for attributes:
- Searching for "John" matches "Johnny", "Johnson", etc.
- Pattern: "abc" becomes regex "a.*b.*c"
- Case-insensitive matching
- Excludes binary fields (profilePhoto, icon) from search

## Storage Schema

### MongoDB Collection: `identityRecords`

```go
type IdentityRecord struct {
    Txid                 string                    `bson:"txid"`
    OutputIndex          int                       `bson:"outputIndex"`
    Certificate          *certificates.Certificate `bson:"certificate"`
    CreatedAt            time.Time                 `bson:"createdAt"`
    SearchableAttributes string                    `bson:"searchableAttributes,omitempty"`
}
```

### Indexes
- Text index on `searchableAttributes` for full-text search

## Use Cases

### Identity Certificate Issuance
1. Certifier creates a VerifiableCertificate with subject's identity key
2. Certifier signs the certificate
3. Certificate is encoded in PushDrop format
4. Transaction is submitted to overlay network
5. Topic manager validates and admits the output
6. Lookup service indexes the certificate

### Identity Verification
1. Verifier queries by identity key and trusted certifiers
2. Lookup service returns matching certificates
3. Verifier can decrypt relevant fields (if authorized)
4. Verifier validates certificate signatures

### Attribute-Based Discovery
1. Query for certificates with specific attributes (e.g., "country": "USA")
2. Fuzzy search helps match partial values
3. Results filtered by trusted certifiers
4. Enables discovery without knowing exact identity keys

### Certificate Revocation
1. Certificate owner spends the UTXO
2. Lookup service removes the certificate from index
3. Certificate is no longer discoverable

## API Methods

### Topic Manager (tm_identity)

```go
func (tm *IdentityTopicManager) IdentifyAdmissibleOutputs(
    ctx context.Context,
    beef []byte,
    previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error)
```

Validates Identity certificate outputs and determines which should be admitted to the overlay network.

### Lookup Service (ls_identity)

```go
func (ls *IdentityLookupService) Lookup(
    ctx context.Context,
    question *lookup.LookupQuestion,
) (*lookup.LookupAnswer, error)
```

Queries identity certificates based on the provided query parameters.

## Example Usage

```go
import (
    "context"
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/identity"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
    "go.mongodb.org/mongo-driver/mongo"
)

func main() {
    // Create MongoDB client
    mongoClient, _ := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    db := mongoClient.Database("overlay")

    // Create overlay server
    overlayServer := server.NewOverlayServer("mynode", privateKey, "https://myhost.com")

    // Configure Identity topic manager
    tm := identity.NewIdentityTopicManager()
    overlayServer.ConfigureTopicManager("tm_identity", tm)

    // Configure Identity lookup service
    ls := identity.NewIdentityLookupService(db)
    overlayServer.ConfigureLookupService("ls_identity", ls)

    // Start server
    overlayServer.Start()
}
```

### Querying with LookupResolver

```go
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by attributes
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_identity",
    Query: map[string]interface{}{
        "attributes": map[string]string{
            "name": "John",
            "country": "USA",
        },
        "certifiers": []string{"certifier_pubkey"},
    },
}, 10000)

// Find by identity key
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_identity",
    Query: map[string]interface{}{
        "identityKey": "subject_pubkey",
        "certifiers": []string{"certifier_pubkey"},
    },
}, 10000)
```

## Security Considerations

### Certificate Validation
- Subject and certifier public keys are validated
- Certificate structure is verified
- Field presence is confirmed
- Future: Signature verification and field decryption

### Trusted Certifiers
- Queries require specifying trusted certifiers
- Prevents accepting certificates from untrusted sources
- Allows building web of trust

### Field Privacy
- Encrypted fields protect sensitive data
- Selective revelation to authorized parties
- Keyring controls access to decrypted values

### UTXO Spending
- Spending a certificate UTXO revokes it
- Prevents reuse of revoked certificates
- Certificate lifecycle tied to blockchain state

## Pending Features

The current implementation includes placeholders for:

1. **Full Certificate Signature Verification**: Verify signatures over certificate fields using ProtoWallet
2. **Field Decryption**: Decrypt certificate fields for public revelation (requires complete wallet.Interface support)
3. **Certificate Chain Validation**: Verify certifier certificates form valid chain

These features require additional wallet infrastructure currently being developed in go-sdk.

## TypeScript Equivalent

This service is a port of the Identity example from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/identity).

## References

- [BRC-48: Identity Certificates](https://brc.dev/48)
- [go-sdk/auth/certificates](https://github.com/bsv-blockchain/go-sdk/tree/master/auth/certificates)
- [PushDrop Protocol](https://github.com/bsv-blockchain/go-sdk/tree/master/transaction/template/pushdrop)
