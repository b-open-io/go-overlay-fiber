# BasketMap Topic Manager and Lookup Service

Basket Registry - A protocol for storing basket type metadata and enabling UX enrichment for BSV applications.

## Overview

The BasketMap service enables registry operators to publish cryptographically signed basket type registrations on-chain, containing metadata like names, descriptions, icons, and documentation URLs. This creates a decentralized basket registry where applications can discover and display rich information about BSV basket types.

### Components

1. **BasketMapTopicManager** - Validates basket registrations with signature verification
2. **BasketMapLookupService** - Provides basket metadata resolution
3. **BasketMapStorage** - MongoDB-based storage for registration indexing

## Protocol Rules

Each valid output must satisfy the following:

1. It is a BRC-48 Pay-to-Push-Drop output
2. Contains exactly 7 fields:
   - **Basket ID**: UTF-8 basket type identifier string
   - **Name**: UTF-8 basket display name
   - **Icon URL**: UTF-8 URL to basket icon
   - **Description**: UTF-8 basket description
   - **Documentation URL**: UTF-8 URL to basket documentation
   - **Registry Operator**: UTF-8 identity key of the registry operator
   - **Signature**: ECDSA signature over fields 0-5
3. All text fields (basketID, name, iconURL, description, documentationURL, registryOperator) must be non-empty

**Note**: The current implementation validates field structure but does not verify signatures or locking key derivation. Full signature verification (as in TypeScript) is pending implementation.

## Usage

### Topic Manager

```go
import "github.com/bsv-blockchain/go-overlay-fiber/example/services/basketmap"

// Create topic manager
tm := basketmap.NewBasketMapTopicManager()

// Use with overlay engine
overlayServer.ConfigureTopicManager("tm_basketmap", tm)
```

### Lookup Service

```go
import (
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/basketmap"
    "go.mongodb.org/mongo-driver/mongo"
)

// Create lookup service (requires MongoDB)
ls := basketmap.NewBasketMapLookupService(mongoDatabase)

// Use with overlay engine
overlayServer.ConfigureLookupService("ls_basketmap", ls)
```

## API

### Topic Manager Methods

- `IdentifyAdmissibleOutputs(beef, previousCoins)` - Validates PushDrop registrations with field validation
- `GetDocumentation()` - Returns topic manager documentation
- `GetMetaData()` - Returns topic manager metadata

### Lookup Service Methods

- `OutputAdmittedByTopic(payload)` - Extracts and stores basket registration
- `OutputSpent(payload)` - Removes spent registrations from index
- `OutputEvicted(outpoint)` - Removes evicted registrations from index
- `Lookup(question)` - Queries stored registrations

### Query Parameters

```json
{
  "basketID": "basket-identifier (optional - string)",
  "name": "basket-name (optional - string, supports fuzzy search)",
  "registryOperators": ["operator1", "operator2"]
}
```

**Note**: Must specify EITHER (`basketID` AND `registryOperators`) OR (`name` AND `registryOperators`).

## Example Lookup Queries

### Find Basket by ID

```json
{
  "service": "ls_basketmap",
  "query": {
    "basketID": "my-basket",
    "registryOperators": ["operator1", "operator2"]
  }
}
```

### Find Basket by Name (Fuzzy Search)

```json
{
  "service": "ls_basketmap",
  "query": {
    "name": "payment",
    "registryOperators": ["operator1"]
  }
}
```

**Note**: Name search supports fuzzy matching. Searching for "pay" will match "payment", "payday", "repayment", etc.

### Find with Multiple Registry Operators

```json
{
  "service": "ls_basketmap",
  "query": {
    "basketID": "loyalty-basket",
    "registryOperators": [
      "operator1-identity-key",
      "operator2-identity-key",
      "operator3-identity-key"
    ]
  }
}
```

## Storage

The service uses MongoDB with the following schema:

