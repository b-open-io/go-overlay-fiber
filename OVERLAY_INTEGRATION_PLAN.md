# Overlay Library Integration Plan

## Executive Summary

This plan outlines the integration of battle-tested components from the `../overlay` directory into the `go-overlay-fiber` project. The overlay library provides production-ready storage, BEEF management, and publishing capabilities with factory patterns for auto-detecting storage types. This integration will replace current basic implementations with robust, multi-backend solutions while maintaining backward compatibility.

## Current State Analysis

### What We Have ✅
- **Core Framework**: Complete Fiber-based HTTP server with all routes defined
- **Basic Engine Integration**: Phase 1 complete with SQLStorage implementation
- **Database Connections**: SQL and MongoDB with health checks
- **Submit Endpoint**: Functional TaggedBEEF processing (2/5 endpoints done)
- **Configuration API**: Full parity with overlay-express configuration methods

### What We Need ❌
- **Production Storage**: Current SQLStorage is basic, lacks advanced features
- **Multi-Backend Support**: No Redis, MongoDB, or filesystem storage options
- **BEEF Management**: No dedicated BEEF storage or caching layer
- **Event Publishing**: No pub/sub capabilities for real-time updates
- **Missing Endpoints**: `/lookup`, documentation, ARC integration need implementation
- **Web UI**: Returns JSON instead of HTML interface

### Overlay Library Assets

The `../overlay` directory contains:

1. **Storage Layer** (`storage/`):
   - Factory pattern with auto-detection: Redis, MongoDB, SQLite
   - EventDataStorage interface extending engine.Storage
   - Advanced event querying and lookup capabilities

2. **BEEF Management** (`beef/`):
   - Multiple storage backends: Redis (caching), MongoDB, SQLite, filesystem
   - Factory pattern for auto-detection from connection strings
   - Production-ready implementations with TTL support

3. **Publishing** (`publish/`):
   - Redis-based pub/sub for real-time event broadcasting
   - Publisher interface with configurable backends

4. **Processing** (`processor/`):
   - Queue-based transaction processing with Redis
   - Concurrent batch processing capabilities

5. **Lookup Infrastructure** (`lookup/events/`):
   - Event-based lookup system for complex queries

## Integration Strategy

### Phase 1: Storage Layer Replacement (Priority 1) ✅ COMPLETED
**Timeline**: 2-3 days  
**Goal**: Replace basic SQLStorage with overlay library's EventDataStorage

**Status**: ✅ Completed - Basic overlay storage integration working with environment variables

**Benefits**:
- Multi-backend support (Redis, MongoDB, SQLite)
- Advanced event querying capabilities
- Production-tested battle-hardened implementations
- Auto-detection from connection strings

**Tasks**:
1. **Import Overlay Storage** 
   - Add `github.com/b-open-io/overlay` to go.mod
   - Import storage factory and implementations

2. **Replace SQLStorage**
   - Create adapter that wraps overlay's EventDataStorage
   - Implement engine.Storage interface using overlay components
   - Maintain backward compatibility with existing data

3. **Add Configuration Support**
   - Support connection string auto-detection
   - Add environment variable support (EVENT_STORAGE, BEEF_STORAGE)
   - Enable Redis, MongoDB, SQLite backends

4. **Migration Path**
   - Create migration utilities to move existing data
   - Support gradual rollout with fallback options

**Risk Mitigation**:
- Keep existing SQLStorage as fallback during transition
- Comprehensive testing of all storage backends
- Data migration validation and rollback procedures

### Phase 2: BEEF and Publishing Integration (Priority 1) 
**Timeline**: 2-3 days  
**Goal**: Add production BEEF storage and event publishing

**Benefits**:
- Dedicated BEEF storage with caching (Redis) and persistence
- Real-time event publishing for live updates
- Scalable architecture supporting high-volume operations

**Tasks**:
1. **BEEF Storage Integration**
   - Import beef factory and implementations
   - Configure based on BEEF_STORAGE environment variable
   - Default to filesystem storage, support Redis caching

2. **Publisher Integration** 
   - Import Redis publisher for event broadcasting
   - Integrate with overlay storage for automatic event publishing
   - Configure pub/sub channels for different event types

3. **Update Engine Configuration**
   - Modify ConfigureEngine to use overlay components
   - Pass BEEF storage and publisher to EventDataStorage
   - Ensure all components are properly wired

