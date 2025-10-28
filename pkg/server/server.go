package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/b-open-io/overlay/config"
	"github.com/b-open-io/overlay/headers"
	"github.com/b-open-io/overlay/pubsub"
	"github.com/b-open-io/overlay/storage"
	"github.com/bsv-blockchain/go-overlay-discovery-services/pkg/advertiser"
	"github.com/bsv-blockchain/go-overlay-discovery-services/pkg/ship"
	"github.com/bsv-blockchain/go-overlay-discovery-services/pkg/slap"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/gasp"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/overlay"
	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
	"github.com/bsv-blockchain/go-sdk/util"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
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
	Logger         *slog.Logger
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
	Managers     map[string]engine.TopicManager
	Services     map[string]engine.LookupService
	ChainTracker chaintracker.ChainTracker
	WebUIConfig  UIConfig
	EngineConfig EngineConfig

	// Fiber app
	App *fiber.App

	// Template manager
	TemplateManager *TemplateManager

	// Queue processing and real-time features
	QueueManager     *QueueManager
	WebSocketManager *WebSocketManager
}

// NewOverlayServer creates a new OverlayServer instance
// adminToken is optional - if empty, a random UUID will be generated
func NewOverlayServer(name, privateKey, fqdn string, adminToken ...string) *OverlayServer {
	// Generate random admin token if not provided
	var token string
	if len(adminToken) > 0 && adminToken[0] != "" {
		token = adminToken[0]
	} else {
		token = uuid.New().String()
	}

	if !strings.HasSuffix(fqdn, "/") {
		// Ensure the hosting URL ends with a slash
		fqdn += "/"
	}

	return &OverlayServer{
		Name:             name,
		PrivateKey:       privateKey,
		AdvertisableFQDN: fqdn,
		AdminToken:       token,
		Port:             3000,
		Network:          "main",
		Logger:           slog.Default(),
		VerboseLogging:   false,
		EnableGASPSync:   true,
		Managers:         make(map[string]engine.TopicManager),
		Services:         make(map[string]engine.LookupService),
		TemplateManager:  NewTemplateManager(),
	}
}

// ConfigurePort sets the server port
func (s *OverlayServer) ConfigurePort(port int) *OverlayServer {
	s.Port = port
	return s
}

// ConfigureLogger sets the server logger
func (s *OverlayServer) ConfigureLogger(logger *slog.Logger) *OverlayServer {
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
		s.Logger.Warn("No SQL database connection string provided")
		return s
	}

	// Open database connection
	db, err := sql.Open(driverName, connectionString)
	if err != nil {
		s.Logger.Error("Failed to open SQL database connection", "error", err)
		return s
	}

	// Configure connection pool for optimal performance
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	s.DB = db
	s.Logger.Info("SQL database configured", "connection", connectionString, "driver", driverName)
	return s
}

// ConfigureMongoDB establishes MongoDB connection
func (s *OverlayServer) ConfigureMongoDB(connectionString string) *OverlayServer {
	return s.ConfigureMongoDBWithDatabase(connectionString, "overlay")
}

// ConfigureMongoDBWithDatabase establishes MongoDB connection with custom database name
func (s *OverlayServer) ConfigureMongoDBWithDatabase(connectionString, database string) *OverlayServer {
	if connectionString == "" {
		s.Logger.Error("No MongoDB connection string provided")
		return s
	}

	// Create MongoDB client options
	clientOptions := options.Client().ApplyURI(connectionString)
	clientOptions.SetMaxPoolSize(100)
	clientOptions.SetMaxConnIdleTime(5 * time.Minute)
	clientOptions.SetConnectTimeout(30 * time.Second)
	clientOptions.SetServerSelectionTimeout(30 * time.Second)

	// Create MongoDB client connection
	ctx := context.Background()
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		s.Logger.Error("Failed to create MongoDB client", "error", err)
		return s
	}

	// Get database reference - ready for use by engine/services
	s.MongoDB = client.Database(database)
	s.Logger.Info("MongoDB configured", "connection", connectionString, "database", database)
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
	s.Logger.Info("Topic manager configured", "name", name)
	return s
}

// ConfigureLookupService stores a lookup service with the given name
func (s *OverlayServer) ConfigureLookupService(name string, service engine.LookupService) *OverlayServer {
	s.Services[name] = service
	s.Logger.Info("Lookup service configured", "name", name)
	return s
}

