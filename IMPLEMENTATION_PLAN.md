# Go Overlay Fiber Implementation Plan

## Overview
This document outlines the implementation plan for porting the exact functionality from `overlay-express` to Go using the Fiber web framework. Based on a comprehensive code review, the current implementation provides an excellent foundation but is missing most core functionality.

**Current Status: 2/5 - Good foundation, but most core functionality missing**

**Strengths:**
- Excellent architecture with clean fluent API design
- Good database configuration (SQL + MongoDB)
- Modern Go patterns with Fiber framework
- Proper error handling and middleware

**Critical Issues:**
1. **Engine integration incomplete** - missing storage, chain tracker, topic managers
2. **Core routes are placeholders** - `/submit`, `/lookup`, `/arc-ingest` don't work
3. **Missing configuration methods** - no topic managers, lookup services, chain tracker
4. **GASP sync not implemented** - all sync endpoints are placeholders

**Key Principles:**
- Focus on direct porting from overlay-express (1:1 mapping)
- Prioritize core functionality that exists in overlay-express
- Remove any planned features not present in the original
- Simplify phases to focus on direct porting
- Address dependency gaps (storage, engine, discovery services)

## Current Implementation Status

### What's Already Implemented (Foundation Complete)
1. **OverlayServer Struct**: Constructor with name, privateKey, advertisableFQDN ✅
2. **Basic Configuration Methods**: Port, logging, network, database connections ✅
3. **Database Integration**: SQL (MySQL/PostgreSQL/SQLite) and MongoDB ✅
4. **HTTP Server Setup**: Fiber app with middleware (CORS, logging, recovery) ✅
5. **Route Structure**: All 13 routes defined but most are placeholders ✅
6. **Admin Authentication**: Bearer token middleware implemented ✅
7. **Error Handling**: Proper error response format ✅

### Critical Missing Components (From Code Review)
1. **Storage Layer**: No KnexStorage equivalent integration
2. **Topic Managers**: Configuration methods missing, no SHIP/SLAP setup
3. **Lookup Services**: Configuration methods missing, no service integration
4. **Chain Tracker**: No WhatsOnChain or chain tracking integration
5. **TaggedBEEF Processing**: Submit endpoint is placeholder
6. **Engine Integration**: Engine created but not properly configured with services
7. **Discovery Services**: No SHIP/SLAP topic managers or lookup services
8. **Web UI**: No makeUserInterface equivalent, just JSON status
9. **Auto-configuration**: No automatic SHIP/SLAP service setup

### Dependencies Status

**Available (Already in go.mod):**
- `github.com/bsv-blockchain/go-overlay-services v0.1.1` - Engine available ✅
- `github.com/bsv-blockchain/go-sdk v1.2.1` - BSV SDK functionality ✅
- Database drivers (MySQL, PostgreSQL, SQLite, MongoDB) ✅
- Fiber web framework and middleware ✅

**Missing Integration (Available but not used):**
- Storage layer from go-overlay-services
- Topic managers and lookup services from go-overlay-services
- Chain tracker from go-sdk
- TaggedBEEF processing from go-sdk
- SHIP/SLAP discovery services

**TypeScript to Go Mapping (Confirmed Available):**
- Engine → `github.com/bsv-blockchain/go-overlay-services/pkg/core/engine`
- TaggedBEEF → `github.com/bsv-blockchain/go-sdk` 
- ChainTracker/WhatsOnChain → `github.com/bsv-blockchain/go-sdk`
- ARC → `github.com/bsv-blockchain/go-sdk`
- SHIP/SLAP services → `github.com/bsv-blockchain/go-overlay-services`

## Missing Configuration Methods (Critical Gap)

The current implementation is missing these essential configuration methods from overlay-express:

**Topic Manager Configuration:**
- `ConfigureTopicManager(name string, manager interface{})`
- Auto-configuration of SHIP/SLAP topic managers

**Lookup Service Configuration:**
- `ConfigureLookupService(name string, service interface{})`
- `ConfigureLookupServiceWithKnex(name string, factory func(*sql.DB) (interface{}, []Migration))`
- `ConfigureLookupServiceWithMongo(name string, factory func(*mongo.Database) interface{})`

**Chain Tracker Configuration:**
- `ConfigureChainTracker(chainTracker interface{})`
- Integration with WhatsOnChain equivalent

**Engine Parameters:**
- `ConfigureEngineParams(params EngineConfig)`
- Advanced engine configuration options

