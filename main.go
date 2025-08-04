package main

import (
	"log"

	"github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
)

// Demo application that shows how to use the go-overlay-fiber library
func main() {
	// Example configuration - in production, these would come from environment variables
	srv := server.NewOverlayServer("go-overlay-fiber", "example-private-key", "localhost").
		ConfigurePort(3001).
		ConfigureNetwork("test").
		ConfigureVerboseLogging(true).
		ConfigureGASPSync(true).
		ConfigureAdminToken("admin-token-123")

	if err := srv.Start(); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
