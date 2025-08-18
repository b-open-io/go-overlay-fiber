# Go Overlay Fiber Implementation Plan

## Current Status
- **Phase 1**: ✅ **COMPLETE** - Core engine integration with SQLStorage implementation, Engine configuration, and auto-configuration functionality is working.
- **Phase 2**: ✅ **COMPLETE** - All core routes (`/submit`, `/lookup`, documentation endpoints, `/arc-ingest`) are implemented and functional.
- **Phase 3**: ⚠️ **MOSTLY COMPLETE** - Auto-configuration works for topic managers; lookup services need MongoDB to be fully functional.
- **Phase 4**: ✅ **COMPLETE** - All GASP sync endpoints and admin endpoints are fully implemented and functional.
- **Phase 5**: ✅ **COMPLETE** - Full HTML web interface with interactive documentation is implemented.

**Overall Project Status**: ~95% Complete - All major functionality implemented, only MongoDB auto-configuration for lookup services remains.

## Overview
This document outlines the complete implementation plan for achieving full feature parity with `overlay-express` in Go using the Fiber web framework. After comprehensive analysis of both codebases, this plan focuses on systematic implementation of missing components and functionality.

## Current State Analysis

### What Works ✅
- **Foundation Architecture**: OverlayServer struct with proper configuration methods
- **Database Integration**: SQL and MongoDB connections with health checks
- **HTTP Server**: Fiber app with middleware (CORS, logging, recovery, admin auth)
- **Route Structure**: All 13+ routes defined with proper middleware protection
- **Configuration Methods**: All configuration methods implemented (port, network, databases, services)
- **Engine Integration**: Engine properly configured with overlay storage and services
- **Core Route Logic**: All core routes (`/submit`, `/lookup`, documentation, `/arc-ingest`) working
- **Storage Layer**: Overlay storage with EventDataStorage interface implemented
- **Service Auto-configuration**: SHIP/SLAP topic managers auto-configured
- **TaggedBEEF Processing**: Submit endpoint processes BSV transactions via Engine
- **Web UI**: Full HTML interface with interactive documentation and real-time features
- **Chain Tracker Integration**: ARC integration for merkle proof handling

### Remaining Gaps ❌
- **Auto-configured Lookup Services**: Need MongoDB configuration to be fully functional

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
- **Service Configuration**: ✅ Complete - storage integration working
- **Engine Setup**: ✅ Complete - Engine configured with storage and services
- **Web UI**: ✅ Complete - full HTML interface with real-time features
- **Core Routes**: ✅ Complete - all core routes working (`/submit`, `/lookup`, docs, `/arc-ingest`)
- **GASP Sync**: ✅ Complete - all sync endpoints functional
- **Admin Endpoints**: ✅ Complete - all admin endpoints functional
- **Auto-configuration**: ⚠️ Mostly Complete - SHIP/SLAP topic managers work, lookup services need MongoDB
- **Chain Integration**: ✅ Complete - ARC integration for merkle proofs working

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
   - [x] Parse x-topics header
   - [x] Process TaggedBEEF from request body
   - [x] Call Engine.Submit() with proper parameters
   - [x] Return transaction ID and status

2. **Lookup Endpoint (`/lookup`)** ✅ COMPLETE
   - [x] Parse lookup request body
   - [x] Call Engine.lookup() with request parameters
   - [x] Return lookup results in overlay-express format

3. **Documentation Endpoints** ✅ COMPLETE
   - [x] `/listTopicManagers` → Engine.listTopicManagers()
   - [x] `/listLookupServiceProviders` → Engine.listLookupServiceProviders()
   - [x] `/getDocumentationForTopicManager` → Engine.getDocumentationForTopicManager()
   - [x] `/getDocumentationForLookupServiceProvider` → Engine.getDocumentationForLookupServiceProvider()

4. **ARC Integration (`/arc-ingest`)** ✅ COMPLETE
   - [x] Parse merklePath from request
   - [x] Call Engine.handleNewMerkleProof()
   - [x] Handle ARC webhook callbacks

**Deliverable**: ✅ Core overlay functionality working end-to-end

### Phase 3: Auto-Configuration (Priority 2) ✅ COMPLETE
**Goal**: Match overlay-express automatic service setup

**Tasks:**
1. **SHIP/SLAP Auto-Configuration** ✅ COMPLETE
   - [x] Auto-configure SHIP topic manager when ConfigureEngine() called
   - [x] Auto-configure SLAP topic manager when ConfigureEngine() called
   - [ ] Auto-configure SHIP lookup service with MongoDB
   - [ ] Auto-configure SLAP lookup service with MongoDB
   - [x] Match overlay-express configureEngine() logic exactly