**Web UI Configuration:**
- `ConfigureWebUI(config UIConfig)`
- Static HTML interface generation

**Storage Integration:**
- Engine storage configuration with KnexStorage equivalent
- Migration handling

## Project Structure (Minimal)
```
go-overlay-fiber/
├── main.go                 # Single file implementation
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
└── IMPLEMENTATION_PLAN.md  # This file
```

### Core Components

## Core Structure (Direct Port from OverlayExpress.ts)

### OverlayExpress Struct
```go
// Direct port of OverlayExpress class properties
type OverlayExpress struct {
    // Required constructor parameters (exact match)
    Name               string
    PrivateKey         string
    AdvertisableFQDN   string
    adminToken         string  // private, generated if not provided
    
    // Configuration properties (exact match from TS)
    App                *fiber.App
    Port               int                    // default: 3000
    Logger             interface{}            // default: console equivalent
    Knex               *sql.DB                // SQL database connection
    MigrationsToRun    []Migration           // migrations array
    MongoDb            *mongo.Database        // MongoDB database
    Network            string                // "main" or "test", default: "main"
    ChainTracker       interface{}           // ChainTracker or "scripts only"
    Engine             interface{}           // Overlay Engine (from go-overlay-services)
    Managers           map[string]interface{} // Topic Managers
    Services           map[string]interface{} // Lookup Services
    EnableGASPSync     bool                  // default: true
    ArcApiKey          string                // optional ARC API key
    VerboseRequestLogging bool               // default: false
    WebUIConfig        UIConfig              // Web UI configuration
    EngineConfig       EngineConfig          // Advanced engine parameters
}

// Supporting types (port from TypeScript)
type Migration struct {
    Name string
    Up   func(db *sql.DB) error
    Down func(db *sql.DB) error  // optional
}

type UIConfig struct {
    Host                     string
    FaviconUrl              string
    BackgroundColor         string
    PrimaryColor            string
    SecondaryColor          string
    FontFamily              string
    HeadingFontFamily       string
    AdditionalStyles        string
    SectionBackgroundColor  string
    PrimaryTextColor        string
    LinkColor               string
    HoverColor              string
    BorderColor             string
    SecondaryBackgroundColor string
    SecondaryTextColor      string
    DefaultContent          string
}

type EngineConfig struct {
    ChainTracker                    interface{}
    ShipTrackers                   []string
    SlapTrackers                   []string
    Broadcaster                    interface{}
    Advertiser                     interface{}
    SyncConfiguration              map[string]interface{} // string[] | "SHIP" | false
    LogTime                        *bool
    LogPrefix                      string
    ThrowOnBroadcastFailure        *bool
    OverlayBroadcastFacilitator    interface{}
    SuppressDefaultSyncAdvertisements *bool
}
```

### Configuration Methods (Exact Port from OverlayExpress.ts)

**Required methods (exact match from TypeScript):**
- `ConfigurePort(port int)`
- `ConfigureWebUI(config UIConfig)`
- `ConfigureLogger(logger interface{})`
- `ConfigureNetwork(network string)` // "main" or "test"
- `ConfigureChainTracker(chainTracker interface{})`
- `ConfigureArcApiKey(apiKey string)`
- `configureEnableGASPSync(enable bool)`
- `ConfigureVerboseRequestLogging(enable bool)`
- `ConfigureKnex(config interface{})` // Knex.Config or connection string
- `ConfigureMongo(connectionString string)`
- `ConfigureTopicManager(name string, manager interface{})`
- `ConfigureLookupService(name string, service interface{})`
- `ConfigureLookupServiceWithKnex(name string, serviceFactory func(*sql.DB) (interface{}, []Migration))`
- `ConfigureLookupServiceWithMongo(name string, serviceFactory func(*mongo.Database) interface{})`
- `ConfigureEngineParams(params EngineConfig)`
- `ConfigureEngine(autoConfigureShipSlap bool)` // default: true
- `GetAdminToken() string`

### HTTP Routes (Exact Port from OverlayExpress.ts)

**Public Routes (11 total):**
1. `GET /` - Serve static HTML UI (calls makeUserInterface)
2. `GET /listTopicManagers` - Call engine.listTopicManagers()
3. `GET /listLookupServiceProviders` - Call engine.listLookupServiceProviders()
4. `GET /getDocumentationForTopicManager?manager=X` - Call engine.getDocumentationForTopicManager()
5. `GET /getDocumentationForLookupServiceProvider?lookupService=X` - Call engine.getDocumentationForLookupServiceProvider()
6. `POST /submit` - Parse x-topics header, construct TaggedBEEF, call engine.submit()
7. `POST /lookup` - Call engine.lookup(req.body)
8. `POST /arc-ingest` - Only if arcApiKey set, parse merklePath, call engine.handleNewMerkleProof()

