package vectordb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

type pgStore struct {
	pool               *pgxpool.Pool
	table              string
	tableIdent         string
	indexName          string
	indexIdent         string
	dimension          int
	metric             string
	ensureIdx          bool
	maxTopK            int
	indexType          string
	ivfLists           int
	hnswM              int
	hnswEFConstruction int
	hnswEFSearch       int
	searchProbes       int
	searchEFSearch     int
}

const (
	pgvectorDefaultTopK         = 5
	pgvectorDefaultMaxTopK      = 1000
	pgvectorDefaultProbes       = 10
	pgvectorDefaultIVFLists     = 100
	pgvectorDefaultHNSWM        = 16
	pgvectorDefaultHNSWEFBuild  = 64
	pgvectorDefaultHNSWEFSearch = 40
)

const (
	metricCosine = "cosine"
	metricL2     = "l2"
	metricIP     = "ip"
)

func newPGStore(ctx context.Context, cfg *Config) (Store, error) {
	if cfg == nil {
		return nil, errors.New("vector_db config is required")
	}
	if cfg.MaxTopK < 0 {
		return nil, fmt.Errorf("vector_db %q: max_top_k must be non-negative", cfg.ID)
	}
	dsn := strings.TrimSpace(cfg.DSN)
	if dsn == "" {
		dsn = resolvePGVectorDSN(ctx, cfg)
	}
	if dsn == "" {
		return nil, errors.New("vector_db dsn is required for pgvector")
	}
	cfg.DSN = dsn
	pool, err := setupPGPool(ctx, cfg)
	if err != nil {
		return nil, err
	}
	store := &pgStore{
		pool:      pool,
		table:     chooseTable(cfg),
		indexName: chooseIndex(cfg),
		dimension: cfg.Dimension,
		metric:    strings.ToLower(strings.TrimSpace(cfg.Metric)),
		ensureIdx: cfg.EnsureIndex,
		maxTopK:   cfg.MaxTopK,
	}
	applyPGVectorOptions(store, cfg.PGVector)
	if store.maxTopK == 0 {
		store.maxTopK = pgvectorDefaultMaxTopK
	}
	store.tableIdent = sanitizeIdentifier(store.table)
	if store.indexName != "" {
		store.indexIdent = sanitizeIdentifier(store.indexName)
	}
	if err := store.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func setupPGPool(ctx context.Context, cfg *Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("vector_db %q: parse pool config: %w", cfg.ID, err)
	}
	applyPoolOptions(poolConfig, cfg.PGVector)
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "compozy_knowledge_pgvector"
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("vector_db %q: failed to connect to postgres: %w", cfg.ID, err)
	}
	return pool, nil
}

func applyPoolOptions(poolConfig *pgxpool.Config, opts *PGVectorOptions) {
	if opts == nil {
		return
	}
	pool := opts.Pool
	if pool.MinConns > 0 {
		poolConfig.MinConns = pool.MinConns
	}
	if pool.MaxConns > 0 {
		poolConfig.MaxConns = pool.MaxConns
	}
	if pool.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = pool.MaxConnLifetime
	}
	if pool.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = pool.MaxConnIdleTime
	}
	if pool.HealthCheckPeriod > 0 {
		poolConfig.HealthCheckPeriod = pool.HealthCheckPeriod
	}
}

func applyPGVectorOptions(store *pgStore, opts *PGVectorOptions) {
	store.indexType = "ivfflat"
	store.searchProbes = pgvectorDefaultProbes
	if opts == nil {
		return
	}
	if idx := opts.Index; (idx != PGVectorIndexOptions{}) {
		if trimmed := strings.TrimSpace(idx.Type); trimmed != "" {
			store.indexType = strings.ToLower(trimmed)
		}
		if idx.Lists > 0 {
			store.ivfLists = idx.Lists
		}
		if idx.M > 0 {
			store.hnswM = idx.M
		}
		if idx.EFConstruction > 0 {
			store.hnswEFConstruction = idx.EFConstruction
		}
		if idx.EFSearch > 0 {
			store.hnswEFSearch = idx.EFSearch
		}
		if idx.Probes > 0 {
			store.searchProbes = idx.Probes
		}
	}
	if search := opts.Search; (search != PGVectorSearchOptions{}) {
		if search.Probes > 0 {
			store.searchProbes = search.Probes
		}
		if search.EFSearch > 0 {
			store.searchEFSearch = search.EFSearch
		}
	}
	if store.indexType == "" {
		store.indexType = "ivfflat"
	}
}

