package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Provider enumerates supported vector database backends.
type Provider string

const (
	ProviderPGVector Provider = "pgvector"
	ProviderQdrant   Provider = "qdrant"
	ProviderRedis    Provider = "redis"
	// ProviderFilesystem persists embeddings to a local filesystem-backed store.
	ProviderFilesystem Provider = "filesystem"
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
	Path        string
	Table       string
	Collection  string
	Namespace   string
	Index       string
	EnsureIndex bool
	Metric      string
	Dimension   int
	Consistency string
	Auth        map[string]string
	// Options carries provider-specific overrides. For postgres-backed stores,
	// values supplied here are considered legacy and are ignored when PGVector is set.
	Options map[string]any
	MaxTopK int
	// PGVector configures postgres vector stores and takes precedence over matching entries in Options.
	PGVector *PGVectorOptions
}

// PGVectorOptions configures postgres vector stores.
type PGVectorOptions struct {
	Index  PGVectorIndexOptions
	Pool   PGVectorPoolOptions
	Search PGVectorSearchOptions
}

// PGVectorIndexType represents supported index types for pgvector.
type PGVectorIndexType string

const (
	// PGVectorIndexHNSW uses Hierarchical Navigable Small World index.
	PGVectorIndexHNSW PGVectorIndexType = "hnsw"
	// PGVectorIndexIVFFlat uses Inverted File with Flat Quantizer index.
	PGVectorIndexIVFFlat PGVectorIndexType = "ivfflat"
)

// PGVectorIndexOptions tunes index creation and runtime behavior.
type PGVectorIndexOptions struct {
	Type           PGVectorIndexType
	Lists          int
	Probes         int
	M              int
	EFConstruction int
	EFSearch       int
}

// IsValidIndexType checks if the Type field contains a known index type value.
func (opts PGVectorIndexOptions) IsValidIndexType() bool {
	if opts.Type == "" {
		return true // empty is valid, will use default
	}
	return opts.Type == PGVectorIndexHNSW || opts.Type == PGVectorIndexIVFFlat
}

// PGVectorPoolOptions customizes pgxpool behavior.
type PGVectorPoolOptions struct {
	MinConns          int32
	MaxConns          int32
	MaxConnLifetime   Duration
	MaxConnIdleTime   Duration
	HealthCheckPeriod Duration
}

// PGVectorSearchOptions adjusts search-related GUCs.
type PGVectorSearchOptions struct {
	Probes   int
	EFSearch int
}

// Duration wraps time.Duration to expose human-readable JSON/text encodings.
type Duration time.Duration

// MarshalText encodes the duration using the canonical string format (e.g., "30s").
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// UnmarshalText decodes canonical duration strings such as "30s" or "500ms".
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalJSON encodes the duration as a human-readable JSON string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON decodes JSON strings into Duration values.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}
	*d = Duration(parsed)
	return nil
}

// ToDuration converts Duration to the standard library representation.
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// String returns the canonical duration representation.
func (d Duration) String() string {
	return time.Duration(d).String()
}
