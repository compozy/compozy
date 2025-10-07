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

func (o *Options) normalizedStrategy() Strategy {
	if o == nil || o.Strategy == "" {
		return StrategyUpsert
	}
	return o.Strategy
}