// ConfigureChainTracker sets the chain tracker
func (s *OverlayServer) ConfigureChainTracker(chainTracker chaintracker.ChainTracker) *OverlayServer {
	s.ChainTracker = chainTracker
	s.Logger.Info("Chain tracker configured")
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
		s.Logger.Warn("ConfigureEngine called but no SQL database configured")
		return s
	}

	// Get storage configuration from environment
	eventStorageURL := os.Getenv("EVENT_STORAGE")
	beefStorageURL := os.Getenv("BEEF_STORAGE")

	// Use overlay storage with database connection
	eventStorage, err := config.CreateEventStorage(eventStorageURL, beefStorageURL, "", "", nil)
	if err != nil {
		s.Logger.Error("Failed to create overlay storage", "error", err)
		return s
	}
	s.Logger.Info("Using overlay storage with", "EVENT_STORAGE", eventStorageURL, "BEEF_STORAGE", beefStorageURL)

	// Initialize SyncConfiguration if needed for GASP sync
	syncConfig := s.EngineConfig.SyncConfiguration
	if syncConfig == nil {
		syncConfig = make(map[string]engine.SyncConfiguration)
	}

	// Set default SLAP trackers if not provided in engine config
	slapTrackers := s.EngineConfig.SlapTrackers
	if len(slapTrackers) == 0 {
		if s.Network == "test" {
			slapTrackers = lookup.DEFAULT_TESTNET_SLAP_TRACKERS
		} else {
			slapTrackers = lookup.DEFAULT_SLAP_TRACKERS
		}
		s.Logger.Info("Using default SLAP trackers for network", "network", s.Network, "slap-trackers", slapTrackers)
	}

	// Prepare advertiser if not set by the user
	var adv = s.EngineConfig.Advertiser
	if adv == nil {
		// Create AuthenticatedWalletAdvertiser for production storage services
		storageURL := "https://storage.babbage.systems"
		if s.Network == "test" {
			storageURL = "https://staging-storage.babbage.systems"
		}

		// Use WalletAdvertiser from go-overlay-discovery-services
		walletAdv, err := advertiser.NewWalletAdvertiser(
			s.Network,
			s.PrivateKey,
			storageURL,
			s.AdvertisableFQDN,
			nil, // lookupResolverConfig - can be nil for basic functionality
		)
		if err != nil {
			s.Logger.Error("Failed to create WalletAdvertiser for FQDN - SHIP and SLAP will be disabled.", "fqdn", s.AdvertisableFQDN, "error", err)
		} else {
			adv = walletAdv
			s.Logger.Info("WalletAdvertiser initialized for FQDN", "fqdn", s.AdvertisableFQDN)
		}
	}

	// Create Engine configuration with real storage
	engineConfig := engine.Engine{
		HostingURL:           s.AdvertisableFQDN,
		Managers:             make(map[string]engine.TopicManager),
		LookupServices:       make(map[string]engine.LookupService),
		Storage:              eventStorage,
		LogPrefix:            s.Name,
		ChainTracker:         s.ChainTracker,
		Broadcaster:          s.EngineConfig.Broadcaster,
		Advertiser:           adv,
		SyncConfiguration:    syncConfig,
		SLAPTrackers:         slapTrackers,
		BroadcastFacilitator: s.EngineConfig.OverlayBroadcastFacilitator,
	}

	// Apply advanced engine configuration if provided
	if s.EngineConfig.LogTime != nil {
		engineConfig.LogTime = *s.EngineConfig.LogTime
	}
	if s.EngineConfig.ThrowOnBroadcastFailure != nil {
		engineConfig.ErrorOnBroadcastFailure = *s.EngineConfig.ThrowOnBroadcastFailure
	}

	if engineConfig.ChainTracker == nil {
		engineConfig.ChainTracker = headers.NewClient(headers.ClientParams{
			Url: "https://mainnet.headers.gorillapool.io",
		})
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

	// Auto-configure SHIP/SLAP services if requested
	if autoConfigureShipSlap {
		if _, ok := s.Engine.SyncConfiguration["tm_ship"]; !ok {
			s.Engine.SyncConfiguration["tm_ship"] = engine.SyncConfiguration{
				Type:  engine.SyncConfigurationSHIP,
				Peers: []string{"https://overlay-us-1.bsvb.tech"},
			}
		}
		s.autoConfigureDiscoveryServices()
	}

	s.Logger.Info("Engine configured with storage: SQL", "HostingURL",
		s.AdvertisableFQDN, "managers", len(s.Engine.Managers), "services", len(s.Engine.LookupServices))

	// Log all configured managers and services
	s.Logger.Debug("Configured topic managers:")
	for name, manager := range s.Engine.Managers {
		s.Logger.Debug("topic manager", "name", name, "manager", manager)
	}
	s.Logger.Debug("Configured lookup services:")
	for name, service := range s.Engine.LookupServices {
		s.Logger.Debug("lookup service", "name", name, "service", service)
	}

	// Initialize queue processing and WebSocket management
	if err := s.initializeBackgroundServices(); err != nil {
		s.Logger.Error("Failed to initialize background services", "error", err)
	}

	return s
}

