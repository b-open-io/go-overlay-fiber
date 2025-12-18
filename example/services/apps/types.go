package apps

import (
	"time"
)

// PublishedAppMetadata represents on-chain App metadata held inside the PushDrop token's JSON payload
type PublishedAppMetadata struct {
	Version        string   `json:"version" bson:"version"`
	Name           string   `json:"name" bson:"name"`
	Description    string   `json:"description" bson:"description"`
	Icon           string   `json:"icon" bson:"icon"` // URL or UHRP
	HTTPURL        string   `json:"httpURL,omitempty" bson:"httpURL,omitempty"`
	UHRPURL        string   `json:"uhrpURL,omitempty" bson:"uhrpURL,omitempty"`
	Domain         string   `json:"domain" bson:"domain"`
	Publisher      string   `json:"publisher" bson:"publisher"` // identity key
	ShortName      string   `json:"short_name,omitempty" bson:"short_name,omitempty"`
	Category       string   `json:"category,omitempty" bson:"category,omitempty"`
	Tags           []string `json:"tags,omitempty" bson:"tags,omitempty"`
	ReleaseDate    string   `json:"release_date" bson:"release_date"` // ISO-8601
	Changelog      string   `json:"changelog,omitempty" bson:"changelog,omitempty"`
	BannerImageURL string   `json:"banner_image_url,omitempty" bson:"banner_image_url,omitempty"`
	ScreenshotURLs []string `json:"screenshot_urls,omitempty" bson:"screenshot_urls,omitempty"`
}

// AppCatalogRecord represents a MongoDB document stored in the "appsCatalogRecords" collection
type AppCatalogRecord struct {
	Txid        string                `bson:"txid" json:"txid"`
	OutputIndex int                   `bson:"outputIndex" json:"outputIndex"`
	Metadata    *PublishedAppMetadata `bson:"metadata" json:"metadata"`
	CreatedAt   time.Time             `bson:"createdAt" json:"createdAt"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
}

// AppCatalogQuery represents query parameters for App lookups
type AppCatalogQuery struct {
	// Filter parameters
	Domain    string   `json:"domain,omitempty"`
	Publisher string   `json:"publisher,omitempty"`
	Name      string   `json:"name,omitempty"`
	Outpoint  string   `json:"outpoint,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Category  string   `json:"category,omitempty"`

	// Pagination parameters
	Limit int `json:"limit,omitempty"` // Maximum number of results to return (default: 50)
	Skip  int `json:"skip,omitempty"`  // Number of results to skip (default: 0)

	// Sorting parameters
	SortOrder string `json:"sortOrder,omitempty"` // Sort direction ('asc' or 'desc', default: 'desc' - newest first)
}
