package embedder

import "context"

// Provider enumerates supported embedder providers.
type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderVertex Provider = "vertex"
	ProviderLocal  Provider = "local"
)

// Embedder exposes the minimal contract required by the knowledge domain.
type Embedder interface {
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

// Config captures the normalized configuration for a concrete embedder instance.
type Config struct {
	ID            string
	Provider      Provider
	Model         string
	APIKey        string
	Dimension     int
	BatchSize     int
	StripNewLines bool
	Options       map[string]any
}