2. **Migration Management** ✅ COMPLETE
   - [x] Implement migration runner for SQL schemas
   - [x] Include overlay service migrations (KnexStorageMigrations equivalent)
   - [x] Handle migration failures gracefully

3. **Sync Configuration** ✅ COMPLETE
   - [x] Configure GASP sync based on EnableGASPSync setting
   - [x] Set up sync configuration for auto-configured services
   - [x] Enable/disable sync per service as needed

**Deliverable**: ✅ Zero-config setup like overlay-express with automatic SHIP/SLAP (partial - lookup services need MongoDB config)

### Phase 4: GASP Sync Implementation (Priority 2) ✅ COMPLETE
**Goal**: Implement peer synchronization functionality

**Tasks:**
1. **Sync Response Endpoint (`/requestSyncResponse`)** ✅ COMPLETE
   - [x] Parse x-bsv-topic header
   - [x] Call Engine.ProvideForeignSyncResponse()
   - [x] Handle cross-node synchronization

2. **Foreign Node Endpoint (`/requestForeignGASPNode`)** ✅ COMPLETE
   - [x] Parse request body {graphID, txid, outputIndex}
   - [x] Call Engine.ProvideForeignGASPNode()
   - [x] Return node data for GASP network

3. **Admin Sync Endpoints** ✅ COMPLETE
   - [x] `/admin/syncAdvertisements` → Engine.SyncAdvertisements()
   - [x] `/admin/startGASPSync` → Engine.StartGASPSync()
   - [x] `/admin/evictOutpoint` → call outputEvicted on relevant services

**Deliverable**: ✅ Full GASP synchronization capability implemented and functional

### Phase 5: Web UI Implementation (Priority 3) ✅ COMPLETE
**Goal**: Replace JSON responses with HTML documentation interface

**Tasks:**
1. **HTML Interface Generation** ✅ COMPLETE
   - [x] Port makeUserInterface() logic from overlay-express
   - [x] Generate dynamic HTML based on configured services
   - [x] Include JavaScript for interactive documentation
   - [x] Support custom styling via UIConfig

2. **Documentation Integration** ✅ COMPLETE
   - [x] Show topic manager documentation in web interface
   - [x] Show lookup service documentation in web interface
   - [x] Display service status and health information
   - [x] Include API endpoint documentation

3. **Branding and Customization** ✅ COMPLETE
   - [x] Support UIConfig styling options
   - [x] Custom colors, fonts, favicon
   - [x] Additional custom CSS styles
   - [x] Configurable content sections

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

### Phase 2 Complete When: ✅ COMPLETE
- [x] `/submit` processes TaggedBEEF and returns transaction ID
- [x] `/lookup` executes queries and returns results
- [x] `/listTopicManagers` returns actual configured managers
- [x] `/listLookupServiceProviders` returns actual configured services
- [x] Documentation endpoints return real service documentation
- [x] `/arc-ingest` handles merkle proofs if ARC API key configured
- [x] All routes return proper JSON responses matching overlay-express format

### Phase 3 Complete When: ⚠️ MOSTLY COMPLETE
- [x] `ConfigureEngine(true)` auto-configures SHIP and SLAP services
- [x] Zero-config setup works like overlay-express example
- [x] Database migrations include overlay service schemas
- [ ] Auto-configured services are functional and show in documentation (lookup services need MongoDB)

### Phase 4 Complete When: ✅ COMPLETE
- [x] `/requestSyncResponse` handles GASP sync requests
- [x] `/requestForeignGASPNode` returns node data for GASP network
- [x] Admin endpoints (`/admin/*`) require Bearer token and work
- [x] GASP sync can be enabled/disabled via configuration

### Phase 5 Complete When: ✅ COMPLETE
- [x] Root endpoint (`/`) returns HTML documentation interface
- [x] Web UI shows configured services with interactive documentation
- [x] UIConfig styling options work (colors, fonts, custom CSS)
- [x] Interface matches overlay-express appearance and functionality

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
- ✅ HTTP endpoints match exactly (implemented)
- ✅ Request/response JSON formats match (error handling implemented)
- ✅ Core endpoint functionality working (`/submit`, `/lookup`, docs, `/arc-ingest`)
- ✅ HTML web interface with interactive features
- ✅ Admin authentication matches (Bearer token implemented)
- ✅ **Complete**: GASP sync endpoint functionality
- ✅ **Complete**: Admin endpoint functionality

### Drop-in Replacement Goals
- Same configuration API (`configurePort`, `configureMongo`, etc.)
- Same HTTP endpoints and responses  
- Same auto-configuration behavior
- Better performance through Go's concurrency
- Same deployment patterns (Docker, environment variables)

This implementation plan provides a clear, systematic approach to achieving full parity with overlay-express while maximizing reuse of existing Go overlay services and maintaining the excellent architectural foundation that already exists in the current codebase.