func resolvePGVectorDSN(ctx context.Context, cfg *Config) string {
	log := logger.FromContext(ctx)
	globalCfg := config.FromContext(ctx)
	if globalCfg == nil {
		log.Warn("pgvector: missing global config for DSN fallback", "vector_db_id", cfg.ID)
		return ""
	}
	if conn := strings.TrimSpace(globalCfg.Database.ConnString); conn != "" {
		log.Debug("pgvector: using global postgres conn_string fallback", "vector_db_id", cfg.ID)
		return conn
	}
	dsn := buildPostgresDSNFromDatabase(&globalCfg.Database)
	if strings.TrimSpace(dsn) == "" {
		log.Warn("pgvector: global database config incomplete for DSN fallback", "vector_db_id", cfg.ID)
		return ""
	}
	log.Debug("pgvector: built DSN from global database settings", "vector_db_id", cfg.ID)
	return dsn
}

func buildPostgresDSNFromDatabase(dbCfg *config.DatabaseConfig) string {
	if dbCfg == nil {
		return ""
	}
	host := fallbackString(dbCfg.Host, "localhost")
	port := fallbackString(dbCfg.Port, "5432")
	user := fallbackString(dbCfg.User, "postgres")
	password := dbCfg.Password
	dbname := fallbackString(dbCfg.DBName, "postgres")
	sslmode := fallbackString(dbCfg.SSLMode, "disable")
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)
}

func fallbackString(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func chooseTable(cfg *Config) string {
	if cfg == nil {
		return "knowledge_chunks"
	}
	if cfg.Table != "" {
		return cfg.Table
	}
	if cfg.Collection != "" {
		return cfg.Collection
	}
	return "knowledge_chunks"
}

func chooseIndex(cfg *Config) string {
	if cfg == nil {
		return "knowledge_chunks_embedding_idx"
	}
	if cfg.Index != "" {
		return cfg.Index
	}
	table := chooseTable(cfg)
	return fmt.Sprintf("%s_embedding_idx", table)
}

func sanitizeIdentifier(raw string) string {
	parts := strings.Split(raw, ".")
	ident := make(pgx.Identifier, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			ident = append(ident, part)
		}
	}
	if len(ident) == 0 {
		return pgx.Identifier{raw}.Sanitize()
	}
	return ident.Sanitize()
}

