package uhrp

// UHRPRecord represents a stored UHRP file hosting advertisement in the database
type UHRPRecord struct {
	Txid                string `bson:"txid" json:"txid"`
	OutputIndex         int    `bson:"outputIndex" json:"outputIndex"`
	UHRPUrl             string `bson:"uhrpUrl" json:"uhrpUrl"`
	HostIdentityKey     string `bson:"hostIdentityKey" json:"hostIdentityKey"`
	HostedFileLocation  string `bson:"hostedFileLocation" json:"hostedFileLocation"`
	ExpiryTime          uint64 `bson:"expiryTime" json:"expiryTime"`
	FileSize            uint64 `bson:"fileSize" json:"fileSize"`
}

// UTXOReference represents a reference to a specific UTXO
type UTXOReference struct {
	Txid        string `bson:"txid" json:"txid"`
	OutputIndex int    `bson:"outputIndex" json:"outputIndex"`
}

// UHRPQuery represents query parameters for looking up UHRP advertisements
type UHRPQuery struct {
	Outpoint         string `json:"outpoint,omitempty"`         // "txid.outputIndex" format
	UHRPUrl          string `json:"uhrpUrl,omitempty"`          // Generated from file hash
	ExpiryTime       uint64 `json:"expiryTime,omitempty"`       // Unix timestamp
	HostIdentityKey  string `json:"hostIdentityKey,omitempty"`  // hex-encoded public key
	FileSize         uint64 `json:"fileSize,omitempty"`         // bytes
}
