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
	provider, err := resolveVectorProvider(project, cfg)
	if err != nil {
		return nil, err
	}
	dsn := resolveVectorStoreDSN(ctx, provider, cfg)
	pathValue := resolveVectorStorePath(ctx, provider, cfg)
	if err := validateVectorDimension(project, cfg); err != nil {
		return nil, err
	}
	storeCfg := buildVectorStoreConfig(provider, cfg, dsn, pathValue)
	if cfg.Config.PGVector != nil {
		options, err := buildPGVectorOptions(project, cfg.ID, cfg.Config.PGVector)
		if err != nil {
			return nil, err
		}
		storeCfg.PGVector = options
	}
	return storeCfg, nil
}

// resolveVectorProvider normalizes and validates the requested vector DB provider.
func resolveVectorProvider(project string, cfg *knowledge.VectorDBConfig) (vectordb.Provider, error) {
	provider := vectordb.Provider(strings.TrimSpace(string(cfg.Type)))
	switch provider {
	case vectordb.ProviderPGVector, vectordb.ProviderQdrant, vectordb.ProviderRedis, vectordb.ProviderFilesystem:
		return provider, nil
	default:
		return "", fmt.Errorf("project %s vector_db %q: unsupported type %q", project, cfg.ID, cfg.Type)
	}
}

// resolveVectorStoreDSN returns the DSN for the given provider, applying fallbacks when needed.
func resolveVectorStoreDSN(
	ctx context.Context,
	provider vectordb.Provider,
	cfg *knowledge.VectorDBConfig,
) string {
	dsn := strings.TrimSpace(cfg.Config.DSN)
	switch provider {
	case vectordb.ProviderPGVector:
		if dsn == "" {
			dsn = resolvePGVectorDSN(ctx, cfg.ID)
		}
	case vectordb.ProviderRedis:
		if dsn == "" {
			dsn = resolveRedisDSN(ctx, cfg.ID)
		}
	}
	return dsn
}

// resolveVectorStorePath provides a filesystem path fallback when required.
func resolveVectorStorePath(
	ctx context.Context,
	provider vectordb.Provider,
	cfg *knowledge.VectorDBConfig,
) string {
	pathValue := strings.TrimSpace(cfg.Config.Path)
	if provider == vectordb.ProviderFilesystem && pathValue == "" {
		return defaultFilesystemPath(ctx, cfg.ID)
	}
	return pathValue
}

// validateVectorDimension checks the configured vector dimension.
func validateVectorDimension(project string, cfg *knowledge.VectorDBConfig) error {
	if cfg.Config.Dimension <= 0 {
		return fmt.Errorf(
			"project %s vector_db %q: config.dimension must be greater than zero",
			project,
			cfg.ID,
		)
	}
	return nil
}