```go
type BasketMapRecord struct {
    Txid         string                // Transaction ID
    OutputIndex  int                   // Output index
    Registration BasketMapRegistration // Registration details
    CreatedAt    time.Time             // Timestamp
}

type BasketMapRegistration struct {
    BasketID         string // Basket type identifier
    Name             string // Display name
    RegistryOperator string // Identity key of operator
}
```

### Query Response

Returns array of UTXO references:

```json
[
  { "txid": "abc...", "outputIndex": 0 },
  { "txid": "def...", "outputIndex": 1 }
]
```

## Protocol Specification

**Topic Name**: `tm_basketmap`

**Lookup Service**: `ls_basketmap`

**Encoding**: BRC-48 Pay-to-Push-Drop

**Validation**:
- PushDrop structure must be valid
- Exactly 7 fields (6 data + signature)
- All text fields must be non-empty UTF-8 strings
- Signature field must be present

**Query Responses**:
- UTXO references (txid + outputIndex)
- Can filter by basketID or name (fuzzy) with registryOperators
- Multiple registrations can exist for same basket (different operators)

**Spend Handling**:
- When a UTXO is spent, the registration is deleted
- Registry operators can update by creating new registrations

## Use Case Flow

1. **Registration Creation**:
   - Registry operator creates PushDrop output with basket metadata:
     - Basket ID: "loyalty-points"
     - Name: "Loyalty Points Basket"
     - Icon URL: "https://cdn.example.com/icons/loyalty.png"
     - Description: "A basket for storing customer loyalty points"
     - Documentation URL: "https://docs.example.com/loyalty-basket"
     - Registry Operator: "operator-identity-key"
     - Signature over all above data

2. **Validation**:
   - Topic manager decodes PushDrop
   - Verifies exactly 7 fields present
   - Verifies all text fields are non-empty
   - Verifies signature field is present
   - Admits output if all checks pass

3. **Indexing**:
   - Lookup service extracts registration data
   - Stores mapping:
     - basketID: "loyalty-points"
     - name: "Loyalty Points Basket"
     - registryOperator: "operator-identity-key"
     - txid/outputIndex
     - createdAt timestamp

4. **Discovery**:
   - Wallet application wants to display info for basket "loyalty-points"
   - Wallet queries ls_basketmap with basketID and trusted registryOperators
   - Gets UTXO references pointing to basket registrations
   - Wallet retrieves full outputs to display name, icon, description

5. **UX Enrichment**:
   - Application shows "Loyalty Points Basket" instead of raw basket ID
   - Displays icon from CDN URL
   - Links to documentation for user education

6. **Fuzzy Search**:
   - User searches for baskets containing "loyalty"
   - Query finds "Loyalty Points Basket", "Customer Loyalty", etc.
   - Helps users discover basket types

7. **Update**:
   - Registry operator can create new registrations without spending old ones
   - Multiple operators can register same basket (trust model)
   - Applications choose which operators to trust

8. **Removal**:
   - Spending a UTXO removes that registration
   - Allows operators to revoke outdated basket information

## Security Considerations

### Registry Operator Trust

- **Critical**: Only query registrations from trusted registry operators
- Applications should maintain allowlist of trusted operator identity keys
- Malicious operators could register false basket information
- Query multiple operators and compare results for consensus

### Signature Verification (Pending)

The TypeScript implementation verifies:
1. Locking public key is correctly derived from registryOperator
2. Signature is valid over fields 0-5
3. Links registration to specific operator identity

This verification is pending implementation in the Go version.

### Fuzzy Search Security

- Fuzzy name search uses regex matching
- May return more results than exact match
- Applications should validate results match expected basket types
- Consider displaying registry operator to users for verification

## TypeScript Equivalent

This is a port of the BasketMap service from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/main/src/services/basketmap).

## Related Standards

- **BRC-48**: Pay-to-Push-Drop (The encoding standard used for BasketMap registrations)
- **BRC-22**: Overlay Services (Data synchronization protocol)
- **BRC-24**: Overlay Lookup Services
