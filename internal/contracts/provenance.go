package contracts

// Provenance identifies the trusted source of an admitted result.
type Provenance struct {
	ProducerKind string `json:"producer_kind"`
	ProducerID   string `json:"producer_id"`
	Contract     string `json:"contract_digest,omitempty"`
}