**GASP Sync Routes (if enableGASPSync is true):**
9. `POST /requestSyncResponse` - Parse x-bsv-topic header, call engine.provideForeignSyncResponse()
10. `POST /requestForeignGASPNode` - Parse body {graphID, txid, outputIndex}, call engine.provideForeignGASPNode()

**Admin Routes (Bearer token protected, 3 total):**
11. `POST /admin/syncAdvertisements` - Call engine.syncAdvertisements()
12. `POST /admin/startGASPSync` - Call engine.startGASPSync()
13. `POST /admin/evictOutpoint` - Call service.outputEvicted() or all services

**404 Handler:**
- Return JSON error for unmatched routes

**Middleware (exact port):**
- Body parser (JSON 1GB limit, raw octet-stream 1GB)
- Verbose request logging (if enabled)
- CORS headers (Access-Control-Allow-*)
- Admin auth middleware (Bearer token check)

### Service Integration (Auto-configure from OverlayExpress.ts)

**Auto-configuration in configureEngine() (if autoConfigureShipSlap = true):**
```go
// Auto-configure SHIP and SLAP services (lines 358-366 in TS)
oe.ConfigureTopicManager("tm_ship", NewSHIPTopicManager())  // from go-overlay-services
oe.ConfigureTopicManager("tm_slap", NewSLAPTopicManager())  // from go-overlay-services
oe.ConfigureLookupServiceWithMongo("ls_ship", func(db *mongo.Database) interface{} {
    return NewSHIPLookupService(NewSHIPStorage(db))  // from go-overlay-services
})
oe.ConfigureLookupServiceWithMongo("ls_slap", func(db *mongo.Database) interface{} {
    return NewSLAPLookupService(NewSLAPStorage(db))  // from go-overlay-services
})
```

**Engine Creation (lines 415-453 in TS):**
- Create KnexStorage (SQL) from go-overlay-services
- Include KnexStorageMigrations
- Configure broadcaster (ARC if apiKey provided)
- Configure advertiser (WalletAdvertiser from go-overlay-services)
- Build sync configuration based on enableGASPSync
- Create Engine with all parameters

## Simplified Implementation Phases (Direct Porting Focus)

### Phase 1: Complete Missing Configuration Methods ⚡ PRIORITY
**Goal: Add all missing configuration methods from overlay-express**
1. `ConfigureTopicManager(name string, manager interface{})` 
2. `ConfigureLookupService(name string, service interface{})`
3. `ConfigureLookupServiceWithKnex(name string, factory)` 
4. `ConfigureLookupServiceWithMongo(name string, factory)`
5. `ConfigureChainTracker(chainTracker interface{})`
6. `ConfigureEngineParams(params EngineConfig)`
7. `ConfigureWebUI(config UIConfig)`
8. `GetAdminToken() string` method

### Phase 2: Integrate Storage Layer ⚡ PRIORITY 
**Goal: Replace placeholder engine integration with real storage**
1. Import and integrate KnexStorage equivalent from go-overlay-services
2. Configure Engine with proper storage (SQL + MongoDB)
3. Handle migrations like overlay-express KnexStorageMigrations
4. Connect database connections to Engine storage

### Phase 3: Implement Core Route Functionality ⚡ PRIORITY
**Goal: Replace placeholder routes with real functionality**
1. `/submit` - TaggedBEEF processing using go-sdk
2. `/lookup` - Engine.lookup() integration 
3. `/arc-ingest` - MerklePath processing and Engine.handleNewMerkleProof()
4. `/listTopicManagers` - Engine.listTopicManagers()
5. `/listLookupServiceProviders` - Engine.listLookupServiceProviders()
6. Documentation endpoints using Engine methods

### Phase 4: Auto-configure SHIP/SLAP Services
**Goal: Implement auto-configuration like overlay-express**
1. Auto-configure SHIP topic manager and lookup service
2. Auto-configure SLAP topic manager and lookup service  
3. Connect to MongoDB storage for SHIP/SLAP
4. Import discovery services from go-overlay-services