// autoConfigureDiscoveryServices automatically configures SHIP and SLAP services
func (s *OverlayServer) autoConfigureDiscoveryServices() {
	// Auto-configure SHIP topic manager if not already configured
	if _, exists := s.Managers["tm_ship"]; !exists {
		var shipStorage ship.StorageInterface
		if s.MongoDB != nil {
			shipStorage = ship.NewStorage(s.MongoDB)
		}
		shipManager := ship.NewTopicManager(shipStorage, nil)
		s.ConfigureTopicManager("tm_ship", shipManager)
		s.Logger.Info("Auto-configured SHIP topic manager")
	}

	// Auto-configure SLAP topic manager if not already configured
	if _, exists := s.Managers["tm_slap"]; !exists {
		var slapStorage slap.StorageInterface
		if s.MongoDB != nil {
			slapStorage = slap.NewStorage(s.MongoDB)
		}
		slapManager := slap.NewTopicManager(slapStorage, nil)
		s.ConfigureTopicManager("tm_slap", slapManager)
		s.Logger.Info("Auto-configured SLAP topic manager")
	}

	// Auto-configure SHIP lookup service with MongoDB if available
	if s.MongoDB != nil {
		if _, exists := s.Services["ls_ship"]; !exists {
			// Create SHIP storage and lookup service
			shipStorage := ship.NewStorage(s.MongoDB)
			shipLookupService := ship.NewLookupService(shipStorage)

			s.ConfigureLookupService("ls_ship", shipLookupService)
			s.Logger.Info("Auto-configured SHIP lookup service with MongoDB")
		}
	}

	// Auto-configure SLAP lookup service with MongoDB if available
	if s.MongoDB != nil {
		if _, exists := s.Services["ls_slap"]; !exists {
			// Create SLAP storage and lookup service
			slapStorage := slap.NewStorage(s.MongoDB)
			slapLookupService := slap.NewLookupService(slapStorage)

			s.ConfigureLookupService("ls_slap", slapLookupService)
			s.Logger.Info("Auto-configured SLAP lookup service with MongoDB")
		}
	}

	// Configure sync configuration when GASP sync is enabled
	if s.EnableGASPSync && s.Engine != nil {
		// Initialize SyncConfiguration map if needed
		if s.Engine.SyncConfiguration == nil {
			s.Engine.SyncConfiguration = make(map[string]engine.SyncConfiguration)
		}

		// Configure tm_ship for SHIP discovery sync if it was auto-configured
		if _, exists := s.Managers["tm_ship"]; exists {
			if _, syncExists := s.Engine.SyncConfiguration["tm_ship"]; !syncExists {
				s.Engine.SyncConfiguration["tm_ship"] = engine.SyncConfiguration{
					Type:        engine.SyncConfigurationSHIP,
					Concurrency: 1,
				}
				s.Logger.Info("Auto-configured SHIP sync for tm_ship topic manager")
			}
		}

		// Configure tm_slap for SHIP discovery sync if it was auto-configured
		if _, exists := s.Managers["tm_slap"]; exists {
			if _, syncExists := s.Engine.SyncConfiguration["tm_slap"]; !syncExists {
				s.Engine.SyncConfiguration["tm_slap"] = engine.SyncConfiguration{
					Type:        engine.SyncConfigurationSHIP,
					Concurrency: 1,
				}
				s.Logger.Info("Auto-configured SHIP sync for tm_slap topic manager")
			}
		}
	}

	// Ensure the engine knows about all configured services
	if s.Engine != nil {
		// Copy any services that were added after engine creation
		for name, manager := range s.Managers {
			if _, exists := s.Engine.Managers[name]; !exists {
				s.Engine.Managers[name] = manager
				s.Logger.Info("Added topic manager to engine", "name", name)
			}
		}
		for name, service := range s.Services {
			if _, exists := s.Engine.LookupServices[name]; !exists {
				s.Engine.LookupServices[name] = service
				s.Logger.Info("Added lookup service to engine", "name", name)
			}
		}
	}
}

