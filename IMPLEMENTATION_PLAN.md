# Go Overlay Fiber Implementation Plan

## Current Status
Phase 1 is **COMPLETE** - Core engine integration with SQLStorage implementation, Engine configuration, and auto-configuration functionality is working.

## Overview
This document outlines the complete implementation plan for achieving full feature parity with `overlay-express` in Go using the Fiber web framework. After comprehensive analysis of both codebases, this plan focuses on systematic implementation of missing components and functionality.

## Current State Analysis

### What Works ✅
- **Foundation Architecture**: OverlayServer struct with proper configuration methods
- **Database Integration**: SQL and MongoDB connections with health checks
- **HTTP Server**: Fiber app with middleware (CORS, logging, recovery, admin auth)
- **Route Structure**: All 13+ routes defined with proper middleware protection
- **Configuration Methods**: Most basic config methods implemented (port, network, databases)

### Critical Gaps ❌
- **Engine Integration**: Engine created but not properly configured with storage/services
- **Core Route Logic**: All routes return placeholders instead of real functionality
- **Storage Layer**: Missing KnexStorage equivalent from go-overlay-services
- **Service Auto-configuration**: No SHIP/SLAP topic managers or lookup services
- **TaggedBEEF Processing**: Submit endpoint doesn't process BSV transactions
- **Web UI**: Returns JSON status instead of HTML interface like overlay-express
- **Chain Tracker Integration**: Not connected to Engine or WhatsOnChain equivalent

### Available Dependencies
- `github.com/bsv-blockchain/go-overlay-services v0.1.1` - Engine, storage, topic managers
- `github.com/bsv-blockchain/go-sdk v1.2.1` - TaggedBEEF, ChainTracker, ARC integration
- Database drivers and Fiber framework - All working

## Feature Parity Analysis

### Overlay-Express Core Features
1. **Constructor & Configuration**: `new OverlayExpress(name, privateKey, fqdn)`
2. **Database Setup**: `configureKnex()`, `configureMongo()`
3. **Service Configuration**: `configureTopicManager()`, `configureLookupService()`
4. **Engine Setup**: `configureEngine()` with auto SHIP/SLAP configuration
5. **Web UI**: `makeUserInterface()` generates HTML documentation interface
6. **Core Routes**: `/submit` (TaggedBEEF), `/lookup`, `/arc-ingest`, documentation endpoints
7. **GASP Sync**: `/requestSyncResponse`, `/requestForeignGASPNode` if enabled
8. **Admin Endpoints**: `/admin/syncAdvertisements`, `/admin/startGASPSync` with Bearer auth
9. **Auto-configuration**: Automatic SHIP/SLAP topic managers and lookup services
10. **Chain Integration**: WhatsOnChain for blockchain data, ARC for broadcasting

### Go Implementation Status
- **Constructor & Configuration**: ✅ Complete - matches overlay-express API
- **Database Setup**: ✅ Complete - supports same connection patterns
- **Service Configuration**: ✅ Methods exist but need storage integration
- **Engine Setup**: ⚠️  Partial - Engine created but missing storage/services
- **Web UI**: ❌ Missing - returns JSON instead of HTML interface
- **Core Routes**: ❌ Missing - all return placeholder responses
- **GASP Sync**: ❌ Missing - placeholder endpoints only
- **Admin Endpoints**: ❌ Missing - placeholder implementations
- **Auto-configuration**: ❌ Missing - no SHIP/SLAP service setup
- **Chain Integration**: ❌ Missing - ChainTracker not connected to Engine

## Implementation Strategy

### Reuse vs Build Analysis

**Available for Reuse (go-overlay-services v0.1.1):**
- Engine core functionality
- Storage interfaces and implementations
- Topic manager interfaces and SHIP/SLAP implementations
- Lookup service interfaces and implementations
- Advertisement and sync functionality

**Available for Reuse (go-sdk v1.2.1):**
- TaggedBEEF parsing and processing
- Chain tracking (WhatsOnChain equivalent)
- ARC integration for transaction broadcasting
- BSV transaction handling and validation

