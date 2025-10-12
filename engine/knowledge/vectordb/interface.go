package vectordb

import "context"

// Provider enumerates supported vector database backends.
type Provider string

const (
	ProviderPGVector Provider = "pgvector"
	ProviderQdrant   Provider = "qdrant"
	ProviderMemory   Provider = "memory"
)

// Record represents a chunk persisted to the vector store.
type Record struct {
	ID        string
	Text      string
	Embedding []float32
	Metadata  map[string]any
}

// SearchOptions controls similarity search execution.
type SearchOptions struct {
	TopK     int
	MinScore float64
	Filters  map[string]string
}

// Match captures a similarity search result.
type Match struct {
	ID       string
	Score    float64
	Text     string
	Metadata map[string]any
}

// Filter specifies delete criteria.
type Filter struct {
	IDs      []string
	Metadata map[string]string
}

// Store exposes the minimal contract for ingestion and retrieval.
type Store interface {
	Upsert(ctx context.Context, records []Record) error
	Search(ctx context.Context, query []float32, opts SearchOptions) ([]Match, error)
	Delete(ctx context.Context, filter Filter) error
	Close(ctx context.Context) error
}

// Config captures normalized connection details for a vector database.
type Config struct {
	ID          string
	Provider    Provider
	DSN         string
	Table       string
	Collection  string
	Namespace   string
	Index       string
	EnsureIndex bool
	Metric      string
	Dimension   int
	Consistency string
	Auth        map[string]string
	Options     map[string]any
}
