package desktopintegrity

import "time"

// DesktopIntegrityRecord represents a desktop integrity record stored in MongoDB
type DesktopIntegrityRecord struct {
	Txid           string    `bson:"txid"`
	OutputIndex    int       `bson:"outputIndex"`
	FileHash       string    `bson:"fileHash"`
	OffChainValues []byte    `bson:"offChainValues"`
	CreatedAt      time.Time `bson:"createdAt"`
}

// UTXOReference represents a UTXO reference (txid + outputIndex)
type UTXOReference struct {
	Txid        string `json:"txid" bson:"txid"`
	OutputIndex int    `json:"outputIndex" bson:"outputIndex"`
}

// DesktopIntegrityQuery represents a lookup query for desktop integrity records
type DesktopIntegrityQuery struct {
	FileHash  string `json:"fileHash,omitempty"`
	Txid      string `json:"txid,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Skip      int    `json:"skip,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"`
}