**Must Implement from Scratch:**
- Web UI HTML generation (makeUserInterface equivalent)
- Route handler logic connecting to Engine methods
- Auto-configuration logic for SHIP/SLAP services
- Migration handling and database schema setup
- Error handling and response formatting to match overlay-express

**Import Strategy:**
```go
// Core engine and interfaces
"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
"github.com/bsv-blockchain/go-overlay-services/pkg/interfaces"
"github.com/bsv-blockchain/go-overlay-services/pkg/storage"

// Discovery services (SHIP/SLAP)
"github.com/bsv-blockchain/go-overlay-services/pkg/discovery/ship"
"github.com/bsv-blockchain/go-overlay-services/pkg/discovery/slap"

// BSV blockchain integration
"github.com/bsv-blockchain/go-sdk/transaction"
"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
"github.com/bsv-blockchain/go-sdk/transaction/broadcaster"
```

## Implementation Phases

### Phase 1: Core Engine Integration ✅ COMPLETE
**Goal**: Replace placeholder engine with fully configured, working engine

**Completed Tasks:**
1. **Storage Integration** ✅
   - SQLStorage implementation from go-overlay-services integrated
   - SQL database connected to Engine storage
   - Database migrations implemented
   - MongoDB storage configured for lookup services

2. **Engine Configuration** ✅
   - Engine properly initialized with storage, managers, services
   - Broadcaster (ARC) and advertiser components configured
   - Chain tracker integration set up
   - EngineConfig parameters applied

3. **Service Registration** ✅
   - Topic managers connected to Engine
   - Lookup services connected to Engine
   - Service lifecycle management implemented

**Deliverable**: ✅ Engine properly initialized and connected to databases

### Phase 2: Core Route Implementation (Priority 1)
**Goal**: Implement actual functionality for core overlay operations

**Tasks:**
1. **Submit Endpoint (`/submit`)** ✅ COMPLETE
   - Parse x-topics header
   - Process TaggedBEEF from request body
   - Call Engine.Submit() with proper parameters
   - Return transaction ID and status

2. **Lookup Endpoint (`/lookup`)**
   - Parse lookup request body
   - Call Engine.lookup() with request parameters
   - Return lookup results in overlay-express format

3. **Documentation Endpoints**
   - `/listTopicManagers` → Engine.listTopicManagers()
   - `/listLookupServiceProviders` → Engine.listLookupServiceProviders()
   - `/getDocumentationForTopicManager` → Engine.getDocumentationForTopicManager()
   - `/getDocumentationForLookupServiceProvider` → Engine.getDocumentationForLookupServiceProvider()

4. **ARC Integration (`/arc-ingest`)**
   - Parse merklePath from request
   - Call Engine.handleNewMerkleProof()
   - Handle ARC webhook callbacks

**Deliverable**: Core overlay functionality working end-to-end

### Phase 3: Auto-Configuration (Priority 2)
**Goal**: Match overlay-express automatic service setup

**Tasks:**
1. **SHIP/SLAP Auto-Configuration**
   - Auto-configure SHIP topic manager when ConfigureEngine() called
   - Auto-configure SLAP topic manager when ConfigureEngine() called
   - Auto-configure SHIP lookup service with MongoDB
   - Auto-configure SLAP lookup service with MongoDB
   - Match overlay-express configureEngine() logic exactly

2. **Migration Management**
   - Implement migration runner for SQL schemas
   - Include overlay service migrations (KnexStorageMigrations equivalent)
   - Handle migration failures gracefully

3. **Sync Configuration**
   - Configure GASP sync based on EnableGASPSync setting
   - Set up sync configuration for auto-configured services
   - Enable/disable sync per service as needed

**Deliverable**: Zero-config setup like overlay-express with automatic SHIP/SLAP

### Phase 4: GASP Sync Implementation (Priority 2)
**Goal**: Implement peer synchronization functionality

**Tasks:**
1. **Sync Response Endpoint (`/requestSyncResponse`)**
   - Parse x-bsv-topic header
   - Call Engine.provideForeignSyncResponse()
   - Handle cross-node synchronization

