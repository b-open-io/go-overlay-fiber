package fractionalize

import (
	"time"
)

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid" bson:"txid"`
	OutputIndex int    `json:"outputIndex" bson:"outputIndex"`
}

// FractionalizeRecord represents a MongoDB document stored in the "fractionalizeRecords" collection
type FractionalizeRecord struct {
	Txid         string    `bson:"txid" json:"txid"`
	OutputIndex  int       `bson:"outputIndex" json:"outputIndex"`
	SpendingTxid string    `bson:"spendingTxid,omitempty" json:"spendingTxid,omitempty"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

// FractionalizeQuery represents query parameters for Fractionalize lookups
type FractionalizeQuery struct {
	Txid      string `json:"txid,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Skip      int    `json:"skip,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"` // 'asc' or 'desc'
}
