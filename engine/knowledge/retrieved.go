package knowledge

// RetrievedContext represents a chunk returned by the retrieval service.
type RetrievedContext struct {
	BindingID     string
	Content       string
	Score         float64
	TokenEstimate int
	Metadata      map[string]any
}

// RetrievalStatus captures the router verdict for a knowledge lookup.
type RetrievalStatus string

const (
	// RetrievalStatusHit indicates that retrieval returned enough contexts for prompt augmentation.
	RetrievalStatusHit RetrievalStatus = "hit"
	// RetrievalStatusFallback signals that retrieval failed and a fallback notice should be surfaced.
	RetrievalStatusFallback RetrievalStatus = "fallback"
	// RetrievalStatusEscalated denotes that retrieval failed and the router escalated to tool usage.
	RetrievalStatusEscalated RetrievalStatus = "escalated"
)