// buildVectorStoreConfig assembles the vectordb.Config structure with normalized values.
func buildVectorStoreConfig(
	provider vectordb.Provider,
	cfg *knowledge.VectorDBConfig,
	dsn string,
	pathValue string,
) *vectordb.Config {
	storeCfg := &vectordb.Config{
		ID:          cfg.ID,
		Provider:    provider,
		DSN:         dsn,
		Path:        pathValue,
		Table:       strings.TrimSpace(cfg.Config.Table),
		Collection:  strings.TrimSpace(cfg.Config.Collection),
		Index:       strings.TrimSpace(cfg.Config.Index),
		EnsureIndex: cfg.Config.EnsureIndex,
		Metric:      strings.ToLower(strings.TrimSpace(cfg.Config.Metric)),
		Dimension:   cfg.Config.Dimension,
		Consistency: strings.TrimSpace(cfg.Config.Consistency),
		MaxTopK:     cfg.Config.MaxTopK,
	}
	if len(cfg.Config.Auth) > 0 {
		storeCfg.Auth = core.CloneMap(cfg.Config.Auth)
	}
	return storeCfg
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

func buildPGVectorOptions(
	project, vectorDBID string,
	cfg *knowledge.PGVectorConfig,
) (*vectordb.PGVectorOptions, error) {
	if err := validatePGVectorOptions(cfg); err != nil {
		return nil, fmt.Errorf("project %s vector_db %q: %w", project, vectorDBID, err)
	}
	options := &vectordb.PGVectorOptions{}
	if idx := cfg.Index; idx != nil {
		trimmedType := vectordb.PGVectorIndexType(strings.TrimSpace(strings.ToLower(idx.Type)))
		options.Index = vectordb.PGVectorIndexOptions{
			Type:           trimmedType,
			Lists:          idx.Lists,
			M:              idx.M,
			EFConstruction: idx.EFConstruction,
		}
	}
	if pool := cfg.Pool; pool != nil {
		options.Pool = vectordb.PGVectorPoolOptions{
			MinConns:          pool.MinConns,
			MaxConns:          pool.MaxConns,
			MaxConnLifetime:   pool.MaxConnLifetime,
			MaxConnIdleTime:   pool.MaxConnIdleTime,
			HealthCheckPeriod: pool.HealthCheckPeriod,
		}
	}
	if search := cfg.Search; search != nil {
		options.Search = vectordb.PGVectorSearchOptions{
			Probes:   search.Probes,
			EFSearch: search.EFSearch,
		}
	}
	return options, nil
}

func validatePGVectorOptions(opts *knowledge.PGVectorConfig) error {
	if opts == nil {
		return nil
	}
	if idx := opts.Index; idx != nil {
		typeName := strings.TrimSpace(strings.ToLower(idx.Type))
		if typeName != "" &&
			typeName != string(vectordb.PGVectorIndexIVFFlat) &&
			typeName != string(vectordb.PGVectorIndexHNSW) {
			return fmt.Errorf("pgvector index.type must be one of {ivfflat,hnsw}: %q", idx.Type)
		}
		if idx.Lists < 0 {
			return fmt.Errorf("pgvector index.lists cannot be negative: %d", idx.Lists)
		}
		if idx.M < 0 {
			return fmt.Errorf("pgvector index.m cannot be negative: %d", idx.M)
		}
		if idx.EFConstruction < 0 {
			return fmt.Errorf("pgvector index.ef_construction cannot be negative: %d", idx.EFConstruction)
		}
	}
	if search := opts.Search; search != nil {
		if search.Probes < 0 {
			return fmt.Errorf("pgvector search.probes cannot be negative: %d", search.Probes)
		}
		if search.EFSearch < 0 {
			return fmt.Errorf("pgvector search.ef_search cannot be negative: %d", search.EFSearch)
		}
	}
	if pool := opts.Pool; pool != nil {
		if err := validatePGVectorPoolOptions(pool); err != nil {
			return err
		}
	}
	return nil
}

func validatePGVectorPoolOptions(pool *knowledge.PGVectorPoolConfig) error {
	if pool.MinConns < 0 {
		return fmt.Errorf("pgvector pool.min_conns cannot be negative: %d", pool.MinConns)
	}
	if pool.MaxConns < 0 {
		return fmt.Errorf("pgvector pool.max_conns cannot be negative: %d", pool.MaxConns)
	}
	if pool.MaxConns > 0 && pool.MinConns > pool.MaxConns {
		return fmt.Errorf(
			"pgvector pool.min_conns (%d) cannot exceed max_conns (%d)",
			pool.MinConns,
			pool.MaxConns,
		)
	}
	if pool.MaxConnLifetime < 0 {
		return fmt.Errorf("pgvector pool.max_conn_lifetime cannot be negative: %s", pool.MaxConnLifetime)
	}
	if pool.MaxConnIdleTime < 0 {
		return fmt.Errorf("pgvector pool.max_conn_idle_time cannot be negative: %s", pool.MaxConnIdleTime)
	}
	if pool.HealthCheckPeriod < 0 {
		return fmt.Errorf("pgvector pool.health_check_period cannot be negative: %s", pool.HealthCheckPeriod)
	}
	return nil
}
