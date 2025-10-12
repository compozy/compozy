package configutil

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
)

func ToEmbedderAdapterConfig(cfg *knowledge.EmbedderConfig) (*embedder.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedder config is required")
	}
	provider := embedder.Provider(strings.TrimSpace(cfg.Provider))
	switch provider {
	case embedder.ProviderOpenAI, embedder.ProviderVertex, embedder.ProviderLocal:
	default:
		return nil, fmt.Errorf("embedder %q: unsupported provider %q", cfg.ID, cfg.Provider)
	}
	strip := true
	if cfg.Config.StripNewLines != nil {
		strip = *cfg.Config.StripNewLines
	}
	adapter := &embedder.Config{
		ID:            cfg.ID,
		Provider:      provider,
		Model:         strings.TrimSpace(cfg.Model),
		APIKey:        strings.TrimSpace(cfg.APIKey),
		Dimension:     cfg.Config.Dimension,
		BatchSize:     cfg.Config.BatchSize,
		StripNewLines: strip,
	}
	if len(cfg.Config.Retry) > 0 {
		adapter.Options = core.CopyMap(cfg.Config.Retry)
	}
	return adapter, nil
}

func ToVectorStoreConfig(project string, cfg *knowledge.VectorDBConfig) (*vectordb.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("vector_db config is required")
	}
	provider := vectordb.Provider(strings.TrimSpace(string(cfg.Type)))
	switch provider {
	case vectordb.ProviderPGVector, vectordb.ProviderQdrant, vectordb.ProviderMemory:
	default:
		return nil, fmt.Errorf("project %s vector_db %q: unsupported type %q", project, cfg.ID, cfg.Type)
	}
	storeCfg := &vectordb.Config{
		ID:          cfg.ID,
		Provider:    provider,
		DSN:         strings.TrimSpace(cfg.Config.DSN),
		Table:       strings.TrimSpace(cfg.Config.Table),
		Collection:  strings.TrimSpace(cfg.Config.Collection),
		Index:       strings.TrimSpace(cfg.Config.Index),
		EnsureIndex: cfg.Config.EnsureIndex,
		Metric:      strings.TrimSpace(cfg.Config.Metric),
		Dimension:   cfg.Config.Dimension,
		Consistency: strings.TrimSpace(cfg.Config.Consistency),
	}
	if len(cfg.Config.Auth) > 0 {
		storeCfg.Auth = core.CopyMap(cfg.Config.Auth)
	}
	return storeCfg, nil
}
