package main

import (
	"log"

	"github.com/bsv-blockchain/go-overlay-fiber/pkg/server"
)

// Demo UI application that shows the web interface
// This matches the simplicity of demo-ui.ts from overlay-express
func main() {
	// Minimal configuration - just what's needed to show the UI
	srv := server.NewOverlayServer("demo-ui", "demo-key", "localhost").
		ConfigurePort(8081).
		ConfigureWebUI(server.UIConfig{
			Host: "http://localhost:8080",
		})

	log.Println("Overlay Fiber demo UI started on http://localhost:8081")

	if err := srv.Start(); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
