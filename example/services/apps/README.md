# Apps Example Service

A protocol for publishing Metanet App catalog entries on-chain, enabling decentralized app discovery and distribution.

## Overview

The Apps service implements a complete application catalog system using PushDrop tokens. Publishers can advertise their applications on-chain with rich metadata, which can then be queried by domain, publisher, name, tags, category, or directly by outpoint.

**Topic Manager**: `tm_apps`
**Lookup Service**: `ls_apps`
**Complexity**: Medium

## Protocol Rules

### App Token Structure

App tokens use PushDrop format with exactly 2 fields:
- **Field 0**: JSON-encoded PublishedAppMetadata
- **Field 1**: BRC-48 signature over the metadata

### Required Metadata Fields

Each app listing must include:
- **version**: Protocol version (currently "0.1.0")
- **name**: Application name
- **description**: Application description
- **icon**: Icon URL or UHRP reference
- **domain**: Primary domain for the app
- **publisher**: Identity key of the publisher
- **release_date**: ISO-8601 formatted release date
- At least one of:
  - **httpURL**: HTTP(S) URL where the app is hosted
  - **uhrpURL**: UHRP reference for the app

### Optional Metadata Fields

- **short_name**: Abbreviated app name
- **category**: Application category
- **tags**: Array of searchable tags
- **changelog**: Release notes
- **banner_image_url**: Banner image URL
- **screenshot_urls**: Array of screenshot URLs

### Admissibility Rules

The topic manager validates that:
1. Output can be decoded as PushDrop with exactly 2 fields
2. Field 0 contains valid JSON app metadata
3. All required fields are present and non-empty
4. At least one URL type (httpURL or uhrpURL) is provided

### Signature Verification

The protocol uses BRC-48 signatures with protocol ID `[1, 'metanet apps']` to verify:
- The signature is valid for the claimed publisher identity key
- The locking public key matches the expected derived child key

Note: Signature verification is pending full wallet infrastructure in go-sdk.

## Query Types

The lookup service supports multiple query patterns:

### 1. Domain Lookup
```go
Query: {
    "domain": "myapp.com"
}
```
Returns all apps published for the specified domain.

### 2. Publisher Lookup
```go
Query: {
    "publisher": "publisher_identity_key"
}
```
Returns all apps from a specific publisher.

### 3. Tag Lookup
```go
Query: {
    "tags": ["productivity", "finance"]
}
```
Returns apps matching any of the specified tags.

### 4. Category Lookup
```go
Query: {
    "category": "Games"
}
```
Returns all apps in the specified category.

### 5. Name Fuzzy Search
```go
Query: {
    "name": "Calculator"
}
```
Fuzzy-matches apps by name (case-insensitive).

### 6. Outpoint Lookup
```go
Query: {
    "outpoint": "txid.outputIndex"
}
```
Returns the specific app at the given UTXO reference.

### 7. Catalog Browse
```go
Query: {
    "limit": 20,
    "skip": 0,
    "sortOrder": "desc"
}
```
Returns all apps with pagination and sorting.

## Pagination and Sorting

All queries support:
- **limit**: Maximum results to return (default: 50)
- **skip**: Number of results to skip for pagination (default: 0)
- **sortOrder**: Sort direction - 'asc' or 'desc' (default: 'desc')

Results are sorted by `release_date` for chronological ordering.

## Storage Schema

### MongoDB Collection: `appsCatalogRecords`

```go
type AppCatalogRecord struct {
    Txid        string                `bson:"txid"`
    OutputIndex int                   `bson:"outputIndex"`
    Metadata    *PublishedAppMetadata `bson:"metadata"`
    CreatedAt   time.Time             `bson:"createdAt"`
}
```

### Indexes
- Full-text index on `metadata.name`, `metadata.description`, `metadata.tags`, `metadata.domain`

## Use Cases

### App Publishing
1. Publisher creates app metadata with all required fields
2. Metadata is encoded as PushDrop token with BRC-48 signature
3. Transaction is submitted to overlay network
4. Topic manager validates metadata structure
5. Lookup service indexes the app listing

### App Discovery
1. User queries by domain, tags, category, or name
2. Lookup service returns matching apps with pagination
3. Results include txid and outputIndex for UTXO references
4. Client can fetch full metadata from the blockchain

### App Updates
1. Publisher creates new version with updated metadata
2. Old UTXO is spent (removes old listing)
3. New UTXO is created (adds new listing)
4. Catalog reflects latest version

### Domain-Based Distribution
1. Domain owner publishes multiple apps under their domain
2. Users browse all apps from a trusted domain
3. Enables domain-based app ecosystems

