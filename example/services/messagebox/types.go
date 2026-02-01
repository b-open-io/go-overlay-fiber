package messagebox

import "time"

// MessageBoxAdvertisement represents a stored MessageBox advertisement record in the database
type MessageBoxAdvertisement struct {
	IdentityKey string    `bson:"identityKey" json:"identityKey"`
	Host        string    `bson:"host" json:"host"`
	Txid        string    `bson:"txid" json:"txid"`
	OutputIndex int       `bson:"outputIndex" json:"outputIndex"`
	CreatedAt   time.Time `bson:"createdAt" json:"createdAt"`
}

// UTXOReference represents a reference to a specific UTXO
type UTXOReference struct {
	Txid        string `bson:"txid" json:"txid"`
	OutputIndex int    `bson:"outputIndex" json:"outputIndex"`
}

// MessageBoxQuery represents query parameters for looking up MessageBox advertisements
type MessageBoxQuery struct {
	IdentityKey string `json:"identityKey"`    // Required: hex-encoded public key
	Host        string `json:"host,omitempty"` // Optional: filter by specific host
}