**Technical Implementation**:
```go
// ConfigureEngine enhancement
func (s *OverlayServer) ConfigureEngine(autoConfigureShipSlap bool) *OverlayServer {
    // Create BEEF storage from environment or default
    beefStore, err := beef.CreateBeefStorage("")
    
    // Create publisher (Redis-based)
    publisher, err := publish.NewRedisPublisher(redisURL)
    
    // Create event data storage with all components
    storage, err := storage.CreateEventDataStorage("", beefStore, publisher)
    
    // Configure engine with overlay storage
    s.Engine = engine.NewEngine(engine.Engine{
        Storage: storage,
        // ... other config
    })
}
```

### Phase 3: Advanced Endpoints Implementation (Priority 2)
**Timeline**: 3-4 days
**Goal**: Implement missing endpoints using overlay capabilities

**Benefits**:
- Full API compatibility with overlay-express
- Advanced lookup capabilities with event querying
- ARC integration for blockchain data

**Missing Endpoints to Implement**:
1. **`/lookup` endpoint** - Complex event-based queries
2. **Documentation endpoints** - Service discovery and docs  
3. **`/arc-ingest`** - Merkle proof processing
4. **Admin endpoints** - Administrative operations

**Tasks**:
1. **Lookup Endpoint Implementation**
   ```go
   func (s *OverlayServer) handleLookup(c *fiber.Ctx) error {
       var question storage.EventQuestion
       if err := c.BodyParser(&question); err != nil {
           return c.Status(400).JSON(ErrorResponse{Message: "Invalid lookup query"})
       }
       
       results, err := s.Engine.Storage.(storage.EventDataStorage).LookupOutpoints(c.Context(), &question, true)
       if err != nil {
           return c.Status(500).JSON(ErrorResponse{Message: err.Error()})
       }
       
       return c.JSON(results)
   }
   ```

2. **Documentation Endpoints**
   - Implement service discovery using engine's topic managers
   - Return service documentation and capabilities
   - Support dynamic service registration

3. **ARC Integration** 
   - Process merkle proofs from ARC webhooks
   - Update transaction confirmation status
   - Integrate with blockchain tracking

### Phase 4: Queue Processing and Real-time Features (Priority 2)
**Timeline**: 2-3 days
**Goal**: Add asynchronous processing and real-time capabilities

**Benefits**:
- Scalable transaction processing via queues
- Real-time event broadcasting to clients  
- High-throughput capability for production workloads

**Tasks**:
1. **Queue Processor Integration**
   - Import and configure Redis queue processor
   - Implement TransactionProcessor interface
   - Add background processing for submitted transactions

2. **Event Subscriptions**
   - Add WebSocket support for real-time events
   - Integrate with Redis publisher for live updates
   - Support topic-based subscriptions

3. **Background Services**
   - Transaction processing workers
   - Event cleanup and archival
   - Health monitoring and metrics

### Phase 5: Web UI and Documentation (Priority 3)
**Timeline**: 2-3 days  
**Goal**: Replace JSON responses with HTML interface

**Benefits**:
- User-friendly web interface matching overlay-express
- Interactive API documentation
- Service status monitoring

**Tasks**:
1. **HTML Interface Generation**
   - Port makeUserInterface() logic from overlay-express
   - Dynamic HTML generation based on configured services
   - Interactive documentation with live examples

2. **Service Status Dashboard**
   - Real-time service health monitoring
   - Storage backend status and metrics
   - Queue processing statistics

## Technical Migration Details

### Dependency Management

**New Dependencies**:
```go
// Add to go.mod
require (
    github.com/b-open-io/overlay v0.1.0
    github.com/redis/go-redis/v9 v9.11.0  // Already present
    // MongoDB and SQLite already supported
)
```

### Configuration Enhancements

**Environment Variables**:
- `EVENT_STORAGE`: Connection string for event data storage (auto-detects type)
- `BEEF_STORAGE`: Connection string for BEEF storage (defaults to ./beef_storage)
- `BEEF_CACHE_TTL`: TTL for Redis BEEF caching
- `REDIS_PUBLISHER_URL`: Redis URL for event publishing