// initializeBackgroundServices initializes queue processing and WebSocket management
func (s *OverlayServer) initializeBackgroundServices() error {
	// TODO: Add publisher to OverlayServer and get from there
	// Get publisher from overlay storage adapter
	var publisher pubsub.PubSub
	if overlayAdapter, ok := s.Engine.Storage.(*storage.EventDataStorage); ok {
		publisher = overlayAdapter.GetPubSub()
	}

	// Initialize queue manager
	queueManager, err := NewQueueManager(s.Engine, publisher, s.Logger)
	if err != nil {
		return fmt.Errorf("failed to create queue manager: %w", err)
	}
	s.QueueManager = queueManager

	// Initialize WebSocket manager
	webSocketManager, err := NewWebSocketManager(publisher, s.Logger)
	if err != nil {
		return fmt.Errorf("failed to create WebSocket manager: %w", err)
	}
	s.WebSocketManager = webSocketManager

	s.Logger.Info("Background services initialized successfully")
	return nil
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
	s.App.Get("/", s.handleDynamicInterface)   // Root path serves dynamic interface
	s.App.Get("/ui", s.handleDynamicInterface) // Keep for backward compatibility
	s.App.Get("/services", s.handleWebUI)      // Move dashboard to /services
	s.App.Get("/dashboard", s.handleDashboard)
	s.App.Get("/test", s.handleAPITester)
	s.App.Get("/health", s.handleHealthCheck)
	s.App.Get("/listTopicManagers", s.handleListTopicManagers)
	s.App.Get("/listLookupServiceProviders", s.handleListLookupServiceProviders)
	s.App.Get("/getDocumentationForTopicManager", s.handleGetTopicManagerDocs)
	s.App.Get("/getDocumentationForLookupServiceProvider", s.handleGetLookupServiceDocs)
	s.App.Post("/submit", s.handleSubmit)
	s.App.Post("/lookup", s.handleLookup)

	// WebSocket endpoint for real-time events
	s.App.Get("/ws", s.handleWebSocketUpgrade)

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

// Route handlers

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
			if _, ok := s.Engine.Storage.(*storage.EventDataStorage); ok {
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

func (s *OverlayServer) handleDashboard(c *fiber.Ctx) error {
	format := c.Query("format", "html")

	// Collect comprehensive dashboard data
	dashboardData := s.collectDashboardData()

	// Return JSON if requested for AJAX updates
	if format == "json" {
		return c.JSON(dashboardData)
	}

	// Render HTML template
	html, err := s.TemplateManager.RenderTemplate("dashboard", dashboardData)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Dashboard template rendering failed: " + err.Error(),
		})
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

func (s *OverlayServer) collectDashboardData() *DashboardData {
	ctx := context.Background()

	// Basic server information
	data := &DashboardData{
		Name:                 s.Name,
		Network:              s.Network,
		FQDN:                 s.AdvertisableFQDN,
		Port:                 s.Port,
		Timestamp:            time.Now().Format("2006-01-02 15:04:05 MST"),
		AdminTokenConfigured: s.AdminToken != "",
		GASPSyncEnabled:      s.EnableGASPSync,
	}

	// Queue Manager status
	if s.QueueManager != nil {
		data.QueueManager = s.QueueManager.GetStatus()
	} else {
		data.QueueManager = map[string]interface{}{"status": "not_configured"}
	}

	// WebSocket Manager status
	if s.WebSocketManager != nil {
		data.WebSocketManager = s.WebSocketManager.GetStats()
	} else {
		data.WebSocketManager = map[string]interface{}{"status": "not_configured"}
	}

	// Database status
	data.Databases = make(map[string]interface{})
	if s.DB != nil {
		if err := s.DB.PingContext(ctx); err != nil {
			data.Databases["sql"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
		} else {
			data.Databases["sql"] = map[string]interface{}{"status": "healthy"}
		}
	}

	if s.MongoDB != nil {
		if err := s.MongoDB.Client().Ping(ctx, nil); err != nil {
			data.Databases["mongodb"] = map[string]interface{}{"status": "unhealthy", "error": err.Error()}
		} else {
			data.Databases["mongodb"] = map[string]interface{}{"status": "healthy"}
		}
	}

	// Engine status
	if s.Engine != nil {
		data.Engine = map[string]interface{}{
			"hosting_url":    s.Engine.HostingURL,
			"managers_count": len(s.Engine.Managers),
			"services_count": len(s.Engine.LookupServices),
		}

		// Storage information
		if s.Engine.Storage != nil {
			if _, ok := s.Engine.Storage.(*storage.EventDataStorage); ok {
				data.StorageType = "overlay_storage"
				data.StorageStatus = "active"
				data.StorageConfig = map[string]interface{}{
					"event_storage": os.Getenv("EVENT_STORAGE"),
					"beef_storage":  os.Getenv("BEEF_STORAGE"),
				}
			} else {
				data.StorageType = "basic_storage"
				data.StorageStatus = "active"
			}
		} else {
			data.StorageType = "none"
			data.StorageStatus = "not_configured"
		}
	} else {
		data.Engine = map[string]interface{}{"status": "not_configured"}
		data.StorageType = "none"
		data.StorageStatus = "not_configured"
	}

	return data
}

func (s *OverlayServer) handleDynamicInterface(c *fiber.Ctx) error {
	// Determine storage info
	storageInfo := "Unknown"
	if s.Engine != nil && s.Engine.Storage != nil {
		if _, ok := s.Engine.Storage.(*storage.EventDataStorage); ok {
			storageInfo = "Overlay Storage"
		} else {
			storageInfo = "Basic Storage"
		}
	}

	// Determine protocol for WebSocket URL
	protocol := "ws://"
	if c.Secure() {
		protocol = "wss://"
	}

	// Prepare template data
	data := &DynamicInterfaceData{
		Name:         s.Name,
		Network:      s.Network,
		FQDN:         s.AdvertisableFQDN,
		StorageInfo:  storageInfo,
		BaseURL:      fmt.Sprintf("http://%s", c.Get("Host")),
		WebSocketURL: fmt.Sprintf("%s%s/ws", protocol, c.Get("Host")),
	}

	// Use HTTPS if secure
	if c.Secure() {
		data.BaseURL = fmt.Sprintf("https://%s", c.Get("Host"))
	}

	// Render template
	html, err := s.TemplateManager.RenderTemplate("dynamic-interface", data)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Dynamic interface template rendering failed: " + err.Error(),
		})
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

func (s *OverlayServer) handleAPITester(c *fiber.Ctx) error {
	// Determine protocol for WebSocket URL
	protocol := "ws://"
	if c.Secure() {
		protocol = "wss://"
	}

	// Prepare template data
	data := &APITesterData{
		Name:         s.Name,
		WebSocketURL: fmt.Sprintf("%s%s/ws", protocol, c.Get("Host")),
	}

	// Render template
	html, err := s.TemplateManager.RenderTemplate("api-tester", data)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "API tester template rendering failed: " + err.Error(),
		})
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

