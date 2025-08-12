package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/b-open-io/overlay/storage"
	"github.com/bsv-blockchain/go-overlay-discovery-services/pkg/ship"
	"github.com/bsv-blockchain/go-overlay-discovery-services/pkg/slap"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/transaction"
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

	// Template manager
	TemplateManager *TemplateManager
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
		TemplateManager:  NewTemplateManager(),
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
func (s *OverlayServer) ConfigureEngine(autoConfigureShipSlap bool) *OverlayServer {
	// Don't configure engine if no databases are available
	if s.DB == nil {
		s.Logger.Printf("Warning: ConfigureEngine called but no SQL database configured")
		return s
	}

	// Get storage configuration from environment
	eventStorageURL := os.Getenv("EVENT_STORAGE")
	beefStorageURL := os.Getenv("BEEF_STORAGE")

	var storage engine.Storage
	var err error

	// Use overlay storage with database connection
	storage, err = CreateOverlayStorageWithDB(eventStorageURL, beefStorageURL, s.DB)
	if err != nil {
		s.Logger.Printf("Failed to create overlay storage: %v", err)
		return s
	}
	s.Logger.Printf("Using overlay storage with EVENT_STORAGE=%s, BEEF_STORAGE=%s", eventStorageURL, beefStorageURL)

	// Create Engine configuration with real storage
	engineConfig := engine.Engine{
		HostingURL:           s.AdvertisableFQDN,
		Managers:             make(map[string]engine.TopicManager),
		LookupServices:       make(map[string]engine.LookupService),
		Storage:              storage,
		LogPrefix:            s.Name,
		ChainTracker:         s.ChainTracker,
		Broadcaster:          s.EngineConfig.Broadcaster,
		Advertiser:           s.EngineConfig.Advertiser,
		SyncConfiguration:    s.EngineConfig.SyncConfiguration,
		BroadcastFacilitator: s.EngineConfig.OverlayBroadcastFacilitator,
	}

	// Apply advanced engine configuration if provided
	if s.EngineConfig.LogTime != nil {
		engineConfig.LogTime = *s.EngineConfig.LogTime
	}
	if s.EngineConfig.ThrowOnBroadcastFailure != nil {
		engineConfig.ErrorOnBroadcastFailure = *s.EngineConfig.ThrowOnBroadcastFailure
	}

	// Copy configured topic managers and lookup services to engine
	for name, manager := range s.Managers {
		engineConfig.Managers[name] = manager
	}
	for name, service := range s.Services {
		engineConfig.LookupServices[name] = service
	}

	// Initialize the Engine with real configuration
	s.Engine = engine.NewEngine(engineConfig)

	// Auto-configure SHIP/SLAP services if requested (like overlay-express)
	if autoConfigureShipSlap {
		s.autoConfigureDiscoveryServices()
	}

	s.Logger.Printf("Engine configured with hosting URL: %s, storage: SQL, managers: %d, services: %d",
		s.AdvertisableFQDN, len(s.Engine.Managers), len(s.Engine.LookupServices))
	return s
}

