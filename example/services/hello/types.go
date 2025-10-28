package hello

import "time"

// UTXOReference represents a reference to a specific UTXO
type UTXOReference struct {
	Txid        string `bson:"txid" json:"txid"`
	OutputIndex int    `bson:"outputIndex" json:"outputIndex"`
}

// HelloWorldRecord represents a stored HelloWorld message record in the database
type HelloWorldRecord struct {
	Txid        string    `bson:"txid" json:"txid"`
	OutputIndex int       `bson:"outputIndex" json:"outputIndex"`
	Message     string    `bson:"message" json:"message"`
	CreatedAt   time.Time `bson:"createdAt" json:"createdAt"`
}

// HelloWorldQuery represents query parameters for looking up HelloWorld messages
type HelloWorldQuery struct {
	Message   string `json:"message,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Skip      int    `json:"skip,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"`
}
