package walletconfig

import (
	"time"
)

// WalletConfigRegistration represents wallet configuration data held inside the PushDrop token
type WalletConfigRegistration struct {
	ConfigID         string `json:"configID" bson:"configID"`
	Name             string `json:"name" bson:"name"`
	Icon             string `json:"icon" bson:"icon"`
	WAB              string `json:"wab" bson:"wab"`               // Wallet Authentication Backend URL
	Storage          string `json:"storage" bson:"storage"`       // Wallet storage URL
	Messagebox       string `json:"messagebox" bson:"messagebox"` // Messagebox URL
	Legal            string `json:"legal" bson:"legal"`           // Legal terms URL
	RegistryOperator string `json:"registryOperator" bson:"registryOperator"`
}

// WalletConfigRecord represents a MongoDB document stored in the "walletConfigRecords" collection
type WalletConfigRecord struct {
	Txid         string                    `bson:"txid" json:"txid"`
	OutputIndex  int                       `bson:"outputIndex" json:"outputIndex"`
	Registration *WalletConfigRegistration `bson:"registration" json:"registration"`
	CreatedAt    time.Time                 `bson:"createdAt" json:"createdAt"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
}

// WalletConfigQuery represents query parameters for WalletConfig lookups
type WalletConfigQuery struct {
	ConfigID          string   `json:"configID,omitempty"`
	Name              string   `json:"name,omitempty"`
	WAB               string   `json:"wab,omitempty"`
	Storage           string   `json:"storage,omitempty"`
	Messagebox        string   `json:"messagebox,omitempty"`
	RegistryOperators []string `json:"registryOperators,omitempty"`
}