// autoConfigureDiscoveryServices automatically configures SHIP and SLAP services like overlay-express
func (s *OverlayServer) autoConfigureDiscoveryServices() {
	// Auto-configure SHIP topic manager if not already configured
	if _, exists := s.Managers["tm_ship"]; !exists {
		var shipStorage ship.SHIPStorageInterface
		if s.MongoDB != nil {
			shipStorage = ship.NewSHIPStorage(s.MongoDB)
		}
		shipManager := ship.NewSHIPTopicManager(shipStorage, nil)
		s.ConfigureTopicManager("tm_ship", shipManager)
		s.Logger.Printf("Auto-configured SHIP topic manager")
	}

	// Auto-configure SLAP topic manager if not already configured
	if _, exists := s.Managers["tm_slap"]; !exists {
		var slapStorage slap.SLAPStorageInterface
		if s.MongoDB != nil {
			slapStorage = slap.NewSLAPStorage(s.MongoDB)
		}
		slapManager := slap.NewSLAPTopicManager(slapStorage, nil)
		s.ConfigureTopicManager("tm_slap", slapManager)
		s.Logger.Printf("Auto-configured SLAP topic manager")
	}

	// Auto-configure SHIP lookup service with MongoDB if available
	if s.MongoDB != nil {
		if _, exists := s.Services["ls_ship"]; !exists {
			// TODO: Create proper PushDropDecoder and Utils implementations
			// For now, SHIP/SLAP lookup services require these dependencies
			s.Logger.Printf("SHIP lookup service requires PushDropDecoder and Utils implementations")
		}
	}

	// Auto-configure SLAP lookup service with MongoDB if available
	if s.MongoDB != nil {
		if _, exists := s.Services["ls_slap"]; !exists {
			// TODO: Create proper PushDropDecoder and Utils implementations
			// For now, SHIP/SLAP lookup services require these dependencies
			s.Logger.Printf("SLAP lookup service requires PushDropDecoder and Utils implementations")
		}
	}

	// Ensure the engine knows about all configured services
	if s.Engine != nil {
		// Copy any services that were added after engine creation
		for name, manager := range s.Managers {
			if _, exists := s.Engine.Managers[name]; !exists {
				s.Engine.Managers[name] = manager
				s.Logger.Printf("Added topic manager '%s' to engine", name)
			}
		}
		for name, service := range s.Services {
			if _, exists := s.Engine.LookupServices[name]; !exists {
				s.Engine.LookupServices[name] = service
				s.Logger.Printf("Added lookup service '%s' to engine", name)
			}
		}
	}
}

// Setup initializes the Fiber app and middleware
func (s *OverlayServer) Setup() error {
	// Load HTML templates
	if err := s.TemplateManager.LoadTemplates(); err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

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
	storageType := "unknown"
	if s.Engine != nil {
		if s.Engine.Storage != nil {
			engineStatus = "configured_with_storage"

			// Try to determine storage type
			if _, ok := s.Engine.Storage.(storage.EventDataStorage); ok {
				storageType = "overlay_storage"
			} else {
				storageType = "basic_storage"
			}
		} else {
			engineStatus = "configured_no_storage"
		}
	}

	// Prepare template data
	data := MainPageData{
		Name:           s.Name,
		Network:        s.Network,
		DatabaseStatus: databaseStatus,
		EngineStatus:   engineStatus,
		StatusClass:    getStatusClass(engineStatus),
		StatusText:     getStatusText(engineStatus),
		StorageType:    storageType,
	}

	// Render template
	html, err := s.TemplateManager.RenderTemplate("main", data)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Template rendering failed: " + err.Error(),
		})
	}

	// Set content type to HTML
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// Helper function to get CSS class for status
func getStatusClass(status string) string {
	switch status {
	case "configured_with_storage":
		return "healthy"
	case "configured_no_storage":
		return "warning"
	default:
		return "error"
	}
}

// Helper function to get human-readable status text
func getStatusText(status string) string {
	switch status {
	case "configured_with_storage":
		return "Active"
	case "configured_no_storage":
		return "Configured"
	default:
		return "Not Configured"
	}
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
		engineHealth := fiber.Map{"hosting_url": s.Engine.HostingURL}
		if s.Engine.Storage != nil {
			engineHealth["status"] = "configured_with_storage"
			engineHealth["managers_count"] = len(s.Engine.Managers)
			engineHealth["services_count"] = len(s.Engine.LookupServices)
		} else {
			engineHealth["status"] = "configured_no_storage"
		}
		healthStatus["engine"] = engineHealth
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
	// Render template
	html, err := s.TemplateManager.RenderTemplate("topic-managers", nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Template rendering failed: " + err.Error(),
		})
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

func (s *OverlayServer) handleListLookupServiceProviders(c *fiber.Ctx) error {
	// Render template
	html, err := s.TemplateManager.RenderTemplate("lookup-services", nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Template rendering failed: " + err.Error(),
		})
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

func (s *OverlayServer) handleGetTopicManagerDocs(c *fiber.Ctx) error {
	// Render template
	html, err := s.TemplateManager.RenderTemplate("topic-manager-docs", nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Template rendering failed: " + err.Error(),
		})
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

