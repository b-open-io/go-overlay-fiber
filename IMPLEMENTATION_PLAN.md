# Go Overlay Fiber Implementation Plan

## Overview
This document outlines the implementation plan for recreating the `overlay-express` project in Go using the Fiber web framework. The goal is to create a focused, single-file implementation that mirrors the core functionality of `OverlayExpress.ts`.

## Analysis of Source Material

### Key Components from OverlayExpress.ts
1. **Main Class Structure**: OverlayExpress class with configuration methods
2. **Database Support**: Both SQL (Knex) and MongoDB configurations
3. **Core Engine**: BSV Overlay Engine with topic managers and lookup services
4. **HTTP Endpoints**: Express.js routes for overlay operations
5. **Admin Routes**: Protected administrative endpoints
6. **Discovery Services**: SHIP and SLAP protocol support

### Primary Dependencies
- Express.js → Go Fiber (HTTP framework)
- @bsv/overlay → go-overlay-discovery-services (overlay functionality)
- Knex → database/sql + driver (SQL operations)
- MongoDB driver → go.mongodb.org/mongo-driver (MongoDB operations)

## Implementation Structure

### Project Layout
```
go-overlay-fiber/
├── main.go                 # Single main implementation file
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
└── IMPLEMENTATION_PLAN.md  # This file
```

### Core Components

#### 1. OverlayServer Struct
```go
type OverlayServer struct {
    // Core properties
    Name               string
    PrivateKey         string
    AdvertisableFQDN   string
    Port               int
    Network            string // "main" or "test"
    
    // Configuration
    Logger             *log.Logger
    AdminToken         string
    VerboseLogging     bool
    EnableGASPSync     bool
    
    // Database connections
    DB                 *sql.DB
    MongoDB            *mongo.Database
    
    // Services
    TopicManagers      map[string]interface{}
    LookupServices     map[string]types.LookupService
    
    // Fiber app
    App                *fiber.App
}
```

#### 2. Configuration Methods
Mirror the TypeScript configuration pattern:
- `ConfigurePort(port int)`
- `ConfigureLogger(logger *log.Logger)`
- `ConfigureNetwork(network string)`
- `ConfigureDatabase(connectionString string)`
- `ConfigureMongoDB(connectionString string)`
- `ConfigureVerboseLogging(enable bool)`
- `ConfigureGASPSync(enable bool)`

#### 3. HTTP Endpoints
Recreate all essential routes from OverlayExpress.ts:

**Public Routes:**
- `GET /` - Web UI/documentation
- `GET /listTopicManagers` - List available topic managers
- `GET /listLookupServiceProviders` - List lookup services
- `GET /getDocumentationForTopicManager` - Get topic manager docs
- `GET /getDocumentationForLookupServiceProvider` - Get service docs
- `POST /submit` - Submit transactions (TaggedBEEF)
- `POST /lookup` - Perform lookup queries
- `POST /arc-ingest` - ARC webhook endpoint (if API key configured)

**GASP Sync Routes (if enabled):**
- `POST /requestSyncResponse` - Handle sync requests
- `POST /requestForeignGASPNode` - Handle foreign GASP node requests

**Admin Routes (Bearer token protected):**
- `POST /admin/syncAdvertisements` - Manually sync advertisements
- `POST /admin/startGASPSync` - Manually start GASP sync
- `POST /admin/evictOutpoint` - Evict specific outpoints

#### 4. Service Integration
Integrate with go-overlay-discovery-services:
- Import SHIP and SLAP services from local package
- Auto-configure topic managers and lookup services
- Handle advertisement creation and syncing
- Support both SQL and MongoDB storage backends

## Implementation Steps

### Phase 1: Basic Structure
1. Initialize Go module with Fiber dependency
2. Create basic OverlayServer struct
3. Implement configuration methods
4. Set up basic Fiber app with CORS and middleware

### Phase 2: Database Integration
1. Add SQL database support with migrations
2. Add MongoDB connection handling
3. Implement storage abstraction layer

### Phase 3: Core Endpoints
1. Implement public API endpoints
2. Add request/response logging middleware
3. Handle TaggedBEEF processing for /submit
4. Implement lookup query handling

### Phase 4: Discovery Services
1. Integrate go-overlay-discovery-services
2. Auto-configure SHIP and SLAP services
3. Implement advertisement syncing
4. Add GASP sync functionality

### Phase 5: Admin Features
1. Implement Bearer token authentication middleware
2. Add admin endpoints
3. Implement manual sync operations
4. Add outpoint eviction