2. **Foreign Node Endpoint (`/requestForeignGASPNode`)**
   - Parse request body {graphID, txid, outputIndex}
   - Call Engine.provideForeignGASPNode()
   - Return node data for GASP network

3. **Admin Sync Endpoints**
   - `/admin/syncAdvertisements` → Engine.syncAdvertisements()
   - `/admin/startGASPSync` → Engine.startGASPSync()
   - `/admin/evictOutpoint` → call outputEvicted on relevant services

**Deliverable**: Full GASP synchronization capability

### Phase 5: Web UI Implementation (Priority 3)
**Goal**: Replace JSON responses with HTML documentation interface

**Tasks:**
1. **HTML Interface Generation**
   - Port makeUserInterface() logic from overlay-express
   - Generate dynamic HTML based on configured services
   - Include JavaScript for interactive documentation
   - Support custom styling via UIConfig

2. **Documentation Integration**
   - Show topic manager documentation in web interface
   - Show lookup service documentation in web interface
   - Display service status and health information
   - Include API endpoint documentation

3. **Branding and Customization**
   - Support UIConfig styling options
   - Custom colors, fonts, favicon
   - Additional custom CSS styles
   - Configurable content sections

**Deliverable**: Full-featured web documentation interface like overlay-express

## Technical Implementation Details

### Package Structure (Maintain Current)
```
go-overlay-fiber/
├── examples/basic/         # Example usage
├── pkg/server/            # Core server implementation
│   ├── server.go          # Main OverlayServer struct and methods
│   └── interfaces.go      # Type definitions and interfaces
├── go.mod                 # Dependencies
├── go.sum                 # Dependency checksums
└── IMPLEMENTATION_PLAN.md # This document
```

### Key Implementation Patterns

**1. Storage Integration Pattern**
```go
// Phase 1: Configure Engine with proper storage
func (s *OverlayServer) ConfigureEngine(autoConfigureShipSlap bool) error {
    // Create KnexStorage equivalent from go-overlay-services
    storage, err := storage.NewKnexStorage(s.DB, s.MongoDB)
    if err != nil {
        return err
    }
    
    // Create Engine with storage
    engineConfig := &engine.Config{
        Storage: storage,
        HostingURL: s.AdvertisableFQDN,
        // ... other config
    }
    
    s.Engine, err = engine.New(engineConfig)
    if err != nil {
        return err
    }
    
    // Auto-configure SHIP/SLAP if requested (like overlay-express)
    if autoConfigureShipSlap {
        s.autoConfigureDiscoveryServices()
    }
    
    return nil
}
```

**2. Route Implementation Pattern**
```go
// Phase 2: Implement actual route functionality
func (s *OverlayServer) handleSubmit(c *fiber.Ctx) error {
    // Parse x-topics header like overlay-express
    topics := c.Get("x-topics")
    if topics == "" {
        return c.Status(400).JSON(ErrorResponse{
            Status: "error",
            Message: "x-topics header required",
        })
    }
    
    // Parse TaggedBEEF from body
    taggedBEEF, err := transaction.ParseTaggedBEEF(c.Body())
    if err != nil {
        return c.Status(400).JSON(ErrorResponse{
            Status: "error", 
            Message: "Invalid TaggedBEEF: " + err.Error(),
        })
    }
    
    // Call Engine.submit() like overlay-express
    result, err := s.Engine.Submit(taggedBEEF, topics)
    if err != nil {
        return c.Status(500).JSON(ErrorResponse{
            Status: "error",
            Message: err.Error(),
        })
    }
    
    return c.JSON(result)
}
```

**3. Auto-Configuration Pattern**
```go
// Phase 3: Auto-configure SHIP/SLAP like overlay-express
func (s *OverlayServer) autoConfigureDiscoveryServices() error {
    // Auto-configure SHIP topic manager
    shipTM, err := ship.NewTopicManager()
    if err != nil {
        return err
    }
    s.ConfigureTopicManager("tm_ship", shipTM)
    
    // Auto-configure SHIP lookup service with MongoDB
    s.ConfigureLookupServiceWithMongo("ls_ship", func(db *mongo.Database) (engine.LookupService, error) {
        storage := ship.NewMongoStorage(db)
        return ship.NewLookupService(storage), nil
    })
    
    // Same for SLAP...
    return nil
}
```