func (s *OverlayServer) handleWebSocketUpgrade(c *fiber.Ctx) error {
	if s.WebSocketManager == nil {
		return c.Status(503).JSON(ErrorResponse{
			Status:  "error",
			Message: "WebSocket support not available",
		})
	}

	return s.WebSocketManager.HandleWebSocketUpgrade(c)
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

	// Check queue manager status
	if s.QueueManager != nil {
		healthStatus["queue_manager"] = s.QueueManager.GetStatus()
	} else {
		healthStatus["queue_manager"] = fiber.Map{"status": "not_configured"}
	}

	// Check WebSocket manager status
	if s.WebSocketManager != nil {
		healthStatus["websocket_manager"] = s.WebSocketManager.GetStats()
	} else {
		healthStatus["websocket_manager"] = fiber.Map{"status": "not_configured"}
	}

	status := 200
	if healthStatus["status"] == "degraded" {
		status = 503
	}

	return c.Status(status).JSON(healthStatus)
}

func (s *OverlayServer) handleListTopicManagers(c *fiber.Ctx) error {
	acceptHeader := c.Get("Accept")
	wantsJSON := strings.Contains(acceptHeader, "application/json") || c.Query("format") == "json"

	// Collect topic managers data
	managers := make(map[string]*overlay.MetaData)

	if s.Engine != nil && s.Engine.Managers != nil {
		for name, manager := range s.Engine.Managers {
			managers[name] = manager.GetMetaData()
		}
	}

	// Return JSON if requested
	if wantsJSON {
		return c.JSON(managers)
	}

	// Otherwise render HTML template
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
	acceptHeader := c.Get("Accept")
	wantsJSON := strings.Contains(acceptHeader, "application/json") || c.Query("format") == "json"

	// Collect lookup service providers data
	providers := make(map[string]*overlay.MetaData)

	if s.Engine != nil && s.Engine.LookupServices != nil {
		for name, service := range s.Engine.LookupServices {
			providers[name] = service.GetMetaData()
		}
	}

	// Return JSON if requested
	if wantsJSON {
		return c.JSON(providers)
	}

	// Otherwise render HTML template
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
	// Get the manager name from query parameter
	managerName := c.Query("manager", "")
	if managerName == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "manager query parameter is required",
		})
	}

	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Use the generic interface method to get documentation
	documentation, err := s.Engine.GetDocumentationForTopicManager(managerName)
	if err != nil {
		return c.Status(404).JSON(ErrorResponse{
			Status:  "error",
			Message: "Documentation not found for topic manager: " + managerName,
		})
	}

	// Return raw markdown with text/markdown content type
	c.Set("Content-Type", "text/markdown")
	return c.SendString(documentation)
}

