package marketplace

const (
	CompozyCatalogSource = "compozy-catalog"
	CompozyCatalogRef    = "catalog:compozy"
)

// Origin is acquisition identity; source display names do not participate in equality.
type Origin struct {
	SourceRef string `json:"source_ref"`
	EntryID   string `json:"entry_id"`
}
