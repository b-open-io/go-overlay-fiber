package main

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
)

// Hi there! Let's configure Go Overlay Fiber!
func main() {

	// We'll make a new server for our overlay node.
	overlayServer := server.NewOverlayServer(
		// Name your overlay node with a one-word lowercase string
		"testnode",

		// Provide the private key that gives your node its identity
		os.Getenv("SERVER_PRIVATE_KEY"),

		// Provide the HTTPS URL where your node is available on the internet
		os.Getenv("HOSTING_URL"),
	)
	var port int
	if envPort := os.Getenv("PORT"); envPort != "" {
		port, _ = strconv.Atoi(envPort)
	} else {
		port = 8081
	}

	slog.SetLogLoggerLevel(slog.LevelInfo)

	overlayServer.ConfigurePort(port)

	// Connect to MySQL database
	if mysqlURL := os.Getenv("MYSQL_URL"); mysqlURL != "" {
		overlayServer.ConfigureDatabase("mysql", mysqlURL)
	} else {
		// Connect to SQLite database with a simple file path
		overlayServer.ConfigureDatabase("sqlite3", "./data.db")
	}

	// Also, be sure to connect to MongoDB
	if mongoURL := os.Getenv("MONGO_URL"); mongoURL != "" {
		overlayServer.ConfigureMongoDB(mongoURL)
	}

	var engineConfig server.EngineConfig
	if shipTracker := os.Getenv("SHIP_TRACKER"); shipTracker != "" {
		engineConfig.ShipTrackers = []string{shipTracker}
	}
	if slapTracker := os.Getenv("SLAP_TRACKER"); slapTracker != "" {
		engineConfig.SlapTrackers = []string{slapTracker}
	}
	engineConfig.SyncConfiguration = map[string]engine.SyncConfiguration{
		"tm_ship": {Type: engine.SyncConfigurationPeers, Peers: []string{"http://localhost:8080"}},
		"tm_slap": {Type: engine.SyncConfigurationPeers, Peers: []string{"http://localhost:8080"}},
	}
	overlayServer.ConfigureEngineParams(engineConfig)

	// Here, you will configure the overlay topic managers and lookup services you want.
	// - Topic managers decide what outputs can go in your overlay
	// - Lookup services help people find things in your overlay
	// - Make use of functions like `ConfigureTopicManager` and `ConfigureLookupServiceWithMongo`
	// ADD YOUR OVERLAY SERVICES HERE

	// Enable GASP sync for testing
	overlayServer.ConfigureGASPSync(true)

	// Configure the engine with auto-configuration for SHIP/SLAP services
	overlayServer.ConfigureEngine(true)

	slog.Info("Starting server", "port", port)

	// Lastly, start the server!
	if err := overlayServer.Start(); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

// Happy hacking :)
