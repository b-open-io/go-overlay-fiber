package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	// SQL drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// OverlayServer represents the main server structure
type OverlayServer struct {
	// Core properties
	Name             string
	PrivateKey       string
	AdvertisableFQDN string
	Port             int
	Network          string // "main" or "test"

	// Configuration
	Logger         *log.Logger
	AdminToken     string
	VerboseLogging bool
	EnableGASPSync bool
	ARCAPIKey      string

	// Database connections
	DB      *sql.DB
	MongoDB *mongo.Database

	// Engine instance
	Engine *engine.Engine

	// Services
	Managers        map[string]engine.TopicManager
	Services        map[string]engine.LookupService
	ChainTracker    chaintracker.ChainTracker
	WebUIConfig     UIConfig
	EngineConfig    EngineConfig
	MigrationsToRun []Migration

	// Fiber app
	App *fiber.App
}

// NewOverlayServer creates a new OverlayServer instance
func NewOverlayServer(name, privateKey, fqdn string) *OverlayServer {
	return &OverlayServer{
		Name:             name,
		PrivateKey:       privateKey,
		AdvertisableFQDN: fqdn,
		Port:             3000,
		Network:          "main",
		Logger:           log.Default(),
		VerboseLogging:   false,
		EnableGASPSync:   true,
		Managers:         make(map[string]engine.TopicManager),
		Services:         make(map[string]engine.LookupService),
		MigrationsToRun:  make([]Migration, 0),
	}
}

// ConfigurePort sets the server port
func (s *OverlayServer) ConfigurePort(port int) *OverlayServer {
	s.Port = port
	return s
}

// ConfigureLogger sets the server logger
func (s *OverlayServer) ConfigureLogger(logger *log.Logger) *OverlayServer {
	s.Logger = logger
	return s
}

// ConfigureNetwork sets the network (main or test)
func (s *OverlayServer) ConfigureNetwork(network string) *OverlayServer {
	s.Network = network
	return s
}

// ConfigureDatabase establishes SQL database connection
func (s *OverlayServer) ConfigureDatabase(driverName, connectionString string) *OverlayServer {
	if connectionString == "" {
		s.Logger.Printf("No SQL database connection string provided")
		return s
	}

	// Open database connection
	db, err := sql.Open(driverName, connectionString)
	if err != nil {
		s.Logger.Printf("Failed to open SQL database connection: %v", err)
		return s
	}

	// Configure connection pool for optimal performance
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	s.DB = db
	s.Logger.Printf("SQL database configured: %s (driver: %s)", connectionString, driverName)
	return s
}

// ConfigureMongoDB establishes MongoDB connection (equivalent to overlay-express configureMongo)
func (s *OverlayServer) ConfigureMongoDB(connectionString string) *OverlayServer {
	return s.ConfigureMongoDBWithDatabase(connectionString, "overlay")
}

// ConfigureMongoDBWithDatabase establishes MongoDB connection with custom database name
func (s *OverlayServer) ConfigureMongoDBWithDatabase(connectionString, database string) *OverlayServer {
	if connectionString == "" {
		s.Logger.Printf("No MongoDB connection string provided")
		return s
	}

	// Create MongoDB client options - direct usage like overlay-express uses mongodb directly
	clientOptions := options.Client().ApplyURI(connectionString)
	clientOptions.SetMaxPoolSize(100)
	clientOptions.SetMaxConnIdleTime(5 * time.Minute)
	clientOptions.SetConnectTimeout(30 * time.Second)
	clientOptions.SetServerSelectionTimeout(30 * time.Second)

	// Create MongoDB client connection
	ctx := context.Background()
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		s.Logger.Printf("Failed to create MongoDB client: %v", err)
		return s
	}

	// Get database reference - ready for use by engine/services
	s.MongoDB = client.Database(database)
	s.Logger.Printf("MongoDB configured: %s (database: %s)", connectionString, database)
	return s
}

// ConfigureVerboseLogging enables/disables verbose logging
func (s *OverlayServer) ConfigureVerboseLogging(enable bool) *OverlayServer {
	s.VerboseLogging = enable
	return s
}

// ConfigureGASPSync enables/disables GASP sync
func (s *OverlayServer) ConfigureGASPSync(enable bool) *OverlayServer {
	s.EnableGASPSync = enable
	return s
}

