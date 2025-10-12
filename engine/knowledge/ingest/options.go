package ingest

import "github.com/compozy/compozy/engine/core"

// Strategy defines how ingestion should write records into the vector store.
type Strategy string

const (
	StrategyUpsert  Strategy = "upsert"
	StrategyReplace Strategy = "replace"
)

// Options controls ingestion execution details provided by callers.
type Options struct {
	CWD      *core.PathCWD
	Strategy Strategy
}

// NormalizedStrategy returns the effective ingestion strategy, defaulting to
// StrategyUpsert when callers omit a specific value.
func (o *Options) NormalizedStrategy() Strategy {
	if o == nil || o.Strategy == "" {
		return StrategyUpsert
	}
	return o.Strategy
}
