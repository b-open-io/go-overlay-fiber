package server

import (
	"os"
	"strings"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
)

// OverlayStorageAdapter wraps SQLStorage with additional overlay capabilities
type OverlayStorageAdapter struct {
	*SQLStorage
	beefStoragePath string
	// Future: add Redis publisher for real-time events
}

// CreateOverlayStorage creates storage based on connection strings
func CreateOverlayStorage(eventStorageURL, beefStorageURL string) (engine.Storage, error) {
	// For now, just use SQLStorage as the base
	// Later phases will add Redis, MongoDB support

	adapter := &OverlayStorageAdapter{
		SQLStorage: &SQLStorage{},
	}

	// Set up BEEF storage path
	if beefStorageURL == "" {
		beefStorageURL = "./beef_storage"
	}
	adapter.beefStoragePath = beefStorageURL

	// Create BEEF storage directory if it doesn't exist
	if !strings.HasPrefix(beefStorageURL, "redis://") && !strings.HasPrefix(beefStorageURL, "mongodb://") {
		if err := os.MkdirAll(beefStorageURL, 0755); err != nil {
			return nil, err
		}
	}

	return adapter, nil
}

// GetBeefStoragePath returns the configured BEEF storage path
func (o *OverlayStorageAdapter) GetBeefStoragePath() string {
	return o.beefStoragePath
}