## Success Criteria

### Phase 1 Complete When:
- [x] Engine initializes with proper storage (SQL + MongoDB)
- [x] Database migrations run successfully
- [x] Engine can be configured with topic managers and lookup services
- [x] Health check shows "engine_configured" status
- [x] No placeholder storage - real go-overlay-services storage integrated

### Phase 2 Complete When:
- [x] `/submit` processes TaggedBEEF and returns transaction ID
- [ ] `/lookup` executes queries and returns results
- [ ] `/listTopicManagers` returns actual configured managers
- [ ] `/listLookupServiceProviders` returns actual configured services
- [ ] Documentation endpoints return real service documentation
- [ ] `/arc-ingest` handles merkle proofs if ARC API key configured
- [ ] All routes return proper JSON responses matching overlay-express format

### Phase 3 Complete When:
- [ ] `ConfigureEngine(true)` auto-configures SHIP and SLAP services
- [ ] Zero-config setup works like overlay-express example
- [ ] Database migrations include overlay service schemas
- [ ] Auto-configured services are functional and show in documentation

### Phase 4 Complete When:
- [ ] `/requestSyncResponse` handles GASP sync requests
- [ ] `/requestForeignGASPNode` returns node data for GASP network
- [ ] Admin endpoints (`/admin/*`) require Bearer token and work
- [ ] GASP sync can be enabled/disabled via configuration

### Phase 5 Complete When:
- [ ] Root endpoint (`/`) returns HTML documentation interface
- [ ] Web UI shows configured services with interactive documentation
- [ ] UIConfig styling options work (colors, fonts, custom CSS)
- [ ] Interface matches overlay-express appearance and functionality

## Next Immediate Actions

### Week 1: Phase 1 - Storage Integration
1. **Explore go-overlay-services packages**
   ```bash
   go doc github.com/bsv-blockchain/go-overlay-services/pkg/storage
   go doc github.com/bsv-blockchain/go-overlay-services/pkg/interfaces
   ```

2. **Implement proper Engine configuration**
   - Replace placeholder `engine.NewEngine()` call with real storage
   - Configure KnexStorage equivalent with SQL database
   - Set up MongoDB storage for lookup services

3. **Add migration support**
   - Implement database migration runner
   - Include overlay service migrations

### Week 2: Phase 2 - Core Routes  
1. **Implement `/submit` endpoint**
   - Import TaggedBEEF parsing from go-sdk
   - Connect to Engine.submit() method
   - Handle x-topics header parsing

2. **Implement `/lookup` endpoint** 
   - Connect to Engine.lookup() method
   - Handle request/response JSON format

3. **Implement documentation endpoints**
   - Connect to Engine methods for service lists and docs

### Week 3-4: Phase 3 - Auto-Configuration
1. **Import SHIP/SLAP services**
   - Add discovery services from go-overlay-services
   - Implement auto-configuration logic
   - Test zero-config setup

### Testing Strategy
- **Integration testing** against real overlay-express for API compatibility
- **Database testing** with both SQL and MongoDB
- **End-to-end testing** of submit/lookup workflows
- **Performance testing** vs TypeScript implementation

## Migration Compatibility

### API Compatibility Requirements
- ✅ HTTP endpoints match exactly (already implemented)
- ✅ Request/response JSON formats match (error handling implemented)
- ❌ **Missing**: Actual endpoint functionality
- ❌ **Missing**: HTML web interface
- ✅ Admin authentication matches (Bearer token implemented)

### Drop-in Replacement Goals
- Same configuration API (`configurePort`, `configureMongo`, etc.)
- Same HTTP endpoints and responses  
- Same auto-configuration behavior
- Better performance through Go's concurrency
- Same deployment patterns (Docker, environment variables)

This implementation plan provides a clear, systematic approach to achieving full parity with overlay-express while maximizing reuse of existing Go overlay services and maintaining the excellent architectural foundation that already exists in the current codebase.