func (p *pgStore) ensureSchema(ctx context.Context) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("pgvector: acquire connection: %w", err)
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		return fmt.Errorf("pgvector: enable extension: %w", err)
	}
	createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id TEXT PRIMARY KEY,
		embedding vector(%d),
		document TEXT,
		metadata JSONB,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`, p.tableIdent, p.dimension)
	if _, err = conn.Exec(ctx, createTable); err != nil {
		return fmt.Errorf("pgvector: create table: %w", err)
	}
	if err := p.ensureMetadataIndex(ctx, conn); err != nil {
		return err
	}
	if p.ensureIdx && p.indexIdent != "" {
		if err := p.ensureVectorIndex(ctx, conn); err != nil {
			return err
		}
	}
	return nil
}

func (p *pgStore) ensureMetadataIndex(ctx context.Context, conn *pgxpool.Conn) error {
	metadataIndexName := fmt.Sprintf("%s_metadata_idx", strings.ReplaceAll(p.table, ".", "_"))
	metadataIdent := sanitizeIdentifier(metadataIndexName)
	createMetadata := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s USING gin (metadata)",
		metadataIdent,
		p.tableIdent,
	)
	if _, err := conn.Exec(ctx, createMetadata); err != nil {
		return fmt.Errorf("pgvector: create metadata index: %w", err)
	}
	return nil
}

func (p *pgStore) ensureVectorIndex(ctx context.Context, conn *pgxpool.Conn) error {
	opClass, err := p.operatorClass()
	if err != nil {
		return err
	}
	switch strings.ToLower(p.indexType) {
	case "hnsw":
		return p.createHNSWIndex(ctx, conn, opClass)
	default:
		return p.createIVFFlatIndex(ctx, conn, opClass)
	}
}

func (p *pgStore) operatorClass() (string, error) {
	distance := metricCosine
	if p.metric != "" {
		switch p.metric {
		case metricCosine, metricL2, metricIP:
			distance = p.metric
		default:
			return "", fmt.Errorf("pgvector: unsupported metric %q", p.metric)
		}
	}
	return fmt.Sprintf("vector_%s_ops", distance), nil
}

func (p *pgStore) createIVFFlatIndex(ctx context.Context, conn *pgxpool.Conn, opClass string) error {
	lists := p.ivfLists
	if lists <= 0 {
		lists = pgvectorDefaultIVFLists
	}
	create := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s USING ivfflat (embedding %s) WITH (lists = %d)",
		p.indexIdent,
		p.tableIdent,
		opClass,
		lists,
	)
	if _, err := conn.Exec(ctx, create); err != nil {
		return fmt.Errorf("pgvector: create ivfflat index: %w", err)
	}
	return nil
}

func (p *pgStore) createHNSWIndex(ctx context.Context, conn *pgxpool.Conn, opClass string) error {
	m := p.hnswM
	if m <= 0 {
		m = pgvectorDefaultHNSWM
	}
	efBuild := p.hnswEFConstruction
	if efBuild <= 0 {
		efBuild = pgvectorDefaultHNSWEFBuild
	}
	withClauses := []string{
		fmt.Sprintf("m = %d", m),
		fmt.Sprintf("ef_construction = %d", efBuild),
	}
	create := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s USING hnsw (embedding %s) WITH (%s)",
		p.indexIdent,
		p.tableIdent,
		opClass,
		strings.Join(withClauses, ", "),
	)
	if _, err := conn.Exec(ctx, create); err != nil {
		return fmt.Errorf("pgvector: create hnsw index: %w", err)
	}
	return nil
}

// prepareBatchRecords validates each record's dimension, marshals its metadata,
// enqueues it into the batch, and returns the ordered list of IDs.
func prepareBatchRecords(records []Record, dimension int, stmt string, batch *pgx.Batch) ([]string, error) {
	ids := make([]string, 0, len(records))
	for i := range records {
		rec := records[i]
		if len(rec.Embedding) != dimension {
			return nil, fmt.Errorf(
				"pgvector: record %q dimension mismatch (got %d want %d)",
				rec.ID,
				len(rec.Embedding),
				dimension,
			)
		}
		vector := pgvector.NewVector(rec.Embedding)
		metadata, marshalErr := json.Marshal(rec.Metadata)
		if marshalErr != nil {
			return nil, fmt.Errorf("pgvector: marshal metadata for %q: %w", rec.ID, marshalErr)
		}
		batch.Queue(stmt, rec.ID, vector, rec.Text, metadata, time.Now().UTC())
		ids = append(ids, rec.ID)
	}
	return ids, nil
}

// executeBatchResults sends the batch, iterates over results in order, and
// returns the first Exec error encountered (if any).
func executeBatchResults(ctx context.Context, tx pgx.Tx, batch *pgx.Batch, ids []string) error {
	results := tx.SendBatch(ctx, batch)
	for i := range ids {
		if _, execErr := results.Exec(); execErr != nil {
			_ = results.Close()
			return fmt.Errorf("pgvector: upsert %q: %w", ids[i], execErr)
		}
	}
	if closeErr := results.Close(); closeErr != nil {
		return fmt.Errorf("pgvector: close batch: %w", closeErr)
	}
	return nil
}

func (p *pgStore) Upsert(ctx context.Context, records []Record) (err error) {
	if len(records) == 0 {
		return nil
	}
	tx, txErr := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if txErr != nil {
		return fmt.Errorf("pgvector: begin tx: %w", txErr)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = fmt.Errorf("pgvector: rollback failed: %w; original error: %v", rbErr, err)
			}
		} else {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				err = fmt.Errorf("pgvector: commit: %w", commitErr)
			}
		}
	}()
	stmt := fmt.Sprintf(`INSERT INTO %s (id, embedding, document, metadata, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET
    embedding = excluded.embedding,
    document = excluded.document,
    metadata = excluded.metadata,
    updated_at = excluded.updated_at`, p.tableIdent)

	batch := &pgx.Batch{}
	ids, err := prepareBatchRecords(records, p.dimension, stmt, batch)
	if err != nil {
		return err
	}
	return executeBatchResults(ctx, tx, batch, ids)
}

func (p *pgStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]Match, error) {
	if len(query) != p.dimension {
		return nil, errors.New("pgvector: query dimension mismatch")
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = pgvectorDefaultTopK
	}
	if topK > p.maxTopK {
		return nil, fmt.Errorf("pgvector: topK exceeds maximum allowed value of %d", p.maxTopK)
	}
	sql, args := buildSearchQuery(p.tableIdent, p.metric, opts.Filters, opts.MinScore)
	args[0] = pgvector.NewVector(query)
	args = append(args, topK)
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("pgvector: acquire connection: %w", err)
	}
	defer conn.Release()

	resetStmt := ""
	switch strings.ToLower(p.indexType) {
	case "hnsw":
		target := p.searchEFSearch
		if target <= 0 {
			target = p.hnswEFSearch
		}
		if target <= 0 {
			target = pgvectorDefaultHNSWEFSearch
		}
		setCmd := fmt.Sprintf("SET hnsw.ef_search = %d", target)
		if _, err := conn.Exec(ctx, setCmd); err != nil {
			return nil, fmt.Errorf("pgvector: set hnsw.ef_search: %w", err)
		}
		resetStmt = "RESET hnsw.ef_search"
	default:
		target := p.searchProbes
		if target <= 0 {
			target = pgvectorDefaultProbes
		}
		if target < 1 {
			target = 1
		}
		setCmd := fmt.Sprintf("SET ivfflat.probes = %d", target)
		if _, err := conn.Exec(ctx, setCmd); err != nil {
			return nil, fmt.Errorf("pgvector: set ivfflat.probes: %w", err)
		}
		resetStmt = "RESET ivfflat.probes"
	}
	defer func() {
		if resetStmt == "" {
			return
		}
		resetCtx := context.WithoutCancel(ctx)
		if _, resetErr := conn.Exec(resetCtx, resetStmt); resetErr != nil {
			logger.FromContext(resetCtx).Warn(
				"pgvector: failed to reset search parameter",
				"parameter", resetStmt,
				"error", resetErr,
			)
		}
	}()

	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("pgvector: search: %w", err)
	}
	defer rows.Close()
	return scanSearchResults(rows, topK)
}

func buildSearchQuery(tableIdent string, metric string, filters map[string]string, minScore float64) (string, []any) {
	builder := strings.Builder{}
	scoreClause, orderExpr := searchExpressions(metric)
	builder.WriteString("SELECT id, document, metadata, ")
	builder.WriteString(scoreClause)
	builder.WriteString(" AS score FROM ")
	builder.WriteString(tableIdent)
	builder.WriteString(" WHERE 1=1")
	args := make([]any, 1, 1+len(filters)*2+2)
	argPos := 2
	for key, value := range filters {
		builder.WriteString(fmt.Sprintf(" AND metadata ->> $%d = $%d", argPos, argPos+1))
		args = append(args, key, value)
		argPos += 2
	}
	if minScore > 0 {
		builder.WriteString(fmt.Sprintf(" AND %s >= $%d", scoreClause, argPos))
		args = append(args, minScore)
		argPos++
	}
	builder.WriteString(" ORDER BY ")
	builder.WriteString(orderExpr)
	builder.WriteString(" ASC LIMIT $")
	builder.WriteString(fmt.Sprint(argPos))
	return builder.String(), args
}

func searchExpressions(metric string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case metricL2:
		return "1.0 / (1.0 + (embedding <-> $1))", "embedding <-> $1"
	case metricIP:
		return "-(embedding <#> $1)", "embedding <#> $1"
	default:
		return "1 - (embedding <=> $1)", "embedding <=> $1"
	}
}

func scanSearchResults(rows pgx.Rows, topK int) ([]Match, error) {
	capacity := topK
	if capacity <= 0 {
		capacity = pgvectorDefaultTopK
	}
	results := make([]Match, 0, capacity)
	for rows.Next() {
		var (
			id          string
			document    string
			metadataRaw []byte
			score       float64
		)
		if err := rows.Scan(&id, &document, &metadataRaw, &score); err != nil {
			return nil, fmt.Errorf("pgvector: scan: %w", err)
		}
		meta := make(map[string]any)
		if len(metadataRaw) > 0 {
			if unmarshalErr := json.Unmarshal(metadataRaw, &meta); unmarshalErr != nil {
				return nil, fmt.Errorf("pgvector: decode metadata: %w", unmarshalErr)
			}
		}
		results = append(results, Match{
			ID:       id,
			Score:    score,
			Text:     document,
			Metadata: meta,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgvector: search rows: %w", err)
	}
	return results, nil
}

func (p *pgStore) Delete(ctx context.Context, filter Filter) error {
	if len(filter.IDs) == 0 && len(filter.Metadata) == 0 {
		return nil
	}
	builder := strings.Builder{}
	builder.WriteString("DELETE FROM ")
	builder.WriteString(p.tableIdent)
	builder.WriteString(" WHERE 1=1")
	args := make([]any, 0)
	argPos := 1
	if len(filter.IDs) > 0 {
		builder.WriteString(fmt.Sprintf(" AND id = ANY($%d)", argPos))
		args = append(args, filter.IDs)
		argPos++
	}
	for key, value := range filter.Metadata {
		builder.WriteString(fmt.Sprintf(" AND metadata ->> $%d = $%d", argPos, argPos+1))
		args = append(args, key, value)
		argPos += 2
	}
	if _, err := p.pool.Exec(ctx, builder.String(), args...); err != nil {
		return fmt.Errorf("pgvector: delete: %w", err)
	}
	return nil
}

func (p *pgStore) Close(_ context.Context) error {
	p.pool.Close()
	return nil
}