func (s *OverlayServer) handleGetLookupServiceDocs(c *fiber.Ctx) error {
	// Get the provider name from query parameter
	providerName := c.Query("provider", "")
	if providerName == "" {
		// Try legacy parameter name for compatibility
		providerName = c.Query("lookupService", "")
	}
	if providerName == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "provider or lookupService query parameter is required",
		})
	}

	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Use the generic interface method to get documentation
	documentation, err := s.Engine.GetDocumentationForLookupServiceProvider(providerName)
	if err != nil {
		return c.Status(404).JSON(ErrorResponse{
			Status:  "error",
			Message: "Documentation not found for lookup service provider: " + providerName,
		})
	}

	// Return raw markdown with text/markdown content type
	c.Set("Content-Type", "text/markdown")
	return c.SendString(documentation)
}

func (s *OverlayServer) handleSubmit(c *fiber.Ctx) error {
	// Parse x-topics header
	topicsHeader := c.Get("x-topics")
	if topicsHeader == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "x-topics header required",
		})
	}

	// Parse topics as JSON
	var topics []string
	if err := json.Unmarshal([]byte(topicsHeader), &topics); err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid x-topics header format (must be valid JSON array): " + err.Error(),
		})
	}

	s.Logger.Debug("Submit received topics", "topics", topics)

	// Check for x-includes-off-chain-values header
	includesOffChain := c.Get("x-includes-off-chain-values") == "true"

	// Check if Engine is configured first
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Process BEEF data and extract off-chain values if present
	var beef []byte
	var offChainValues []byte
	bodyData := c.Body()

	if includesOffChain {
		// Extract BEEF and off-chain values using Reader
		reader := util.NewReader(bodyData)
		beefLength, err := reader.ReadVarInt()
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{
				Status:  "error",
				Message: "Failed to read BEEF length from off-chain data: " + err.Error(),
			})
		}

		beef, err = reader.ReadBytes(int(beefLength))
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{
				Status:  "error",
				Message: "Failed to read BEEF data: " + err.Error(),
			})
		}

		// Read remaining bytes as off-chain values
		remainingData := reader.ReadRemaining()
		if len(remainingData) > 0 {
			offChainValues = remainingData
		}
	} else {
		// No off-chain values, use full body as BEEF
		beef = bodyData
	}

	// Create TaggedBEEF from processed data
	taggedBEEF := overlay.TaggedBEEF{
		Beef:           beef,
		Topics:         topics,
		OffChainValues: offChainValues,
	}

	s.Logger.Debug("TaggedBEEF created with", "topics", topics, "BEEF length", len(beef))

	// Create context
	ctx := c.Context()

	// Using a callback function, we can return once the STEAK is ready
	var responseSent bool
	s.Logger.Debug("Calling s.Engine.Submit with topics", "topics", topics)
	result, err := s.Engine.Submit(ctx, taggedBEEF, engine.SubmitModeHistorical, func(steak *overlay.Steak) {
		for topic, instructions := range *steak {
			if instructions != nil {
				s.Logger.Debug("Topic instructions", "topic", topic, "OutputsToAdmit", instructions.OutputsToAdmit,
					"CoinsToRetain", instructions.CoinsToRetain, "CoinsRemoved", instructions.CoinsRemoved)
			} else {
				s.Logger.Debug("Topic instructions is nil", "topic", topic)
			}
		}
		if !responseSent {
			responseSent = true
			_ = c.JSON(steak)
		}
	})
	s.Logger.Debug("s.Engine.Submit completed", "error", err, "result", result)
	if err != nil {
		if !responseSent {
			return c.Status(500).JSON(ErrorResponse{
				Status:  "error",
				Message: "Submit failed: " + err.Error(),
			})
		}
		// If callback already sent response, just log the error
		s.Logger.Error("Submit error after callback response", "error", err)
		return nil
	}

	// Parse BEEF to get transaction ID (after successful engine submission)
	_, _, txid, err := transaction.ParseBeef(beef)
	if err != nil {
		// If BEEF parsing fails but engine submission succeeded, log warning but continue
		s.Logger.Error("BEEF parsing failed after successful submission", "error", err)
		txid = nil
	}

	// Enqueue transaction for background processing if queue manager is available
	if s.QueueManager != nil && txid != nil {
		// Enqueue with default values - in production, these would come from the submission context
		if err := s.QueueManager.EnqueueTransaction(txid, 0, 0); err != nil {
			s.Logger.Error("Failed to enqueue transaction for processing", "error", err)
		}
	}

	// Broadcast submission event via WebSocket if available
	if s.WebSocketManager != nil && txid != nil {
		submissionEvent := WebSocketMessage{
			Type:  "transaction_submitted",
			Topic: "all",
			Data: fiber.Map{
				"txid":   txid.String(),
				"topics": topics,
				"steak":  result,
			},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		s.WebSocketManager.BroadcastToAll(submissionEvent)
	}

	// Return success with steak information if callback hasn't already sent response
	if !responseSent {
		return c.JSON(result)
	}

	// Response already sent by callback
	return nil
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
	eventDataStorage, ok := s.Engine.Storage.(*storage.EventDataStorage)
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

// EvictOutpointRequest represents the payload structure for evict outpoint request
type EvictOutpointRequest struct {
	TxID        string `json:"txid"`
	OutputIndex uint32 `json:"outputIndex"`
	Service     string `json:"service,omitempty"` // Optional: specific service to evict from
}

// ForeignGASPNodeRequest represents the payload structure for foreign GASP node request
type ForeignGASPNodeRequest struct {
	GraphID     string `json:"graphID"`     // Transaction ID for graph ID
	TxID        string `json:"txid"`        // Transaction ID for the node
	OutputIndex uint32 `json:"outputIndex"` // Output index for the node
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
	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Parse x-bsv-topic header (required for GASP sync)
	topic := c.Get("x-bsv-topic")
	if topic == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "x-bsv-topic header is required",
		})
	}

	// Parse JSON body into GASP InitialRequest
	var initialRequest gasp.InitialRequest
	if err := c.BodyParser(&initialRequest); err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid GASP initial request payload: " + err.Error(),
		})
	}

	// Create context from request
	ctx := c.Context()

	// Call Engine.ProvideForeignSyncResponse()
	response, err := s.Engine.ProvideForeignSyncResponse(ctx, &initialRequest, topic)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Failed to provide foreign sync response: " + err.Error(),
		})
	}

	// Return the InitialResponse as JSON
	return c.JSON(response)
}

