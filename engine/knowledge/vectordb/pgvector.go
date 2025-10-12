package vectordb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

type pgStore struct {
	pool       *pgxpool.Pool
	table      string
	tableIdent string
	indexName  string
	indexIdent string
	dimension  int
	metric     string
	ensureIdx  bool
}

func newPGStore(ctx context.Context, cfg *Config) (Store, error) {
	if cfg == nil {
		return nil, errors.New("vector_db config is required")
	}
	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("vector_db %q: failed to connect to postgres: %w", cfg.ID, err)
	}
	store := &pgStore{
		pool:      pool,
		table:     chooseTable(cfg),
		indexName: chooseIndex(cfg),
		dimension: cfg.Dimension,
		metric:    strings.ToLower(strings.TrimSpace(cfg.Metric)),
		ensureIdx: cfg.EnsureIndex,
	}
	store.tableIdent = pgx.Identifier{store.table}.Sanitize()
	if store.indexName != "" {
		store.indexIdent = pgx.Identifier{store.indexName}.Sanitize()
	}
	if err := store.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
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
	if p.ensureIdx {
		distance := "cosine"
		if p.metric != "" {
			distance = p.metric
		}
		createIndex := fmt.Sprintf(
			"CREATE INDEX IF NOT EXISTS %s ON %s USING ivfflat (embedding vector_%s_ops)",
			p.indexIdent,
			p.tableIdent,
			distance,
		)
		if _, err = conn.Exec(ctx, createIndex); err != nil {
			return fmt.Errorf("pgvector: create index: %w", err)
		}
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
	for i := range records {
		rec := records[i]
		if len(rec.Embedding) != p.dimension {
			return fmt.Errorf(
				"pgvector: record %q dimension mismatch (got %d want %d)",
				rec.ID,
				len(rec.Embedding),
				p.dimension,
			)
		}
		vector := pgvector.NewVector(rec.Embedding)
		metadata, marshalErr := json.Marshal(rec.Metadata)
		if marshalErr != nil {
			return fmt.Errorf("pgvector: marshal metadata for %q: %w", rec.ID, marshalErr)
		}
		if _, execErr := tx.Exec(ctx, stmt, rec.ID, vector, rec.Text, metadata, time.Now().UTC()); execErr != nil {
			return fmt.Errorf("pgvector: upsert %q: %w", rec.ID, execErr)
		}
	}
	return nil
}

func (p *pgStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]Match, error) {
	if len(query) != p.dimension {
		return nil, errors.New("pgvector: query dimension mismatch")
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}
	builder := strings.Builder{}
	builder.WriteString("SELECT id, document, metadata, 1 - (embedding <=> $1) AS score FROM ")
	builder.WriteString(p.tableIdent)
	builder.WriteString(" WHERE 1=1")
	args := []any{pgvector.NewVector(query)}
	argPos := 2
	for key, value := range opts.Filters {
		builder.WriteString(fmt.Sprintf(" AND metadata ->> $%d = $%d", argPos, argPos+1))
		args = append(args, key, value)
		argPos += 2
	}
	if opts.MinScore > 0 {
		builder.WriteString(fmt.Sprintf(" AND 1 - (embedding <=> $1) >= $%d", argPos))
		args = append(args, opts.MinScore)
		argPos++
	}
	builder.WriteString(" ORDER BY embedding <=> $1 ASC LIMIT $")
	builder.WriteString(fmt.Sprint(argPos))
	args = append(args, topK)
	rows, err := p.pool.Query(ctx, builder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("pgvector: search: %w", err)
	}
	defer rows.Close()
	results := make([]Match, 0, topK)
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
		if score < opts.MinScore {
			continue
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
