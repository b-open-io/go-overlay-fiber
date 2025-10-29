package slackthreads

import "time"

// UTXOReference represents a reference to a specific UTXO
type UTXOReference struct {
	Txid        string `bson:"txid" json:"txid"`
	OutputIndex int    `bson:"outputIndex" json:"outputIndex"`
}

// SlackThreadRecord represents a stored SlackThread hash record in the database
type SlackThreadRecord struct {
	Txid        string    `bson:"txid" json:"txid"`
	OutputIndex int       `bson:"outputIndex" json:"outputIndex"`
	ThreadHash  string    `bson:"threadHash" json:"threadHash"`
	CreatedAt   time.Time `bson:"createdAt" json:"createdAt"`
}

// SlackThreadQuery represents query parameters for looking up SlackThread hashes
type SlackThreadQuery struct {
	ThreadHash string `json:"threadHash,omitempty"`
	Txid       string `json:"txid,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Skip       int    `json:"skip,omitempty"`
	StartDate  string `json:"startDate,omitempty"`
	EndDate    string `json:"endDate,omitempty"`
	SortOrder  string `json:"sortOrder,omitempty"`
}
