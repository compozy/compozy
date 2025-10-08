package uc

import (
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/configutil"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
)

func ToEmbedderAdapterConfig(cfg *knowledge.EmbedderConfig) (*embedder.Config, error) {
	return configutil.ToEmbedderAdapterConfig(cfg)
}

func ToVectorStoreConfig(project string, cfg *knowledge.VectorDBConfig) (*vectordb.Config, error) {
	return configutil.ToVectorStoreConfig(project, cfg)
}