// ConfigureAdminToken sets the admin authentication token
func (s *OverlayServer) ConfigureAdminToken(token string) *OverlayServer {
	s.AdminToken = token
	return s
}

// ConfigureARCAPIKey sets the ARC API key
func (s *OverlayServer) ConfigureARCAPIKey(key string) *OverlayServer {
	s.ARCAPIKey = key
	return s
}

// ConfigureTopicManager stores a topic manager with the given name
func (s *OverlayServer) ConfigureTopicManager(name string, manager engine.TopicManager) *OverlayServer {
	s.Managers[name] = manager
	s.Logger.Printf("Topic manager '%s' configured", name)
	return s
}

// ConfigureLookupService stores a lookup service with the given name
func (s *OverlayServer) ConfigureLookupService(name string, service engine.LookupService) *OverlayServer {
	s.Services[name] = service
	s.Logger.Printf("Lookup service '%s' configured", name)
	return s
}

// ConfigureLookupServiceWithDB creates a lookup service with SQL database and migrations
func (s *OverlayServer) ConfigureLookupServiceWithDB(name string, factory LookupServiceFactory) *OverlayServer {
	if s.DB == nil {
		s.Logger.Printf("Warning: ConfigureLookupServiceWithDB called but no SQL database configured")
		return s
	}

	service, migrations, err := factory(s.DB)
	if err != nil {
		s.Logger.Printf("Error creating lookup service '%s': %v", name, err)
		return s
	}

	s.Services[name] = service

	// Add migrations to the list to be run
	s.MigrationsToRun = append(s.MigrationsToRun, migrations...)

	s.Logger.Printf("Lookup service '%s' configured with SQL database and %d migrations", name, len(migrations))
	return s
}

// ConfigureLookupServiceWithMongo creates a lookup service with MongoDB
func (s *OverlayServer) ConfigureLookupServiceWithMongo(name string, factory MongoLookupServiceFactory) *OverlayServer {
	if s.MongoDB == nil {
		s.Logger.Printf("Warning: ConfigureLookupServiceWithMongo called but no MongoDB configured")
		return s
	}

	service, err := factory(s.MongoDB)
	if err != nil {
		s.Logger.Printf("Error creating lookup service '%s': %v", name, err)
		return s
	}

	s.Services[name] = service

	s.Logger.Printf("Lookup service '%s' configured with MongoDB", name)
	return s
}

// ConfigureChainTracker sets the chain tracker
func (s *OverlayServer) ConfigureChainTracker(chainTracker chaintracker.ChainTracker) *OverlayServer {
	s.ChainTracker = chainTracker
	s.Logger.Printf("Chain tracker configured")
	return s
}

// ConfigureEngineParams stores advanced engine configuration
func (s *OverlayServer) ConfigureEngineParams(params EngineConfig) *OverlayServer {
	s.EngineConfig = params
	return s
}

// ConfigureWebUI stores web UI configuration
func (s *OverlayServer) ConfigureWebUI(config UIConfig) *OverlayServer {
	s.WebUIConfig = config
	return s
}

// GetAdminToken returns the admin token
func (s *OverlayServer) GetAdminToken() string {
	return s.AdminToken
}

// ConfigureEngine creates and configures the Engine instance using database connections
// This matches overlay-express's pattern of creating Engine from @bsv/overlay with database
func (s *OverlayServer) ConfigureEngine(hostingURL string) *OverlayServer {
	// Create Engine configuration matching overlay-express pattern
	engineConfig := engine.Engine{
		HostingURL:     hostingURL,
		Managers:       make(map[string]engine.TopicManager),
		LookupServices: make(map[string]engine.LookupService),
		// Storage will be configured when database connections are available
		// This matches overlay-express's approach of passing database to Engine
		LogPrefix: s.Name,
		LogTime:   s.VerboseLogging,
	}

	// Initialize the Engine
	s.Engine = engine.NewEngine(engineConfig)

	s.Logger.Printf("Engine configured with hosting URL: %s", hostingURL)
	return s
}

