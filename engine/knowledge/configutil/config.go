package configutil

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
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

func ToVectorStoreConfig(ctx context.Context, project string, cfg *knowledge.VectorDBConfig) (*vectordb.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("vector_db config is required")
	}
	provider := vectordb.Provider(strings.TrimSpace(string(cfg.Type)))
	switch provider {
	case vectordb.ProviderPGVector, vectordb.ProviderQdrant, vectordb.ProviderMemory, vectordb.ProviderFilesystem:
	default:
		return nil, fmt.Errorf("project %s vector_db %q: unsupported type %q", project, cfg.ID, cfg.Type)
	}
	dsn := strings.TrimSpace(cfg.Config.DSN)
	if dsn == "" && provider == vectordb.ProviderPGVector {
		dsn = resolvePGVectorDSN(ctx, cfg.ID)
	}
	pathValue := strings.TrimSpace(cfg.Config.Path)
	if provider == vectordb.ProviderFilesystem && pathValue == "" {
		pathValue = defaultFilesystemPath(ctx, cfg.ID)
	}
	storeCfg := &vectordb.Config{
		ID:          cfg.ID,
		Provider:    provider,
		DSN:         dsn,
		Path:        pathValue,
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

func resolvePGVectorDSN(ctx context.Context, vectorDBID string) string {
	log := logger.FromContext(ctx)
	log.Debug("pgvector: DSN not provided, using global postgres database config", "vector_db_id", vectorDBID)
	globalCfg := config.FromContext(ctx)
	if globalCfg == nil {
		log.Warn("pgvector: failed to retrieve global config for DSN fallback", "vector_db_id", vectorDBID)
		return ""
	}
	return buildPostgresDSN(&globalCfg.Database)
}

func buildPostgresDSN(cfg *config.DatabaseConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.ConnString != "" {
		return cfg.ConnString
	}
	host := defaultIfEmpty(cfg.Host, "localhost")
	port := defaultIfEmpty(cfg.Port, "5432")
	user := defaultIfEmpty(cfg.User, "postgres")
	password := defaultIfEmpty(cfg.Password, "")
	dbname := defaultIfEmpty(cfg.DBName, "postgres")
	sslmode := defaultIfEmpty(cfg.SSLMode, "disable")
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)
}

func defaultIfEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func defaultFilesystemPath(ctx context.Context, id string) string {
	base := strings.TrimSpace(id)
	if base == "" {
		base = "vector_db"
	}
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return unicode.ToLower(r)
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, base)
	if safe == "" || safe == "_" {
		safe = "vector_db"
	}
	root := ""
	if cfg := config.FromContext(ctx); cfg != nil {
		if cwd := strings.TrimSpace(cfg.CLI.CWD); cwd != "" {
			root = cwd
		}
	}
	storeDir := core.GetStoreDir(root)
	return filepath.Join(storeDir, "cache", safe+".store")
}
