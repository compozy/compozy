package configutil

import (
	"context"
	"fmt"
	"net/url"
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

const (
	defaultPGHost = "localhost"
	defaultPGPort = "5432"
	defaultPGUser = "postgres"
	defaultPGDB   = "postgres"
	defaultPGSSL  = "disable"

	defaultRedisHost = "localhost"
	defaultRedisPort = "6379"
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
		adapter.Options = core.CloneMap(cfg.Config.Retry)
	}
	return adapter, nil
}

func ToVectorStoreConfig(ctx context.Context, project string, cfg *knowledge.VectorDBConfig) (*vectordb.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("vector_db config is required")
	}
	provider := vectordb.Provider(strings.TrimSpace(string(cfg.Type)))
	switch provider {
	case vectordb.ProviderPGVector, vectordb.ProviderQdrant, vectordb.ProviderRedis, vectordb.ProviderFilesystem:
	default:
		return nil, fmt.Errorf("project %s vector_db %q: unsupported type %q", project, cfg.ID, cfg.Type)
	}
	dsn := strings.TrimSpace(cfg.Config.DSN)
	if dsn == "" && provider == vectordb.ProviderPGVector {
		dsn = resolvePGVectorDSN(ctx, cfg.ID)
	}
	if dsn == "" && provider == vectordb.ProviderRedis {
		dsn = resolveRedisDSN(ctx, cfg.ID)
	}
	pathValue := strings.TrimSpace(cfg.Config.Path)
	if provider == vectordb.ProviderFilesystem && pathValue == "" {
		pathValue = defaultFilesystemPath(ctx, cfg.ID)
	}
	if cfg.Config.Dimension <= 0 {
		return nil, fmt.Errorf(
			"project %s vector_db %q: config.dimension must be greater than zero",
			project,
			cfg.ID,
		)
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
		MaxTopK:     cfg.Config.MaxTopK,
	}
	if len(cfg.Config.Auth) > 0 {
		storeCfg.Auth = core.CloneMap(cfg.Config.Auth)
	}
	if cfg.Config.PGVector != nil {
		options := &vectordb.PGVectorOptions{}
		if idx := cfg.Config.PGVector.Index; idx != nil {
			options.Index = vectordb.PGVectorIndexOptions{
				Type:           strings.TrimSpace(strings.ToLower(idx.Type)),
				Lists:          idx.Lists,
				Probes:         idx.Probes,
				M:              idx.M,
				EFConstruction: idx.EFConstruction,
				EFSearch:       idx.EFSearch,
			}
		}
		if pool := cfg.Config.PGVector.Pool; pool != nil {
			options.Pool = vectordb.PGVectorPoolOptions{
				MinConns:          pool.MinConns,
				MaxConns:          pool.MaxConns,
				MaxConnLifetime:   pool.MaxConnLifetime,
				MaxConnIdleTime:   pool.MaxConnIdleTime,
				HealthCheckPeriod: pool.HealthCheckPeriod,
			}
		}
		if search := cfg.Config.PGVector.Search; search != nil {
			options.Search = vectordb.PGVectorSearchOptions{
				Probes:   search.Probes,
				EFSearch: search.EFSearch,
			}
		}
		storeCfg.PGVector = options
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

func resolveRedisDSN(ctx context.Context, vectorDBID string) string {
	log := logger.FromContext(ctx)
	log.Debug("redis vector_db: DSN not provided, using global redis config", "vector_db_id", vectorDBID)
	globalCfg := config.FromContext(ctx)
	if globalCfg == nil {
		log.Warn("redis vector_db: failed to retrieve global config for DSN fallback", "vector_db_id", vectorDBID)
		return ""
	}
	return buildRedisDSN(&globalCfg.Redis)
}

func buildPostgresDSN(cfg *config.DatabaseConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.ConnString != "" {
		return cfg.ConnString
	}
	host := defaultIfEmpty(cfg.Host, defaultPGHost)
	port := defaultIfEmpty(cfg.Port, defaultPGPort)
	user := defaultIfEmpty(cfg.User, defaultPGUser)
	password := defaultIfEmpty(cfg.Password, "")
	dbname := defaultIfEmpty(cfg.DBName, defaultPGDB)
	sslmode := defaultIfEmpty(cfg.SSLMode, defaultPGSSL)
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

func buildRedisDSN(cfg *config.RedisConfig) string {
	if cfg == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(cfg.URL); trimmed != "" {
		return trimmed
	}
	host := defaultIfEmpty(cfg.Host, defaultRedisHost)
	port := defaultIfEmpty(cfg.Port, defaultRedisPort)
	scheme := "redis"
	if cfg.TLSEnabled {
		scheme = "rediss"
	}
	addr := fmt.Sprintf("%s:%s", host, port)
	u := &url.URL{
		Scheme: scheme,
		Host:   addr,
		Path:   fmt.Sprintf("/%d", cfg.DB),
	}
	if pwd := strings.TrimSpace(cfg.Password); pwd != "" {
		u.User = url.UserPassword("", pwd)
	}
	return u.String()
}