### Phase 5: Complete GASP Sync Implementation
**Goal: Implement sync functionality**
1. `/requestSyncResponse` - Engine.provideForeignSyncResponse()
2. `/requestForeignGASPNode` - Engine.provideForeignGASPNode()
3. Admin sync endpoints (syncAdvertisements, startGASPSync)
4. Configure sync settings in Engine

### Phase 6: Implement Web UI (makeUserInterface Port)
**Goal: Replace JSON status with HTML interface**
1. Port UIConfig struct with all styling options
2. Generate HTML interface with JavaScript like overlay-express
3. Embed static content for documentation interface
4. Support custom styling and branding

### Phase 7: Chain Tracker Integration
**Goal: Add WhatsOnChain equivalent functionality**
1. Import chain tracker from go-sdk
2. Configure with Engine like overlay-express
3. Handle "scripts only" mode
4. Integrate with ARC broadcaster

## Implementation Roadmap by Priority

### 🔥 Critical Blockers (Phase 1-3)
**Must be completed to reach functional parity**

1. **Missing Configuration Methods** - No topic managers or lookup services can be configured
2. **Storage Integration** - Engine has no real storage, just placeholder
3. **Core Route Implementation** - `/submit`, `/lookup`, `/arc-ingest` are placeholders
4. **TaggedBEEF Processing** - Core overlay functionality missing
5. **Chain Tracker Integration** - No blockchain interaction capability

### ⚠️ Important Features (Phase 4-5)  
**Required for full overlay-express compatibility**

6. **SHIP/SLAP Auto-configuration** - Discovery services setup
7. **GASP Sync Implementation** - Peer synchronization
8. **Admin Functionality** - Advertisement sync and management

### ✨ Polish Features (Phase 6-7)
**Nice-to-have for complete experience**

9. **Web UI Interface** - HTML documentation interface
10. **Advanced Engine Configuration** - Full parameter support

### Package Import Strategy
```go
// Already available and confirmed working:
"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"  // ✅ Engine
"github.com/bsv-blockchain/go-sdk"                              // ✅ TaggedBEEF, ARC, ChainTracker

// Need to explore and integrate:
"github.com/bsv-blockchain/go-overlay-services/pkg/storage"     // Storage layer
"github.com/bsv-blockchain/go-overlay-services/pkg/interfaces" // TopicManager, LookupService
"github.com/bsv-blockchain/go-overlay-services/pkg/discovery"   // SHIP/SLAP services
```

## Usage Pattern (Exact Port from OverlayExpress.ts)

### Constructor and Configuration (Mirror TypeScript API)
```go
// Constructor (exact match)
overlay := NewOverlayExpress("MyOverlayService", privateKey, "example.com", adminToken)

// Fluent configuration (exact method names from TS)
overlay.ConfigurePort(3000)
overlay.ConfigureNetwork("test")  // "main" or "test"
overlay.ConfigureKnex("mysql://user:pass@localhost/overlay")
amenow.ConfigureMongo("mongodb://localhost:27017")
overlay.ConfigureVerboseRequestLogging(true)
overlay.ConfigureEnableGASPSync(true)
overlay.ConfigureArcApiKey("your-arc-api-key")

// Configure web UI (optional)
overlay.ConfigureWebUI(UIConfig{
    Host: "https://example.com",
    PrimaryColor: "#3b6efb",
    BackgroundColor: "#191919",
})

// Configure engine (auto-configure SHIP/SLAP by default)
overlay.ConfigureEngine(true)  // true = auto-configure SHIP/SLAP

// Start server (runs migrations, syncs, starts listening)
overlay.Start()
```

### Manual Service Configuration (Advanced)
```go
// Manual topic manager and lookup service configuration
overlay.ConfigureTopicManager("my_custom_tm", customTopicManager)
overlay.ConfigureLookupService("my_custom_ls", customLookupService)

// With database-specific factories
overlay.ConfigureLookupServiceWithKnex("sql_service", func(db *sql.DB) (interface{}, []Migration) {
    return NewCustomService(db), []Migration{/* migrations */}
})

overlay.ConfigureLookupServiceWithMongo("mongo_service", func(db *mongo.Database) interface{} {
    return NewCustomMongoService(db)
})

// Advanced engine parameters
overlay.ConfigureEngineParams(EngineConfig{
    LogTime: true,
    ThrowOnBroadcastFailure: true,
    SyncConfiguration: map[string]interface{}{
        "tm_ship": "SHIP",
        "tm_slap": false,
    },
})
```

## Error Handling (Port Exact Format from OverlayExpress.ts)

