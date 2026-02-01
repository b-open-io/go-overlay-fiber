package certmap

import (
	"time"
)

// CertMapRegistration represents certificate type registration data held inside the PushDrop token
type CertMapRegistration struct {
	Type             string                 `json:"type" bson:"type"`
	Name             string                 `json:"name" bson:"name"`
	IconURL          string                 `json:"iconURL" bson:"iconURL"`
	Description      string                 `json:"description" bson:"description"`
	DocumentationURL string                 `json:"documentationURL" bson:"documentationURL"`
	CertFields       map[string]interface{} `json:"certFields" bson:"certFields"`
	RegistryOperator string                 `json:"registryOperator" bson:"registryOperator"`
}

// CertMapRecord represents a MongoDB document stored in the "certmapRecords" collection
type CertMapRecord struct {
	Txid         string               `bson:"txid" json:"txid"`
	OutputIndex  int                  `bson:"outputIndex" json:"outputIndex"`
	Registration *CertMapRegistration `bson:"registration" json:"registration"`
	CreatedAt    time.Time            `bson:"createdAt" json:"createdAt"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
}

// CertMapLookupQuery represents query parameters for CertMap lookups
type CertMapLookupQuery struct {
	Type              string   `json:"type,omitempty"`
	Name              string   `json:"name,omitempty"`
	RegistryOperators []string `json:"registryOperators,omitempty"`
}