func (s *OverlayServer) handleRequestForeignGASPNode(c *fiber.Ctx) error {
	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Parse JSON body into ForeignGASPNodeRequest
	var request ForeignGASPNodeRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid foreign GASP node request payload: " + err.Error(),
		})
	}

	// Validate required fields
	if request.GraphID == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "graphID field is required",
		})
	}
	if request.TxID == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "txid field is required",
		})
	}

	// Parse transaction IDs from hex
	graphIDHash, err := chainhash.NewHashFromHex(request.GraphID)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid graphID format: " + err.Error(),
		})
	}

	txidHash, err := chainhash.NewHashFromHex(request.TxID)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid txid format: " + err.Error(),
		})
	}

	// Create outpoints
	graphIDOutpoint := &transaction.Outpoint{
		Txid:  *graphIDHash,
		Index: 0, // Graph ID typically uses output index 0
	}

	outpoint := &transaction.Outpoint{
		Txid:  *txidHash,
		Index: request.OutputIndex,
	}

	// Parse x-bsv-topic header (required for GASP sync)
	topic := c.Get("x-bsv-topic")
	if topic == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "x-bsv-topic header is required",
		})
	}

	// Create context from request
	ctx := c.Context()

	// Call Engine.ProvideForeignGASPNode()
	node, err := s.Engine.ProvideForeignGASPNode(ctx, graphIDOutpoint, outpoint, topic)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Failed to provide foreign GASP node: " + err.Error(),
		})
	}

	// Return the Node as JSON
	return c.JSON(node)
}

func (s *OverlayServer) handleSyncAdvertisements(c *fiber.Ctx) error {
	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Create context from request
	ctx := c.Context()

	// Call Engine.SyncAdvertisements()
	err := s.Engine.SyncAdvertisements(ctx)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Failed to sync advertisements: " + err.Error(),
		})
	}

	// Return success response
	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Advertisements synced successfully",
	})
}

func (s *OverlayServer) handleStartGASPSync(c *fiber.Ctx) error {
	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Create context from request
	ctx := c.Context()

	// Call Engine.StartGASPSync()
	err := s.Engine.StartGASPSync(ctx)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Failed to start GASP sync: " + err.Error(),
		})
	}

	// Return success response
	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "GASP sync started and completed",
	})
}