func (s *OverlayServer) handleGetLookupServiceDocs(c *fiber.Ctx) error {
	// Render template
	html, err := s.TemplateManager.RenderTemplate("lookup-service-docs", nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Template rendering failed: " + err.Error(),
		})
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

func (s *OverlayServer) handleSubmit(c *fiber.Ctx) error {
	// Parse x-topics header like overlay-express
	topicsHeader := c.Get("x-topics")
	if topicsHeader == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "x-topics header required",
		})
	}

	// Split topics by comma
	topics := strings.Split(topicsHeader, ",")
	for i := range topics {
		topics[i] = strings.TrimSpace(topics[i])
	}

	// Create TaggedBEEF from body
	taggedBEEF := overlay.TaggedBEEF{
		Beef:   c.Body(),
		Topics: topics,
	}

	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Create context
	ctx := c.Context()

	// Call Engine.Submit() with parsed data
	// Use historical mode for now - this should be the standard submit mode
	result, err := s.Engine.Submit(ctx, taggedBEEF, engine.SubmitModeHistorical, nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Submit failed: " + err.Error(),
		})
	}

	// Return success with steak information
	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Transaction submitted successfully",
		"steak":   result,
	})
}

func (s *OverlayServer) handleLookup(c *fiber.Ctx) error {
	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Parse JSON body into EventQuestion
	var question storage.EventQuestion
	if err := c.BodyParser(&question); err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid lookup query: " + err.Error(),
		})
	}

	// Validate that we have at least one event to query
	if question.Event == "" && len(question.Events) == 0 {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "At least one event must be specified (event or events field)",
		})
	}

	// Type assert storage to EventDataStorage interface
	eventDataStorage, ok := s.Engine.Storage.(storage.EventDataStorage)
	if !ok {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Storage does not support event-based lookups",
		})
	}

	// Create context from request
	ctx := c.Context()

	// Perform the lookup with data included by default
	results, err := eventDataStorage.LookupOutpoints(ctx, &question, true)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Lookup failed: " + err.Error(),
		})
	}

	// Return successful results
	return c.JSON(fiber.Map{
		"status":  "success",
		"results": results,
		"count":   len(results),
	})
}

// ARCIngestRequest represents the payload structure for ARC webhook ingestion
type ARCIngestRequest struct {
	TxID        string `json:"txid"`
	MerklePath  string `json:"merklePath"`
	BlockHeight uint32 `json:"blockHeight"`
}

func (s *OverlayServer) handleARCIngest(c *fiber.Ctx) error {
	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Parse JSON body into ARCIngestRequest
	var request ARCIngestRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid ARC ingest payload: " + err.Error(),
		})
	}

	// Validate required fields
	if request.TxID == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "txid field is required",
		})
	}

	if request.MerklePath == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "merklePath field is required",
		})
	}

	if request.BlockHeight == 0 {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "blockHeight must be a positive integer (greater than 0)",
		})
	}

	// Parse transaction ID from hex
	txid, err := chainhash.NewHashFromHex(request.TxID)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid transaction ID format: " + err.Error(),
		})
	}

	// Parse merkle path from hex
	merklePath, err := transaction.NewMerklePathFromHex(request.MerklePath)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid merkle path format: " + err.Error(),
		})
	}

	// Set block height on merkle path
	merklePath.BlockHeight = request.BlockHeight

	// Create context from request
	ctx := c.Context()

	// Call Engine.HandleNewMerkleProof
	err = s.Engine.HandleNewMerkleProof(ctx, txid, merklePath)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Failed to process merkle proof: " + err.Error(),
		})
	}

	// Return success response
	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Transaction status updated successfully",
		"txid":    request.TxID,
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

		// Run database migrations using storage
		storage, err := NewSQLStorage(s.DB)
		if err != nil {
			s.Logger.Printf("Error creating storage for migrations: %v", err)
			return fmt.Errorf("failed to create storage for migrations: %w", err)
		}
		if err := storage.RunMigrations(ctx, s.MigrationsToRun, s.Logger); err != nil {
			s.Logger.Printf("Database migration failed: %v", err)
			return fmt.Errorf("database migration failed: %w", err)
		}
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

	// Engine should already be configured with storage from ConfigureEngine call

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
