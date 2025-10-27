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