**Connection String Examples**:
```bash
# Redis (with caching)
EVENT_STORAGE=redis://localhost:6379
BEEF_STORAGE=redis://localhost:6379

# MongoDB  
EVENT_STORAGE=mongodb://localhost:27017/overlay
BEEF_STORAGE=mongodb://localhost:27017/beef

# SQLite (default)
EVENT_STORAGE=./overlay.db
BEEF_STORAGE=./beef.db

# Filesystem BEEF storage
BEEF_STORAGE=./beef_storage/
```

### Data Migration Strategy

**Migration Approach**:
1. **Parallel Operation**: Run both old and new storage during transition
2. **Data Validation**: Compare results between implementations
3. **Gradual Rollout**: Enable new storage for specific topics first
4. **Rollback Capability**: Quick fallback to existing implementation

**Migration Tools**:
```go
// Migration utility
func MigrateToOverlayStorage(oldStorage *SQLStorage, newStorage storage.EventDataStorage) error {
    // Copy existing data to new storage
    // Validate data integrity
    // Update configuration
    return nil
}
```

## Testing Strategy

### Integration Testing
- **Multi-Backend Testing**: Test all storage combinations (Redis, MongoDB, SQLite)
- **Performance Testing**: Compare throughput with current implementation
- **Compatibility Testing**: Ensure API compatibility with overlay-express
- **Data Migration Testing**: Validate data integrity during transitions

### Test Scenarios
1. **Storage Backend Switching**: Test seamless backend changes
2. **High-Volume Submissions**: Test queue processing under load
3. **Real-time Events**: Test pub/sub functionality
4. **Failure Recovery**: Test resilience to storage failures

## Risk Assessment & Mitigation

### High Risk
**Risk**: Data loss during migration  
**Mitigation**: Comprehensive backup strategy, parallel operation, rollback procedures

**Risk**: Performance degradation  
**Mitigation**: Performance benchmarking, gradual rollout, monitoring

### Medium Risk  
**Risk**: Compatibility issues with overlay-services  
**Mitigation**: Version pinning, comprehensive integration testing

**Risk**: Configuration complexity  
**Mitigation**: Auto-detection, sensible defaults, documentation

### Low Risk
**Risk**: Learning curve for new components  
**Mitigation**: Code documentation, examples, gradual adoption

## Success Metrics

### Performance Targets
- **Throughput**: Handle 1000+ transactions/second
- **Latency**: < 100ms for lookups, < 500ms for submissions
- **Storage**: Support for Redis caching with < 10ms access times

### Functional Targets  
- **API Compatibility**: 100% compatibility with overlay-express endpoints
- **Multi-Backend**: All storage types (Redis, MongoDB, SQLite) functional
- **Real-time**: Event publishing with < 1s latency

### Operational Targets
- **Reliability**: 99.9% uptime with graceful failure handling
- **Monitoring**: Comprehensive health checks and metrics
- **Documentation**: Complete API documentation and usage examples

## Implementation Timeline

### Week 1: Storage Foundation
- **Days 1-2**: Phase 1 - Storage layer replacement
- **Days 3-4**: Phase 2 - BEEF and publishing integration  
- **Day 5**: Testing and validation

### Week 2: Feature Implementation
- **Days 1-3**: Phase 3 - Advanced endpoints implementation
- **Days 4-5**: Phase 4 - Queue processing and real-time features

### Week 3: Polish and Production
- **Days 1-2**: Phase 5 - Web UI implementation
- **Days 3-4**: Performance optimization and testing
- **Day 5**: Documentation and deployment preparation

## Next Immediate Actions

### Day 1 - Storage Integration Start
1. **Import overlay library**
   ```bash
   cd /Users/jason/src/bsv/go-overlay-fiber
   go get github.com/b-open-io/overlay@latest
   ```

2. **Create overlay storage adapter**
   ```go
   // pkg/server/overlay_storage.go
   type OverlayStorageAdapter struct {
       eventStorage storage.EventDataStorage
   }
   ```

3. **Update ConfigureEngine method**
   - Import overlay factories
   - Create BEEF storage and publisher
   - Wire components together

### Day 2 - Testing and Validation  
1. **Create storage backend tests**
2. **Test Redis, MongoDB, SQLite configurations**
3. **Validate existing functionality works**

This integration plan provides a systematic approach to incorporating the powerful overlay library components while maintaining stability and backward compatibility. The phased approach allows for iterative testing and validation at each step, ensuring a smooth transition to production-ready infrastructure.