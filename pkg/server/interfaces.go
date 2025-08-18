package server

import (
	"database/sql"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"

	"github.com/bsv-blockchain/go-overlay-services/pkg/core/advertiser"
	"github.com/bsv-blockchain/go-overlay-services/pkg/core/engine"
	"github.com/bsv-blockchain/go-sdk/overlay/topic"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"go.mongodb.org/mongo-driver/mongo"
)

// UIConfig represents web UI configuration
type UIConfig struct {
	Host                     string
	FaviconUrl               string
	BackgroundColor          string
	PrimaryColor             string
	SecondaryColor           string
	FontFamily               string
	HeadingFontFamily        string
	AdditionalStyles         string
	SectionBackgroundColor   string
	PrimaryTextColor         string
	LinkColor                string
	HoverColor               string
	BorderColor              string
	SecondaryBackgroundColor string
	SecondaryTextColor       string
	DefaultContent           string
}

// EngineConfig represents advanced engine configuration
type EngineConfig struct {
	ChainTracker                      chaintracker.ChainTracker
	ShipTrackers                      []string
	SlapTrackers                      []string
	Broadcaster                       transaction.Broadcaster
	Advertiser                        advertiser.Advertiser
	SyncConfiguration                 map[string]engine.SyncConfiguration // string[] | "SHIP" | false
	LogTime                           *bool
	LogPrefix                         string
	ThrowOnBroadcastFailure           *bool
	OverlayBroadcastFacilitator       topic.Facilitator
	SuppressDefaultSyncAdvertisements *bool
}

// ErrorResponse represents the standard error response format
type ErrorResponse struct {
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// Script represents a Bitcoin script
type Script struct {
	Data []byte `json:"data"`
}

// Migration represents a database migration
type Migration struct {
	Name string
	Up   func(db *sql.DB) error
	Down func(db *sql.DB) error // optional
}

// LookupServiceFactory represents a factory function for creating lookup services with SQL database
type LookupServiceFactory func(db *sql.DB) (engine.LookupService, []Migration, error)

// MongoLookupServiceFactory represents a factory function for creating lookup services with MongoDB
type MongoLookupServiceFactory func(db *mongo.Database) (engine.LookupService, error)
