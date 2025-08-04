package server

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.mongodb.org/mongo-driver/mongo"
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

	// Services
	TopicManagers  map[string]interface{}
	LookupServices map[string]interface{}

	// Fiber app
	App *fiber.App
}

// ErrorResponse represents the standard error response format
type ErrorResponse struct {
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
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
		TopicManagers:    make(map[string]interface{}),
		LookupServices:   make(map[string]interface{}),
	}
}

// Configuration methods with fluent API

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

// ConfigureDatabase sets up SQL database connection
func (s *OverlayServer) ConfigureDatabase(connectionString string) *OverlayServer {
	// TODO: Implement database connection in Phase 2
	s.Logger.Printf("Database configuration set: %s", connectionString)
	return s
}

// ConfigureMongoDB sets up MongoDB connection
func (s *OverlayServer) ConfigureMongoDB(connectionString string) *OverlayServer {
	// TODO: Implement MongoDB connection in Phase 2
	s.Logger.Printf("MongoDB configuration set: %s", connectionString)
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
	return c.JSON(fiber.Map{
		"name":    s.Name,
		"network": s.Network,
		"fqdn":    s.AdvertisableFQDN,
		"status":  "running",
		"message": "Go Overlay Fiber Server - Phase 1 Implementation",
	})
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

// Start starts the server
func (s *OverlayServer) Start() error {
	if err := s.Setup(); err != nil {
		return err
	}

	s.Logger.Printf("Starting %s on port %d (network: %s)", s.Name, s.Port, s.Network)
	s.Logger.Printf("Server FQDN: %s", s.AdvertisableFQDN)
	s.Logger.Printf("GASP Sync: %t", s.EnableGASPSync)
	s.Logger.Printf("Verbose Logging: %t", s.VerboseLogging)

	return s.App.Listen(fmt.Sprintf(":%d", s.Port))
}

// Stop gracefully stops the server
func (s *OverlayServer) Stop() error {
	if s.App != nil {
		return s.App.Shutdown()
	}
	return nil
}