// ConfigureEngineStorage configures the Engine's storage with available database connections
func (s *OverlayServer) ConfigureEngineStorage() error {
	if s.Engine == nil {
		return fmt.Errorf("engine not configured - call ConfigureEngine first")
	}

	if s.DB != nil || s.MongoDB != nil {
		s.Logger.Printf("Database connections available for Engine storage")
		return nil
	}

	return fmt.Errorf("no database connections available for engine storage")
}

// Setup initializes the Fiber app and middleware
func (s *OverlayServer) Setup() error {
	s.App = fiber.New(fiber.Config{
		ErrorHandler: s.errorHandler,
	})

	// Add middleware
	s.App.Use(recover.New())
	s.App.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	if s.VerboseLogging {
		s.App.Use(logger.New())
	}

	// Setup routes
	s.setupRoutes()

	return nil
}

// setupRoutes configures all the HTTP routes
func (s *OverlayServer) setupRoutes() {
	// Public routes
	s.App.Get("/", s.handleWebUI)
	s.App.Get("/health", s.handleHealthCheck)
	s.App.Get("/listTopicManagers", s.handleListTopicManagers)
	s.App.Get("/listLookupServiceProviders", s.handleListLookupServiceProviders)
	s.App.Get("/getDocumentationForTopicManager", s.handleGetTopicManagerDocs)
	s.App.Get("/getDocumentationForLookupServiceProvider", s.handleGetLookupServiceDocs)
	s.App.Post("/submit", s.handleSubmit)
	s.App.Post("/lookup", s.handleLookup)

	// ARC webhook endpoint (if API key configured)
	if s.ARCAPIKey != "" {
		s.App.Post("/arc-ingest", s.handleARCIngest)
	}

	// GASP sync routes (if enabled)
	if s.EnableGASPSync {
		s.App.Post("/requestSyncResponse", s.handleRequestSyncResponse)
		s.App.Post("/requestForeignGASPNode", s.handleRequestForeignGASPNode)
	}

	// Admin routes (Bearer token protected)
	if s.AdminToken != "" {
		admin := s.App.Group("/admin", s.requireAdminAuth)
		admin.Post("/syncAdvertisements", s.handleSyncAdvertisements)
		admin.Post("/startGASPSync", s.handleStartGASPSync)
		admin.Post("/evictOutpoint", s.handleEvictOutpoint)
	}
}

// Middleware for admin authentication
func (s *OverlayServer) requireAdminAuth(c *fiber.Ctx) error {
	if s.AdminToken == "" {
		return c.Status(401).JSON(ErrorResponse{
			Status:  "error",
			Code:    "ADMIN_DISABLED",
			Message: "Admin functionality is disabled",
		})
	}

	auth := c.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		return c.Status(401).JSON(ErrorResponse{
			Status:  "error",
			Code:    "UNAUTHORIZED",
			Message: "Bearer token required",
		})
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token != s.AdminToken {
		return c.Status(401).JSON(ErrorResponse{
			Status:  "error",
			Code:    "INVALID_TOKEN",
			Message: "Invalid admin token",
		})
	}

	return c.Next()
}

// Error handler
func (s *OverlayServer) errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(ErrorResponse{
		Status:  "error",
		Message: err.Error(),
	})
}

// Route handlers (placeholders for Phase 1)

func (s *OverlayServer) handleWebUI(c *fiber.Ctx) error {
	// Check database status
	databaseStatus := "none"
	if s.DB != nil {
		databaseStatus = "sql"
	}
	if s.MongoDB != nil {
		if databaseStatus == "sql" {
			databaseStatus = "sql+mongodb"
		} else {
			databaseStatus = "mongodb"
		}
	}

	// Check engine status
	engineStatus := "not_configured"
	if s.Engine != nil {
		engineStatus = "configured"
	}

	return c.JSON(fiber.Map{
		"name":                     s.Name,
		"network":                  s.Network,
		"fqdn":                     s.AdvertisableFQDN,
		"status":                   "running",
		"database_status":          databaseStatus,
		"engine_status":            engineStatus,
		"message":                  "Go Overlay Fiber Server - Phase 1 Complete: Missing Configuration Methods Added",
		"phase":                    "1",
		"topic_managers":           len(s.Managers),
		"lookup_services":          len(s.Services),
		"migrations_count":         len(s.MigrationsToRun),
		"webui_configured":         s.WebUIConfig.Host != "",
		"chain_tracker_configured": s.ChainTracker != nil,
	})
}

