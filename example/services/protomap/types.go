package protomap

import "time"

// ProtoMapRecord represents a protocol registration record in MongoDB
type ProtoMapRecord struct {
	Txid         string               `bson:"txid" json:"txid"`
	OutputIndex  int                  `bson:"outputIndex" json:"outputIndex"`
	Registration ProtoMapRegistration `bson:"registration" json:"registration"`
	CreatedAt    time.Time            `bson:"createdAt" json:"createdAt"`
}

// ProtoMapRegistration contains the protocol registration details
type ProtoMapRegistration struct {
	RegistryOperator string     `bson:"registryOperator" json:"registryOperator"`
	ProtocolID       ProtocolID `bson:"protocolID" json:"protocolID"`
	Name             string     `bson:"name" json:"name"`
}

// ProtocolID represents a wallet protocol identifier
type ProtocolID struct {
	SecurityLevel int    `bson:"securityLevel" json:"securityLevel"`
	Protocol      string `bson:"protocol" json:"protocol"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
}

// ProtoMapQuery represents query parameters for ProtoMap lookups
type ProtoMapQuery struct {
	Name              string       `json:"name,omitempty"`
	RegistryOperators []string     `json:"registryOperators,omitempty"`
	ProtocolID        *ProtocolID  `json:"protocolID,omitempty"`
}
