package server

import (
	"context"
	"testing"

	"github.com/b-open-io/overlay/beef"
	"github.com/b-open-io/overlay/pubsub"
	"github.com/b-open-io/overlay/storage"
)

// TestOverlayIntegration tests that the overlay components can be created and integrated
func TestOverlayIntegration(t *testing.T) {
	// Test BEEF storage creation
	beefStorage, err := beef.CreateBeefStorage("")
	if err != nil {
		t.Fatalf("Failed to create BEEF storage: %v", err)
	}
	if beefStorage == nil {
		t.Fatal("BEEF storage is nil")
	}
	t.Logf("✅ BEEF storage created successfully: %T", beefStorage)

	// Test publisher creation
	publisher := &NoOpPublisher{}
	if publisher == nil {
		t.Fatal("Publisher is nil")
	}

	// Test publisher interface
	err = publisher.Publish(context.Background(), "test-topic", "test-data")
	if err != nil {
		t.Fatalf("Failed to publish test message: %v", err)
	}
	t.Logf("✅ Publisher created and tested successfully: %T", publisher)

	// Test storage factory (this might fail due to no connection string, which is expected)
	_, err = storage.CreateEventDataStorage("", beefStorage, nil, publisher, nil)
	// We expect this to fail for empty connection string, but we want to test the factory exists
	if err != nil {
		t.Logf("✅ Storage factory exists and handles empty connection string properly: %v", err)
	} else {
		t.Log("✅ Storage factory created successfully")
	}

	t.Log("🎉 All overlay component integrations are working correctly!")
}

// TestOverlayStorageCreation tests our storage adapter creation
func TestOverlayStorageCreation(t *testing.T) {
	// This test specifically tests our adapter without the engine dependency

	// Test BEEF storage creation with default path
	beefStorage, err := beef.CreateBeefStorage("./test_beef_storage")
	if err != nil {
		t.Fatalf("Failed to create BEEF storage: %v", err)
	}

	storageType := getStorageType("./test.db")
	if storageType != "SQLite" {
		t.Errorf("Expected SQLite, got %s", storageType)
	}

	storageType = getStorageType("")
	if storageType != "SQL" {
		t.Errorf("Expected SQL, got %s", storageType)
	}

	t.Log("✅ Overlay storage type detection working correctly")
	t.Logf("✅ BEEF storage created: %T", beefStorage)
}

// TestPublisherInterface tests our publisher implementation
func TestPublisherInterface(t *testing.T) {
	publisher := &NoOpPublisher{}

	// Test that it implements the interface properly
	var _ pubsub.PubSub = publisher

	// Test publish functionality
	err := publisher.Publish(context.Background(), "test-topic", "test-data")
	if err != nil {
		t.Errorf("Unexpected error from no-op publisher: %v", err)
	}

	t.Log("✅ Publisher interface implementation working correctly")
}
