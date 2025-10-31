package supplychain

import (
	"time"
)

// SupplyChainRecord represents a MongoDB document stored in the "supplyChainRecords" collection
type SupplyChainRecord struct {
	Txid           string                 `bson:"txid" json:"txid"`
	OutputIndex    int                    `bson:"outputIndex" json:"outputIndex"`
	OffChainValues map[string]interface{} `bson:"offChainValues" json:"offChainValues"`
	SpendingTxid   string                 `bson:"spendingTxid,omitempty" json:"spendingTxid,omitempty"`
	CreatedAt      time.Time              `bson:"createdAt" json:"createdAt"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid" bson:"txid"`
	OutputIndex int    `json:"outputIndex" bson:"outputIndex"`
}

// SupplyChainQuery represents query parameters for SupplyChain lookups
type SupplyChainQuery struct {
	Txid      string `json:"txid,omitempty"`
	ChainID   string `json:"chainId,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Skip      int    `json:"skip,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"` // 'asc' or 'desc'
}
