package main

import (
	"log/slog"
	"os"

	"github.com/bsv-blockchain/go-overlay-fiber/example/services/any"
	"github.com/bsv-blockchain/go-overlay-fiber/example/services/did"
	"github.com/bsv-blockchain/go-overlay-fiber/example/services/hello"
	"github.com/bsv-blockchain/go-overlay-fiber/example/services/messagebox"
	"github.com/bsv-blockchain/go-overlay-fiber/example/services/protomap"
	"github.com/bsv-blockchain/go-overlay-fiber/example/services/slackthreads"
	"github.com/bsv-blockchain/go-overlay-fiber/example/services/uhrp"
	"github.com/bsv-blockchain/go-overlay-fiber/example/services/ump"
	"github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
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

	slog.SetLogLoggerLevel(slog.LevelInfo)

	// Decide what port you want the server to listen on.
	overlayServer.ConfigurePort(8080)

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

	// Here, you will configure the overlay topic managers and lookup services you want.
	// - Topic managers decide what outputs can go in your overlay
	// - Lookup services help people find things in your overlay

	// Any
	overlayServer.ConfigureTopicManager("tm_anytx", any.NewAnyTopicManager())
	overlayServer.ConfigureLookupService("ls_anytx", any.NewAnyLookupService(overlayServer.MongoDB))

	// DID
	overlayServer.ConfigureTopicManager("tm_did", did.NewDIDTopicManager())
	overlayServer.ConfigureLookupService("ls_did", did.NewDIDLookupService(overlayServer.MongoDB))

	// HelloWorld
	overlayServer.ConfigureTopicManager("tm_helloworld", hello.NewHelloWorldTopicManager())
	overlayServer.ConfigureLookupService("ls_helloworld", hello.NewHelloWorldLookupService(overlayServer.MongoDB))

	// SlackThreads
	overlayServer.ConfigureTopicManager("tm_slackthread", slackthreads.NewSlackThreadsTopicManager())
	overlayServer.ConfigureLookupService("ls_slackthread", slackthreads.NewSlackThreadLookupService(overlayServer.MongoDB))

	// MessageBox
	overlayServer.ConfigureTopicManager("tm_messagebox", messagebox.NewMessageBoxTopicManager())
	overlayServer.ConfigureLookupService("ls_messagebox", messagebox.NewMessageBoxLookupService(overlayServer.MongoDB))

	// UHRP
	overlayServer.ConfigureTopicManager("tm_uhrp", uhrp.NewUHRPTopicManager())
	overlayServer.ConfigureLookupService("ls_uhrp", uhrp.NewUHRPLookupService(overlayServer.MongoDB))

	// UMP
	overlayServer.ConfigureTopicManager("tm_users", ump.NewUMPTopicManager())
	overlayServer.ConfigureLookupService("ls_users", ump.NewUMPLookupService(overlayServer.MongoDB))

	// ProtoMap
	overlayServer.ConfigureTopicManager("tm_protomap", protomap.NewProtoMapTopicManager())
	overlayServer.ConfigureLookupService("ls_protomap", protomap.NewProtoMapLookupService(overlayServer.MongoDB))

	// Enable GASP sync for testing
	overlayServer.ConfigureGASPSync(true)

	// Configure the engine with auto-configuration for SHIP/SLAP services
	overlayServer.ConfigureEngine(true)

	// Lastly, start the server!
	if err := overlayServer.Start(); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

// Happy hacking :)