### Error Response Format (Lines 603-607, 735-739 in TS)
```go
type ErrorResponse struct {
    Status  string `json:"status"`           // Always "error"
    Message string `json:"message"`          // Error message
    Code    string `json:"code,omitempty"`   // Optional error code
}

// 404 handler format (lines 936-941 in TS)
type NotFoundError struct {
    Status      string `json:"status"`      // "error"
    Code        string `json:"code"`        // "ERR_ROUTE_NOT_FOUND"
    Description string `json:"description"` // "Route not found."
}
```

### Exact Error Handling Pattern
- Async IIFE pattern with nested error handling (as in TS)
- 400 status for business logic errors
- 500 status for unexpected errors
- Consistent JSON error format across all endpoints
- Console.error logging with chalk equivalent

## Testing Strategy (Future Enhancement)

**Note: overlay-express has "No tests implemented yet" in package.json**

### Suggested Testing (Post-Port)
- Configuration method testing
- HTTP endpoint testing with exact request/response validation
- Database integration testing
- Mock external dependencies (Engine, services)
- Full server lifecycle testing

## Migration Compatibility

### API Compatibility Requirements
- Exact HTTP endpoint paths and methods
- Identical request/response JSON formats
- Same error response structures
- Compatible admin token authentication
- Matching Web UI functionality

### Behavior Compatibility
- Same configuration method chain patterns
- Identical auto-configuration logic for SHIP/SLAP
- Same migration handling
- Matching sync and advertisement behavior

## Implementation Quality Assessment

**Structure (4/5): Excellent architectural foundation** ✅
- Clean fluent API design
- Proper separation of concerns
- Good error handling patterns
- Modern Go idioms

**Completeness (1/5): Most core functionality missing** ❌
- Engine integration incomplete
- Core routes are placeholders
- Missing configuration methods
- No storage layer integration

**Correctness (3/5): What exists is well-implemented** ⚠️
- Database connections work properly
- Middleware setup is correct
- Error responses follow overlay-express format
- Authentication middleware functional

**Dependencies (2/5): Key overlay dependencies not properly integrated** ⚠️
- go-overlay-services not fully utilized
- go-sdk features not implemented
- Storage layer missing
- Discovery services not configured

### Immediate Next Steps
1. **Explore go-overlay-services packages** to understand available exports
2. **Implement missing configuration methods** for topic managers and lookup services
3. **Integrate storage layer** to replace placeholder engine configuration
4. **Port TaggedBEEF processing** for the submit endpoint
5. **Add chain tracker integration** for blockchain functionality

## Success Criteria (Updated Based on Code Review)

### Immediate Goals (Phases 1-3)
1. **Core Route Functionality**: `/submit`, `/lookup`, `/arc-ingest` work like overlay-express
2. **Configuration Completeness**: All missing configuration methods implemented
3. **Storage Integration**: Engine properly configured with storage layer
4. **TaggedBEEF Processing**: Submit endpoint handles BSV transactions

### Full Parity Goals (Phases 4-7) 
5. **SHIP/SLAP Auto-configuration**: Discovery services work identically
6. **GASP Sync Compatibility**: Peer synchronization endpoints functional
7. **Web UI Compatibility**: HTML documentation interface like TypeScript
8. **Admin Compatibility**: All admin endpoints work with Bearer token auth

## Next Immediate Actions

### Phase 1 Tasks (Start Immediately)
1. **Explore go-overlay-services structure**:
   ```bash
   # Investigate available packages and exports
   go doc github.com/bsv-blockchain/go-overlay-services/pkg/storage
   go doc github.com/bsv-blockchain/go-overlay-services/pkg/interfaces  
   go doc github.com/bsv-blockchain/go-overlay-services/pkg/discovery
   ```

2. **Add missing configuration methods** to OverlayServer struct
3. **Integrate storage layer** with Engine configuration
4. **Import and use TaggedBEEF** from go-sdk for submit endpoint

### Testing Strategy
- **Integration testing** with real overlay-express for API compatibility
- **Database integration testing** for storage layer
- **Route compatibility testing** with existing clients
- **Performance benchmarking** against TypeScript version

### Migration Path
- **Drop-in replacement**: Existing overlay-express clients work without changes
- **Configuration compatibility**: Same fluent API patterns
- **Docker compatibility**: Same deployment patterns
- **Performance improvement**: Leverage Go's concurrent processing

This simplified plan focuses on **completing the missing core functionality** to achieve full parity with overlay-express, building on the excellent foundation that already exists.