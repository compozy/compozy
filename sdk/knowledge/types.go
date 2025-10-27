package knowledge

import (
	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
)

// BindingConfig mirrors engine core knowledge bindings for SDK consumers.
type BindingConfig = core.KnowledgeBinding

// VectorDBType mirrors engine knowledge vector database types for SDK usage.
type VectorDBType = engineknowledge.VectorDBType

// VectorDBConfig mirrors engine knowledge vector database configuration.
type VectorDBConfig = engineknowledge.VectorDBConfig

// ChunkStrategy mirrors engine knowledge chunking strategies for SDK usage.
type ChunkStrategy = engineknowledge.ChunkStrategy

const (
	// ChunkStrategyRecursiveTextSplitter splits documents using recursive text splitting.
	ChunkStrategyRecursiveTextSplitter ChunkStrategy = engineknowledge.ChunkStrategyRecursiveTextSplitter
)

// IngestMode mirrors engine knowledge ingestion scheduling modes for SDK usage.
type IngestMode = engineknowledge.IngestMode

const (
	// IngestModeManual runs ingestion only when triggered explicitly.
	IngestModeManual IngestMode = engineknowledge.IngestManual
	// IngestModeOnStart runs ingestion during application startup.
	IngestModeOnStart IngestMode = engineknowledge.IngestOnStart
)
