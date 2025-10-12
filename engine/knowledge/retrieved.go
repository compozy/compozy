package knowledge

// RetrievedContext represents a chunk returned by the retrieval service.
type RetrievedContext struct {
	BindingID     string
	Content       string
	Score         float64
	TokenEstimate int
	Metadata      map[string]any
}