func (s *OverlayServer) handleEvictOutpoint(c *fiber.Ctx) error {
	// Check if Engine is configured
	if s.Engine == nil {
		return c.Status(500).JSON(ErrorResponse{
			Status:  "error",
			Message: "Engine not configured",
		})
	}

	// Parse JSON body into EvictOutpointRequest
	var request EvictOutpointRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "Invalid evict outpoint payload: " + err.Error(),
		})
	}

	// Validate required fields
	if request.TxID == "" {
		return c.Status(400).JSON(ErrorResponse{
			Status:  "error",
			Message: "txid field is required",
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

	// Create outpoint
	outpoint := &transaction.Outpoint{
		Txid:  *txid,
		Index: request.OutputIndex,
	}

	// Create context from request
	ctx := c.Context()

	// If specific service is provided, evict from that service only
	if request.Service != "" {
		service, exists := s.Engine.LookupServices[request.Service]
		if !exists {
			return c.Status(404).JSON(ErrorResponse{
				Status:  "error",
				Message: "Service not found: " + request.Service,
			})
		}

		err := service.OutputEvicted(ctx, outpoint)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{
				Status:  "error",
				Message: "Failed to evict outpoint from service: " + err.Error(),
			})
		}
	} else {
		// Evict from all services
		for serviceName, service := range s.Engine.LookupServices {
			if err := service.OutputEvicted(ctx, outpoint); err != nil {
				// Log error but continue with other services
				s.Logger.Error("Failed to evict outpoint from service", "service", serviceName, "error", err)
			}
		}
	}

	// Return success response
	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Outpoint evicted",
	})
}

// InitializeDatabases connects and initializes all configured databases
func (s *OverlayServer) InitializeDatabases(ctx context.Context) error {
	// Initialize SQL database if configured
	if s.DB != nil {
		s.Logger.Info("Connecting to SQL database...")
		if err := s.DB.PingContext(ctx); err != nil {
			s.Logger.Error("Failed to connect to SQL database", "error", err)
			return fmt.Errorf("SQL database connection failed: %w", err)
		}
		s.Logger.Info("SQL database connected successfully")
	}

	// Initialize MongoDB if configured
	if s.MongoDB != nil {
		s.Logger.Info("Connecting to MongoDB...")
		if err := s.MongoDB.Client().Ping(ctx, nil); err != nil {
			s.Logger.Error("Failed to ping MongoDB", "error", err)
			return fmt.Errorf("MongoDB ping failed: %w", err)
		}
		s.Logger.Info("MongoDB connected successfully")
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

	s.Logger.Info("Starting Overlay Server", "name", s.Name, "port", s.Port, "network", s.Network)
	s.Logger.Info("Server Info", "FQDN", s.AdvertisableFQDN,
		"GASP Sync", s.EnableGASPSync, "Verbose Logging", s.VerboseLogging)

	// Start background services
	if s.QueueManager != nil {
		if err := s.QueueManager.Start(); err != nil {
			s.Logger.Error("Failed to start queue manager", "error", err)
		}
	}

	if s.WebSocketManager != nil {
		if err := s.WebSocketManager.Start(); err != nil {
			s.Logger.Error("Failed to start WebSocket manager", "error", err)
		}
	}

	// Perform initial health check
	if err := s.CheckDatabaseHealth(ctx); err != nil {
		s.Logger.Error("Database health check failed", "error", err)
	}

	// Automatic startup synchronization
	if s.Engine != nil {
		if s.Engine.Advertiser != nil {
			if a, ok := s.Engine.Advertiser.(*advertiser.WalletAdvertiser); ok {
				if err := a.Init(); err != nil {
					s.Logger.Error("Failed to initialize wallet advertiser", "error", err)
				} else {
					s.Logger.Info("Wallet advertiser initialized successfully")
				}
			}
		}

		// Attempt to sync advertisements
		if err := s.Engine.SyncAdvertisements(ctx); err != nil {
			s.Logger.Error("Error syncing advertisements", "error", err)
		}

		// Attempt to do GASP sync if enabled
		if s.EnableGASPSync {
			s.Logger.Info("Starting GASP sync...")
			if err := s.Engine.StartGASPSync(ctx); err != nil {
				s.Logger.Error("Failed to GASP sync", "error", err)
			} else {
				s.Logger.Info("GASP sync complete!")
			}
		} else {
			s.Logger.Warn("Will not sync because GASP has been disabled", "name", s.Name)
		}
	}

	return s.App.Listen(fmt.Sprintf(":%d", s.Port))
}

// Stop gracefully stops the server
func (s *OverlayServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop background services
	if s.QueueManager != nil {
		if err := s.QueueManager.Stop(); err != nil {
			s.Logger.Error("Error stopping queue manager", "error", err)
		}
	}

	if s.WebSocketManager != nil {
		if err := s.WebSocketManager.Stop(); err != nil {
			s.Logger.Error("Error stopping WebSocket manager", "error", err)
		}
	}

	// Disconnect from databases
	if s.DB != nil {
		if err := s.DB.Close(); err != nil {
			s.Logger.Error("Error closing SQL database", "error", err)
		}
	}

	if s.MongoDB != nil {
		if err := s.MongoDB.Client().Disconnect(ctx); err != nil {
			s.Logger.Error("Error disconnecting from MongoDB", "error", err)
		}
	}

	// Shutdown Fiber app
	if s.App != nil {
		return s.App.Shutdown()
	}
	return nil
}