func (s *OverlayServer) handleHealthCheck(c *fiber.Ctx) error {
	ctx := context.Background()

	healthStatus := fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"server":    s.Name,
		"network":   s.Network,
	}

	// Check database health
	databaseHealth := fiber.Map{}

	if s.DB != nil {
		if err := s.DB.PingContext(ctx); err != nil {
			databaseHealth["sql"] = fiber.Map{"status": "unhealthy", "error": err.Error()}
			healthStatus["status"] = "degraded"
		} else {
			databaseHealth["sql"] = fiber.Map{"status": "healthy"}
		}
	}

	if s.MongoDB != nil {
		if err := s.MongoDB.Client().Ping(ctx, nil); err != nil {
			databaseHealth["mongodb"] = fiber.Map{"status": "unhealthy", "error": err.Error()}
			healthStatus["status"] = "degraded"
		} else {
			databaseHealth["mongodb"] = fiber.Map{"status": "healthy"}
		}
	}

	healthStatus["databases"] = databaseHealth

	// Check engine status
	if s.Engine != nil {
		healthStatus["engine"] = fiber.Map{"status": "configured", "hosting_url": s.Engine.HostingURL}
	} else {
		healthStatus["engine"] = fiber.Map{"status": "not_configured"}
	}

	status := 200
	if healthStatus["status"] == "degraded" {
		status = 503
	}

	return c.Status(status).JSON(healthStatus)
}

func (s *OverlayServer) handleListTopicManagers(c *fiber.Ctx) error {
	// TODO: Implement in Phase 4
	return c.JSON(fiber.Map{
		"topicManagers": []string{},
		"message":       "Topic managers will be implemented in Phase 4",
	})
}

func (s *OverlayServer) handleListLookupServiceProviders(c *fiber.Ctx) error {
	// TODO: Implement in Phase 4
	return c.JSON(fiber.Map{
		"lookupServices": []string{},
		"message":        "Lookup services will be implemented in Phase 4",
	})
}

func (s *OverlayServer) handleGetTopicManagerDocs(c *fiber.Ctx) error {
	// TODO: Implement in Phase 4
	return c.JSON(fiber.Map{
		"documentation": "Topic manager documentation will be available in Phase 4",
	})
}

func (s *OverlayServer) handleGetLookupServiceDocs(c *fiber.Ctx) error {
	// TODO: Implement in Phase 4
	return c.JSON(fiber.Map{
		"documentation": "Lookup service documentation will be available in Phase 4",
	})
}

func (s *OverlayServer) handleSubmit(c *fiber.Ctx) error {
	// TODO: Implement TaggedBEEF processing in Phase 3
	return c.JSON(fiber.Map{
		"status":  "placeholder",
		"message": "Submit endpoint will be implemented in Phase 3",
	})
}

func (s *OverlayServer) handleLookup(c *fiber.Ctx) error {
	// TODO: Implement lookup queries in Phase 3
	return c.JSON(fiber.Map{
		"status":  "placeholder",
		"message": "Lookup endpoint will be implemented in Phase 3",
	})
}

func (s *OverlayServer) handleARCIngest(c *fiber.Ctx) error {
	// TODO: Implement ARC webhook processing in Phase 3
	return c.JSON(fiber.Map{
		"status":  "placeholder",
		"message": "ARC ingest endpoint will be implemented in Phase 3",
	})
}

func (s *OverlayServer) handleRequestSyncResponse(c *fiber.Ctx) error {
	// TODO: Implement GASP sync in Phase 4
	return c.JSON(fiber.Map{
		"status":  "placeholder",
		"message": "Sync response endpoint will be implemented in Phase 4",
	})
}

func (s *OverlayServer) handleRequestForeignGASPNode(c *fiber.Ctx) error {
	// TODO: Implement GASP sync in Phase 4
	return c.JSON(fiber.Map{
		"status":  "placeholder",
		"message": "Foreign GASP node endpoint will be implemented in Phase 4",
	})
}

func (s *OverlayServer) handleSyncAdvertisements(c *fiber.Ctx) error {
	// TODO: Implement in Phase 5
	return c.JSON(fiber.Map{
		"status":  "placeholder",
		"message": "Sync advertisements endpoint will be implemented in Phase 5",
	})
}

