package any

import "time"

// UTXOReference represents a reference to a specific UTXO
type UTXOReference struct {
	Txid        string `bson:"txid" json:"txid"`
	OutputIndex int    `bson:"outputIndex" json:"outputIndex"`
}

// AnyRecord represents a stored record in the database
type AnyRecord struct {
	Txid         string    `bson:"txid" json:"txid"`
	OutputIndex  int       `bson:"outputIndex" json:"outputIndex"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
	SpendingTxid *string   `bson:"spendingTxid,omitempty" json:"spendingTxid,omitempty"`
}

// AnyQuery represents query parameters for looking up records
type AnyQuery struct {
	Txid      string `json:"txid,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Skip      int    `json:"skip,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"`
}