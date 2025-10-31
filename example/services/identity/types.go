package identity

import (
	"time"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
)

// IdentityRecord represents an identity certificate record in MongoDB
type IdentityRecord struct {
	Txid                 string                           `bson:"txid" json:"txid"`
	OutputIndex          int                              `bson:"outputIndex" json:"outputIndex"`
	Certificate          *certificates.Certificate        `bson:"certificate" json:"certificate"`
	CreatedAt            time.Time                        `bson:"createdAt" json:"createdAt"`
	SearchableAttributes string                           `bson:"searchableAttributes,omitempty" json:"searchableAttributes,omitempty"`
}

// UTXOReference represents a reference to a UTXO
type UTXOReference struct {
	Txid        string `json:"txid"`
	OutputIndex int    `json:"outputIndex"`
}

// IdentityAttributes represents key-value pairs of identity attributes
type IdentityAttributes map[string]string

// IdentityQuery represents query parameters for Identity lookups
type IdentityQuery struct {
	Attributes       IdentityAttributes `json:"attributes,omitempty"`
	Certifiers       []string           `json:"certifiers,omitempty"`
	IdentityKey      string             `json:"identityKey,omitempty"`
	CertificateTypes []string           `json:"certificateTypes,omitempty"`
	SerialNumber     string             `json:"serialNumber,omitempty"`
}