func (s *OverlayServer) handleStartGASPSync(c *fiber.Ctx) error {
	// TODO: Implement in Phase 5
	return c.JSON(fiber.Map{
		"status":  "placeholder",
		"message": "Start GASP sync endpoint will be implemented in Phase 5",
	})
}

func (s *OverlayServer) handleEvictOutpoint(c *fiber.Ctx) error {
	// TODO: Implement in Phase 5
	return c.JSON(fiber.Map{
		"status":  "placeholder",
		"message": "Evict outpoint endpoint will be implemented in Phase 5",
	})
}

// InitializeDatabases connects and initializes all configured databases
func (s *OverlayServer) InitializeDatabases(ctx context.Context) error {
	// Initialize SQL database if configured
	if s.DB != nil {
		s.Logger.Printf("Connecting to SQL database...")
		if err := s.DB.PingContext(ctx); err != nil {
			s.Logger.Printf("Failed to connect to SQL database: %v", err)
			return fmt.Errorf("SQL database connection failed: %w", err)
		}
		s.Logger.Printf("SQL database connected successfully")
	}

	// Initialize MongoDB if configured
	if s.MongoDB != nil {
		s.Logger.Printf("Connecting to MongoDB...")
		if err := s.MongoDB.Client().Connect(ctx); err != nil {
			s.Logger.Printf("Failed to connect to MongoDB: %v", err)
			return fmt.Errorf("MongoDB connection failed: %w", err)
		}
		if err := s.MongoDB.Client().Ping(ctx, nil); err != nil {
			s.Logger.Printf("Failed to ping MongoDB: %v", err)
			return fmt.Errorf("MongoDB ping failed: %w", err)
		}
		s.Logger.Printf("MongoDB connected successfully")
	}

	return nil
}

// GetPrimaryDatabase returns the primary database connection (SQL preferred over MongoDB)
func (s *OverlayServer) GetPrimaryDatabase() interface{} {
	if s.DB != nil {
		return s.DB
	}
	return s.MongoDB
}

// CheckDatabaseHealth performs health checks on all configured databases
func (s *OverlayServer) CheckDatabaseHealth(ctx context.Context) error {
	if s.DB != nil {
		if err := s.DB.PingContext(ctx); err != nil {
			return fmt.Errorf("SQL database health check failed: %w", err)
		}
	}

	if s.MongoDB != nil {
		if err := s.MongoDB.Client().Ping(ctx, nil); err != nil {
			return fmt.Errorf("MongoDB health check failed: %w", err)
		}
	}

	return nil
}

// Start starts the server
func (s *OverlayServer) Start() error {
	ctx := context.Background()

	// Initialize databases first
	if err := s.InitializeDatabases(ctx); err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// Configure Engine storage with database connections (matching overlay-express pattern)
	if s.Engine != nil {
		if err := s.ConfigureEngineStorage(); err != nil {
			s.Logger.Printf("Warning: Engine storage configuration failed: %v", err)
		}
	}

	if err := s.Setup(); err != nil {
		return err
	}

	s.Logger.Printf("Starting %s on port %d (network: %s)", s.Name, s.Port, s.Network)
	s.Logger.Printf("Server FQDN: %s", s.AdvertisableFQDN)
	s.Logger.Printf("GASP Sync: %t", s.EnableGASPSync)
	s.Logger.Printf("Verbose Logging: %t", s.VerboseLogging)

	// Perform initial health check
	if err := s.CheckDatabaseHealth(ctx); err != nil {
		s.Logger.Printf("Warning: Database health check failed: %v", err)
	}

	return s.App.Listen(fmt.Sprintf(":%d", s.Port))
}

// Stop gracefully stops the server
func (s *OverlayServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Disconnect from databases
	if s.DB != nil {
		if err := s.DB.Close(); err != nil {
			s.Logger.Printf("Error closing SQL database: %v", err)
		}
	}

	if s.MongoDB != nil {
		if err := s.MongoDB.Client().Disconnect(ctx); err != nil {
			s.Logger.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}

	// Shutdown Fiber app
	if s.App != nil {
		return s.App.Shutdown()
	}
	return nil
}
