package did

import "time"

// DIDRecord represents a DID record stored in MongoDB
type DIDRecord struct {
	Txid         string    `bson:"txid" json:"txid"`
	OutputIndex  int       `bson:"outputIndex" json:"outputIndex"`
	SerialNumber string    `bson:"serialNumber" json:"serialNumber"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

// DIDQuery represents query parameters for looking up DID records
type DIDQuery struct {
	SerialNumber string `json:"serialNumber,omitempty"`
	Outpoint     string `json:"outpoint,omitempty"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
}