## API Methods

### Topic Manager (tm_apps)

```go
func (tm *AppsTopicManager) IdentifyAdmissibleOutputs(
    ctx context.Context,
    beef []byte,
    previousCoins map[uint32]*transaction.TransactionOutput,
) (overlay.AdmittanceInstructions, error)
```

Validates app metadata and determines which outputs should be admitted to the overlay network.

### Lookup Service (ls_apps)

```go
func (ls *AppsLookupService) Lookup(
    ctx context.Context,
    question *lookup.LookupQuestion,
) (*lookup.LookupAnswer, error)
```

Queries app catalog based on the provided query parameters.

## Example Usage

```go
import (
    "context"
    "github.com/bsv-blockchain/go-overlay-fiber/example/services/apps"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
    "go.mongodb.org/mongo-driver/mongo"
)

func main() {
    // Create MongoDB client
    mongoClient, _ := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
    db := mongoClient.Database("overlay")

    // Create overlay server
    overlayServer := server.NewOverlayServer("mynode", privateKey, "https://myhost.com")

    // Configure Apps topic manager
    tm := apps.NewAppsTopicManager()
    overlayServer.ConfigureTopicManager("tm_apps", tm)

    // Configure Apps lookup service
    ls := apps.NewAppsLookupService(db)
    overlayServer.ConfigureLookupService("ls_apps", ls)

    // Start server
    overlayServer.Start()
}
```

### Querying with LookupResolver

```go
import "github.com/bsv-blockchain/go-sdk/overlay/lookup"

resolver := lookup.NewLookupResolver()

// Find by domain
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_apps",
    Query: map[string]interface{}{
        "domain": "example.com",
        "limit": 20,
    },
}, 10000)

// Find by name (fuzzy)
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_apps",
    Query: map[string]interface{}{
        "name": "Calculator",
        "sortOrder": "asc",
    },
}, 10000)

// Find by tags
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_apps",
    Query: map[string]interface{}{
        "tags": []string{"productivity", "tools"},
        "limit": 10,
    },
}, 10000)

// Browse all apps
response, err := resolver.Query(ctx, &lookup.LookupQuestion{
    Service: "ls_apps",
    Query: map[string]interface{}{
        "limit": 50,
        "skip": 0,
        "sortOrder": "desc",
    },
}, 10000)
```

### Publishing an App

```go
import (
    "encoding/json"
    "github.com/bsv-blockchain/go-sdk/transaction/template/pushdrop"
)

metadata := apps.PublishedAppMetadata{
    Version:     "0.1.0",
    Name:        "My Calculator",
    Description: "A simple calculator app",
    Icon:        "https://example.com/icon.png",
    HTTPURL:     "https://calculator.example.com",
    Domain:      "example.com",
    Publisher:   publisherIdentityKey,
    ReleaseDate: "2025-10-31T00:00:00Z",
    Category:    "Productivity",
    Tags:        []string{"calculator", "math", "tools"},
}

// Encode metadata as JSON
metadataJSON, _ := json.Marshal(metadata)

// Create PushDrop token with signature
// (Signature creation requires wallet infrastructure)
lockingScript := pushdrop.Encode([][]byte{metadataJSON, signature}, lockingPublicKey)

// Add output to transaction and broadcast
```

## Security Considerations

### Publisher Verification
- BRC-48 signatures link tokens to publisher identity keys
- Prevents impersonation of publishers
- Enables trust verification

### Domain Ownership
- Publishers must prove domain ownership through DNS or other means
- Not enforced on-chain but can be verified off-chain

### Content Filtering
- Overlay operators can choose which apps to index
- Additional filtering rules can be applied at lookup service level

### Version Control
- Spending old app UTXO removes outdated listings
- Only latest version remains discoverable
- Historical versions can be found via blockchain explorers

## Pending Features

The current implementation includes placeholders for:

1. **BRC-48 Signature Verification**: Verify signatures over metadata using ProtoWallet with protocol `[1, 'metanet apps']`
2. **Domain Verification**: Off-chain verification of domain ownership
3. **Content Moderation**: Optional filtering rules for app content

These features require additional infrastructure currently being developed in go-sdk.

## TypeScript Equivalent

This service is a port of the Apps example from [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples/tree/master/src/services/apps).

## References

- [BRC-48: PushDrop and Key Derivation](https://brc.dev/48)
- [Metanet Protocol](https://metanet.org)
- [PushDrop Protocol](https://github.com/bsv-blockchain/go-sdk/tree/master/transaction/template/pushdrop)
