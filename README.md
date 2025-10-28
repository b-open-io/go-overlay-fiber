# go-overlay-fiber

An opinionated but configurable [Overlay Services](https://github.com/bsv-blockchain/go-overlay-services) deployment system using the Fiber web framework for Go. This library provides HTTP server infrastructure and database integration for deploying Overlay service nodes on the BSV blockchain network, offering a production-ready alternative to the TypeScript [`overlay-express`](https://github.com/bsv-blockchain/overlay-express) library.

## Features

- **Overlay Services Integration**: Deploys the [go-overlay-services](https://github.com/bsv-blockchain/go-overlay-services) engine with opinionated defaults while maintaining full configurability for advanced use cases
- **HTTP Server**: Fiber-based web server with REST API endpoints for transaction submission (`/submit`) and UTXO lookups (`/lookup`)
- **Database Connectivity**: Out-of-the-box support for SQL databases (SQLite, MySQL, PostgreSQL) and MongoDB with easy configuration
- **Configurable Web UI**: Dynamic interface for viewing topic managers, lookup services, and node status
- **Discovery Protocol Support**: Built-in SHIP, SLAP, and GASP synchronization protocols for overlay network participation
- **Real-time Communications**: WebSocket support for live event subscriptions and topic-based message broadcasting
- **Queue-Based Processing**: Background transaction processing with health monitoring and automatic retry logic
- **Admin Interface**: Web UI and API for managing sync operations, GASP nodes, and service monitoring
- **Fluent Configuration**: Builder pattern API for easy server setup with sensible defaults

## Quick Start

### Installation

```bash
go get github.com/bsv-blockchain/go-overlay-fiber
```

### Basic Example

```go
package main

import (
    "log/slog"
    "os"
    "github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
)

func main() {
    // Create a new overlay server
    overlayServer := server.NewOverlayServer(
        "testnode",                        // Node name
        os.Getenv("SERVER_PRIVATE_KEY"),   // Node identity key
        os.Getenv("HOSTING_URL"),          // Public HTTPS URL
    )

    slog.SetLogLoggerLevel(slog.LevelInfo)

    // Configure server
    overlayServer.ConfigurePort(8080)
    overlayServer.ConfigureDatabase("sqlite3", "./data.db")

    // Optional: Connect to MongoDB for SHIP/SLAP services
    if mongoURL := os.Getenv("MONGO_URL"); mongoURL != "" {
        overlayServer.ConfigureMongoDB(mongoURL)
    }

    // Add your topic managers and lookup services here
    // overlayServer.ConfigureTopicManager(...)
    // overlayServer.ConfigureLookupServiceWithMongo(...)

    // Enable GASP synchronization
    overlayServer.ConfigureGASPSync(true)

    // Configure engine with auto-configuration
    overlayServer.ConfigureEngine(true)

    // Start the server
    if err := overlayServer.Start(); err != nil {
        slog.Error("Failed to start server", "error", err)
        os.Exit(1)
    }
}
```

### Running Examples

#### Basic Example (SQLite)
```bash
cd examples/basic
SERVER_PRIVATE_KEY=dummy HOSTING_URL=http://localhost:8080 go run main.go
```

#### Local Example (Docker with MySQL)
```bash
cd examples/local
docker compose up
```

## API Endpoints

- `GET /` - Web interface and service information
- `GET /health` - Health status check
- `POST /submit` - Submit BEEF transactions with topics
- `POST /lookup` - Query outpoints by topic and events
- `GET /ws` - WebSocket connection for real-time updates
- `GET /listTopicManagers` - List available topic managers
- `GET /listLookupServiceProviders` - List available lookup services
- `POST /admin/requestSyncResponse` - Request sync from GASP node
- `POST /admin/evictGASPNode` - Remove a GASP node
- `POST /admin/requestForeignGASPNode` - Request sync from foreign node

## Related Repositories

### Go Implementations

| Repository | Purpose |
|------------|---------|
| [go-overlay-fiber](https://github.com/bsv-blockchain/go-overlay-fiber) | Web server framework for Overlay nodes (this repo) |
| [go-sdk](https://github.com/bsv-blockchain/go-sdk) | Core BSV SDK for transaction handling and blockchain operations |
| [go-overlay-services](https://github.com/bsv-blockchain/go-overlay-services) | Overlay engine and service implementations |
| [go-overlay-discovery-services](https://github.com/bsv-blockchain/go-overlay-discovery-services) | SHIP/SLAP discovery protocol implementations |
| [go-wallet-toolbox](https://github.com/bsv-blockchain/go-wallet-toolbox) | Wallet utilities and tools |
| [go-bsv-middleware](https://github.com/bsv-blockchain/go-bsv-middleware) | Middleware components for BSV applications |
| [overlay](https://github.com/b-open-io/overlay) | Core overlay protocol definitions and types |

### TypeScript Equivalents

| Go Repository | TypeScript Equivalent | Purpose |
|---------------|----------------------|---------|
| go-overlay-fiber | [overlay-express](https://github.com/bsv-blockchain/overlay-express) | Express.js-based overlay server |
| go-sdk | [ts-sdk](https://github.com/bsv-blockchain/ts-sdk) | TypeScript BSV SDK |
| go-overlay-services | [overlay-services](https://github.com/bsv-blockchain/overlay-services) | Overlay service implementations |
| go-overlay-discovery-services | [overlay-discovery-services](https://github.com/bsv-blockchain/overlay-discovery-services) | Discovery services |
| - | [overlay-express-examples](https://github.com/bsv-blockchain/overlay-express-examples) | Example implementations |

## BRC Standards

This implementation follows several Bitcoin Request for Comment (BRC) standards related to overlay networks:

### Core Overlay Standards

- **[BRC-22: Overlay Network Data Synchronization](https://github.com/bitcoin-sv/BRCs/blob/master/brc-22/0022.md)** - Establishes protocols for synchronizing data across overlay networks, enabling distributed systems to maintain consistency when sharing transaction and state information.

- **[BRC-24: Overlay Network Lookup Services](https://github.com/bitcoin-sv/BRCs/blob/master/brc-24/0024.md)** - Provides methods for discovering and locating resources within overlay networks, enabling efficient service and data discovery across distributed architectures.

- **[BRC-64: Overlay Network Transaction History Tracking](https://github.com/bitcoin-sv/BRCs/blob/master/brc-64/0064.md)** - Addresses persistent record-keeping across overlay infrastructure, enabling participants to maintain and verify transaction lineage within distributed networks.

### Discovery and Synchronization

- **[BRC-23: Confederacy Host Interconnect Protocol (CHIP)](https://github.com/bitcoin-sv/BRCs/blob/master/brc-23/0023.md)** - Defines communication mechanisms between hosts within confederacy structures for reliable peer connections.

- **[BRC-25: Confederacy Lookup Availability Protocol (CLAP)](https://github.com/bitcoin-sv/BRCs/blob/master/brc-25/0025.md)** - Establishes availability checking mechanisms to verify service accessibility and responsiveness.

- **[BRC-88: Overlay Services Synchronization Architecture](https://github.com/bitcoin-sv/BRCs/blob/master/brc-88/0088.md)** - Defines comprehensive synchronization frameworks for overlay service coordination and reliable state management.

- **[BRC-101: Diverse Facilitators and URL Protocols for SHIP and SLAP Overlay Advertisements](https://github.com/bitcoin-sv/BRCs/blob/master/brc-101/0101.md)** - Expands advertisement capabilities for overlay discovery protocols using multiple transport mechanisms.

### Naming and Privacy

- **[BRC-81: Private Overlays with P2PKH Transactions](https://github.com/bitcoin-sv/BRCs/blob/master/brc-81/0081.md)** - Extends overlay functionality to private implementations using standard payment script types while maintaining privacy.

- **[BRC-87: Standardized Naming Conventions for BRC-22 Topic Managers and BRC-24 Lookup Services](https://github.com/bitcoin-sv/BRCs/blob/master/brc-87/0087.md)** - Establishes consistent naming schemes for overlay components to improve interoperability across implementations.

## Architecture

### Project Structure

```
go-overlay-fiber/
├── pkg/server/
│   ├── server.go              # Main server implementation
│   ├── websocket.go           # WebSocket management
│   ├── queue_processor.go     # Background processing
│   ├── overlay_storage.go     # Storage integration
│   ├── interfaces.go          # Type definitions
│   ├── templates.go           # Template management
│   └── templates/             # HTML UI templates
├── examples/
│   ├── basic/                 # Basic SQLite example
│   └── local/                 # Docker-based example
└── .claude/
    ├── scripts/               # Test and utility scripts
    └── reports/               # Analysis reports
```

### Key Components

- **Server**: Fiber-based HTTP server with routing and middleware
- **Queue Processor**: Background service for asynchronous transaction processing
- **Overlay Storage**: Integration layer for topic managers and lookup services
- **WebSocket Manager**: Real-time connection and broadcast management
- **Template Engine**: Dynamic HTML rendering for web interface

## Configuration

### Server Configuration Methods

```go
// Basic setup
server.NewOverlayServer(name, privateKey, hostingURL)

// Configure port
server.ConfigurePort(8080)

// Database configuration
server.ConfigureDatabase("sqlite3", "./data.db")
server.ConfigureDatabase("mysql", "user:pass@tcp(host:3306)/dbname")
server.ConfigureDatabase("postgres", "postgresql://user:pass@host:5432/dbname")

// MongoDB for SHIP/SLAP services
server.ConfigureMongoDB("mongodb://localhost:27017")

// Enable GASP synchronization
server.ConfigureGASPSync(true)

// Add topic managers and lookup services
server.ConfigureTopicManager(manager)
server.ConfigureLookupService(service)
server.ConfigureLookupServiceWithMongo(service)

// Custom SHIP/SLAP trackers
server.ConfigureSHIPTrackers(trackers)
server.ConfigureSLAPTrackers(trackers)

// Admin authentication
server.ConfigureAdminAuthentication(token)

// Engine configuration
server.ConfigureEngine(autoConfig)
```

### Environment Variables

- `SERVER_PRIVATE_KEY` - Private key for node identity (required)
- `HOSTING_URL` - Public URL where the server is accessible (required)
- `MONGO_URL` - MongoDB connection string (optional, for SHIP/SLAP services)
- `SERVER_PORT` - Server port (optional, default: 8080 in examples)

## Contributing

This project aims to maintain parity with the TypeScript `overlay-express` implementation while following Go best practices. Contributions should:

- Follow existing file structure and naming conventions
- Use existing code from related Go repositories when possible
- Maintain compatibility with corresponding TypeScript implementations
- Include tests and documentation

## License

See LICENSE file for details.

## Resources

- [BRC Standards Repository](https://github.com/bitcoin-sv/BRCs)
- [BSV Blockchain Documentation](https://docs.bsvblockchain.org/)
- [Overlay Network Documentation](https://projectbabbage.com/docs/)