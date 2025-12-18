package basketmap

import "time"

// BasketMapRecord represents a basket type registration record in MongoDB
type BasketMapRecord struct {
	Txid         string                `bson:"txid" json:"txid"`
	OutputIndex  int                   `bson:"outputIndex" json:"outputIndex"`
	Registration BasketMapRegistration `bson:"registration" json:"registration"`
	CreatedAt    time.Time             `bson:"createdAt" json:"createdAt"`
}

// BasketMapRegistration contains the basket registration details
type BasketMapRegistration struct {
	BasketID         string `bson:"basketID" json:"basketID"`
	Name             string `bson:"name" json:"name"`
	RegistryOperator string `bson:"registryOperator" json:"registryOperator"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
}

// BasketMapQuery represents query parameters for BasketMap lookups
type BasketMapQuery struct {
	BasketID          string   `json:"basketID,omitempty"`
	Name              string   `json:"name,omitempty"`
	RegistryOperators []string `json:"registryOperators,omitempty"`
}
