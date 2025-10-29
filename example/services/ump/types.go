package ump

import "time"

// UMPRecord represents a User Management Protocol token record in MongoDB
type UMPRecord struct {
	Txid             string    `bson:"txid" json:"txid"`
	OutputIndex      int       `bson:"outputIndex" json:"outputIndex"`
	PresentationHash string    `bson:"presentationHash" json:"presentationHash"`
	RecoveryHash     string    `bson:"recoveryHash" json:"recoveryHash"`
	CreatedAt        time.Time `bson:"createdAt" json:"createdAt"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
}

// UMPQuery represents query parameters for UMP lookups
type UMPQuery struct {
	PresentationHash string `json:"presentationHash,omitempty"`
	RecoveryHash     string `json:"recoveryHash,omitempty"`
	Outpoint         string `json:"outpoint,omitempty"`
}