### Phase 6: Production Features
1. Add proper error handling and logging
2. Implement graceful shutdown
3. Add health check endpoint
4. Optimize performance and memory usage

## Key Dependencies

### Required Go Packages
```go
// Web framework
"github.com/gofiber/fiber/v2"
"github.com/gofiber/fiber/v2/middleware/cors"
"github.com/gofiber/fiber/v2/middleware/logger"

// Database
"database/sql"
"go.mongodb.org/mongo-driver/mongo"
"go.mongodb.org/mongo-driver/mongo/options"
"github.com/go-sql-driver/mysql" // MySQL driver

// Local packages
"../go-overlay-discovery-services/pkg/types"
"../go-overlay-discovery-services/pkg/ship"
"../go-overlay-discovery-services/pkg/slap"
"../go-overlay-discovery-services/pkg/advertiser"

// Utilities
"github.com/google/uuid"
"encoding/json"
"encoding/hex"
```

## Configuration Patterns

### Environment-Based Configuration
```go
type Config struct {
    Name             string `env:"SERVER_NAME" envDefault:"go-overlay-fiber"`
    Port             int    `env:"PORT" envDefault:"3000"`
    Network          string `env:"NETWORK" envDefault:"main"`
    PrivateKey       string `env:"PRIVATE_KEY,required"`
    FQDN             string `env:"FQDN,required"`
    DatabaseURL      string `env:"DATABASE_URL"`
    MongoURL         string `env:"MONGO_URL"`
    AdminToken       string `env:"ADMIN_TOKEN"`
    ARCAPIKey        string `env:"ARC_API_KEY"`
    VerboseLogging   bool   `env:"VERBOSE_LOGGING" envDefault:"false"`
    EnableGASPSync   bool   `env:"ENABLE_GASP_SYNC" envDefault:"true"`
}
```

### Fluent Configuration API
```go
server := NewOverlayServer("MyOverlayService", privateKey, fqdn).
    ConfigurePort(3000).
    ConfigureNetwork("test").
    ConfigureDatabase("mysql://user:pass@localhost/overlay").
    ConfigureMongoDB("mongodb://localhost:27017").
    ConfigureVerboseLogging(true).
    ConfigureGASPSync(true)

if err := server.Start(); err != nil {
    log.Fatal(err)
}
```

## Error Handling Strategy

### Consistent Error Response Format
```go
type ErrorResponse struct {
    Status  string `json:"status"`
    Code    string `json:"code,omitempty"`
    Message string `json:"message"`
}
```

### Middleware for Error Handling
- Panic recovery middleware
- Structured error logging
- Consistent JSON error responses
- Request tracing for debugging

## Testing Strategy

### Unit Tests
- Configuration method testing
- Endpoint handler testing
- Database integration testing
- Mock external service dependencies

### Integration Tests
- Full server startup/shutdown testing
- Database migration testing
- Cross-service communication testing

## Performance Considerations

### Optimizations
- Connection pooling for databases
- Efficient JSON marshaling/unmarshaling  
- Request response caching where appropriate
- Concurrent request handling with Fiber's built-in support

### Monitoring
- Request duration metrics
- Database connection health
- Memory usage tracking
- Error rate monitoring

## Migration Notes

### Key Differences from TypeScript
1. **Explicit Error Handling**: Go's explicit error handling vs TypeScript's try/catch
2. **Type Safety**: Go's compile-time type checking vs TypeScript's runtime
3. **Concurrency**: Go's goroutines vs Node.js event loop
4. **Memory Management**: Go's garbage collector vs Node.js V8

### Compatibility Considerations
- Maintain API compatibility with existing clients
- Preserve request/response formats
- Support same configuration options
- Match behavior of admin endpoints

## Success Criteria

1. **Functional Parity**: All essential endpoints from OverlayExpress.ts working
2. **Performance**: Better performance than Node.js equivalent
3. **Maintainability**: Clean, readable Go code following best practices
4. **Integration**: Seamless integration with go-overlay-discovery-services
5. **Production Ready**: Proper logging, error handling, and configuration

## Future Enhancements

### Post-MVP Features
- Metrics and monitoring endpoints
- Configuration hot-reloading
- Advanced caching strategies
- Horizontal scaling support
- Docker containerization
- Kubernetes deployment manifests

This implementation plan focuses on creating a minimal but complete recreation of the overlay-express functionality while leveraging Go's strengths and the existing go-overlay-discovery-services package.