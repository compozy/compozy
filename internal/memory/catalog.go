package memory

import (
	"context"

	"database/sql"

	"errors"
	"fmt"

	"strings"
	"sync"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
)

const (
	catalogEFilenamePath    = "  e.filename,"
	catalogENamePath        = "  e.name,"
	catalogEScopePath       = "  e.scope,"
	catalogETypePath        = "  e.type,"
	catalogEWorkspaceIDPath = "  e.workspace_id,"
	catalogSelectValue      = "SELECT"
)

const (
	defaultSearchLimit         = 10
	maxSearchLimit             = 50
	defaultHistoryLimit        = 25
	maxHistoryLimit            = 100
	maxOperationSummaryBytes   = 2048
	catalogStateKeyLastReindex = "last_reindex_at"
	catalogStateKeyScopePrefix = "scope_synced::"
	catalogEventAgentName      = "daemon"
)

const (
	memoryEventWriteCommitted         = eventspkg.MemoryWriteCommitted
	memoryEventWriteRejected          = eventspkg.MemoryWriteRejected
	memoryEventWriteShadowed          = eventspkg.MemoryWriteShadowed
	memoryEventWriteReindex           = eventspkg.MemoryWriteReindex
	memoryEventWriteReverted          = eventspkg.MemoryWriteReverted
	memoryEventRecallExecuted         = eventspkg.MemoryRecallExecuted
	memoryEventRecallSkipped          = eventspkg.MemoryRecallSkipped
	memoryEventRecallSignalDropped    = eventspkg.MemoryRecallDropped
	memoryEventRecallSignalFailed     = eventspkg.MemoryRecallFailed
	memoryEventDecisionsSummarized    = eventspkg.MemoryDecisionsSummary
	memoryEventDecisionsPruned        = eventspkg.MemoryDecisionsPruned
	memoryEventDreamStarted           = eventspkg.MemoryDreamStarted
	memoryEventDreamPromoted          = eventspkg.MemoryDreamPromoted
	memoryEventDreamFailed            = eventspkg.MemoryDreamFailed
	memoryEventExtractorStarted       = eventspkg.MemoryExtractorStarted
	memoryEventExtractorCompleted     = eventspkg.MemoryExtractorComplete
	memoryEventExtractorFailed        = eventspkg.MemoryExtractorFailed
	memoryEventExtractorCoalesced     = eventspkg.MemoryExtractorCoalesced
	memoryEventExtractorDropped       = eventspkg.MemoryExtractorDropped
	memoryEventDailyRotated           = eventspkg.MemoryDailyRotated
	memoryEventDailyArchived          = eventspkg.MemoryDailyArchived
	memoryEventDailyRestored          = eventspkg.MemoryDailyRestored
	memoryEventDailyPurged            = eventspkg.MemoryDailyPurged
	memoryEventDailyArchivePurged     = eventspkg.MemoryDailyArchivePurged
	memoryEventProviderEnabled        = eventspkg.MemoryProviderEnabled
	memoryEventProviderDisabled       = eventspkg.MemoryProviderDisabled
	memoryEventProviderCollision      = eventspkg.MemoryProviderCollision
	memoryEventWorkspaceRelocated     = eventspkg.MemoryWorkspaceRelocated
	memoryEventWorkspaceRecovered     = eventspkg.MemoryWorkspaceRecovered
	memoryEventAgentPurged            = eventspkg.MemoryAgentPurged
	memoryEventMigrationApplied       = eventspkg.MemoryMigrationApplied
	memoryEventMetadataActionKey      = "action"
	memoryEventMetadataFilenameKey    = "filename"
	memoryEventMetadataLegacyIDKey    = "legacy_id"
	memoryEventMetadataSummaryKey     = "summary"
	memoryEventMetadataQueryKey       = "query"
	memoryEventMetadataResultCountKey = "result_count"
)

type catalog struct {
	path    string
	now     func() time.Time
	writeMu sync.Mutex

	mu sync.Mutex
	db *sql.DB
}

type catalogDocument struct {
	ID            string
	Scope         memcontract.Scope
	WorkspaceID   string
	WorkspaceRoot string
	AgentName     string
	AgentTier     memcontract.AgentTier
	Filename      string
	Type          memcontract.Type
	Name          string
	Description   string
	Content       string
	ContentHash   string
	Injection     bool
	UpdatedAt     time.Time
}

type catalogChunk struct {
	id          string
	content     string
	contentHash string
	startLine   int
	endLine     int
}

type catalogWriteExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func newCatalog(path string, now func() time.Time) *catalog {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &catalog{path: cleanPath, now: now}
}

func (c *catalog) enabled() bool {
	return c != nil && strings.TrimSpace(c.path) != ""
}

func (c *catalog) open(ctx context.Context) error {
	if !c.enabled() {
		return nil
	}
	if ctx == nil {
		return errors.New("memory: catalog open context is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		return nil
	}

	db, err := storepkg.OpenSQLiteDatabase(ctx, c.path, func(ctx context.Context, db *sql.DB) error {
		return storepkg.Apply(ctx, db, MigrationStream())
	})
	if err != nil {
		return fmt.Errorf("memory: open catalog database %q: %w", c.path, err)
	}
	c.db = db
	return nil
}

func (c *catalog) ensureDB(ctx context.Context) (*sql.DB, error) {
	if !c.enabled() {
		return nil, nil
	}
	if ctx == nil {
		return nil, errors.New("memory: catalog context is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil, fmt.Errorf("memory: catalog database %q is not open", c.path)
	}
	return c.db, nil
}

func (c *catalog) close(ctx context.Context) error {
	if !c.enabled() {
		return nil
	}
	if ctx == nil {
		return errors.New("memory: catalog close context is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil
	}
	db := c.db
	c.db = nil
	return errors.Join(storepkg.Checkpoint(ctx, db), db.Close())
}

func (c *catalog) replaceScope(
	ctx context.Context,
	scope memcontract.Scope,
	workspaceID string,
	agentName string,
	agentTier memcontract.AgentTier,
	docs []catalogDocument,
) (err error) {
	return c.withCatalogWriteTx(ctx, "catalog scope replace", func(tx *storepkg.WriteTx) error {
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM memory_catalog_entries
			 WHERE scope = ? AND workspace_id = ? AND agent_name = ? AND agent_tier = ?`,
			string(scope.Normalize()),
			strings.TrimSpace(workspaceID),
			strings.TrimSpace(agentName),
			string(agentTier.Normalize()),
		); err != nil {
			return fmt.Errorf("memory: clear catalog scope %q/%q: %w", scope, workspaceID, err)
		}
		for _, doc := range docs {
			if err := insertCatalogDocumentTx(ctx, tx, doc); err != nil {
				return err
			}
			if err := replaceCatalogChunksTx(ctx, tx, doc); err != nil {
				return err
			}
		}
		return c.upsertCatalogIdentityStateTx(
			ctx,
			tx,
			newCatalogIdentity(scope, workspaceID, agentName, agentTier),
		)
	})
}

func (c *catalog) upsertDocument(ctx context.Context, doc catalogDocument) (err error) {
	return c.withCatalogWriteTx(ctx, "catalog document upsert", func(tx *storepkg.WriteTx) error {
		if err := upsertCatalogDocumentTx(ctx, tx, doc); err != nil {
			return err
		}
		if err := replaceCatalogChunksTx(ctx, tx, doc); err != nil {
			return err
		}
		if err := c.upsertCatalogIdentityStateTx(
			ctx,
			tx,
			newCatalogIdentity(doc.Scope, doc.WorkspaceID, doc.AgentName, doc.AgentTier),
		); err != nil {
			return err
		}
		if err := upsertCatalogStateTx(
			ctx,
			tx,
			catalogStateKeyLastReindex,
			storepkg.FormatTimestamp(c.now().UTC()),
		); err != nil {
			return fmt.Errorf("memory: persist catalog reindex timestamp: %w", err)
		}
		return nil
	})
}

func (c *catalog) withCatalogWriteTx(
	ctx context.Context,
	operation string,
	fn func(*storepkg.WriteTx) error,
) (err error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	db, err := c.ensureDB(ctx)
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}

	if err := storepkg.ExecuteWrite(ctx, db, func(writeCtx context.Context, tx *storepkg.WriteTx) error {
		if err := writeCtx.Err(); err != nil {
			return err
		}
		if err := fn(tx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("memory: %s: %w", operation, err)
	}
	return nil
}

func insertCatalogDocumentTx(ctx context.Context, tx catalogWriteExecutor, doc catalogDocument) error {
	return insertCatalogDocumentNewTx(ctx, tx, doc)
}

func insertCatalogDocumentNewTx(ctx context.Context, tx catalogWriteExecutor, doc catalogDocument) error {
	return insertCatalogDocumentIntoTx(ctx, tx, "memory_catalog_entries", doc)
}

func insertCatalogDocumentIntoTx(
	ctx context.Context,
	tx catalogWriteExecutor,
	table string,
	doc catalogDocument,
) error {
	tableName, err := storepkg.NormalizeSQLiteIdentifier(table)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(`INSERT INTO %s (
			id, workspace_id, scope, agent_name, agent_tier, type, slug, filename, name,
			description, content, content_hash, injection, mtime_ms, indexed_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, tableName),
		doc.ID,
		strings.TrimSpace(doc.WorkspaceID),
		string(doc.Scope.Normalize()),
		strings.TrimSpace(doc.AgentName),
		string(doc.AgentTier.Normalize()),
		string(doc.Type.Normalize()),
		catalogSlug(doc.Filename),
		doc.Filename,
		doc.Name,
		doc.Description,
		doc.Content,
		doc.ContentHash,
		boolToInt(doc.Injection),
		timeToUnixMillis(doc.UpdatedAt),
		timeToUnixMillis(doc.UpdatedAt),
		storepkg.FormatTimestamp(doc.UpdatedAt),
	); err != nil {
		return fmt.Errorf("memory: insert catalog entry %q: %w", doc.Filename, err)
	}
	return nil
}

func upsertCatalogDocumentTx(ctx context.Context, tx catalogWriteExecutor, doc catalogDocument) error {
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO memory_catalog_entries (
			id, workspace_id, scope, agent_name, agent_tier, type, slug, filename, name,
			description, content, content_hash, injection, mtime_ms, indexed_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, scope, agent_name, agent_tier, type, slug) DO UPDATE SET
			id = excluded.id,
			filename = excluded.filename,
			type = excluded.type,
			name = excluded.name,
			description = excluded.description,
			content = excluded.content,
			content_hash = excluded.content_hash,
			injection = excluded.injection,
			mtime_ms = excluded.mtime_ms,
			indexed_at = excluded.indexed_at,
			updated_at = excluded.updated_at`,
		doc.ID,
		strings.TrimSpace(doc.WorkspaceID),
		string(doc.Scope.Normalize()),
		strings.TrimSpace(doc.AgentName),
		string(doc.AgentTier.Normalize()),
		string(doc.Type.Normalize()),
		catalogSlug(doc.Filename),
		doc.Filename,
		doc.Name,
		doc.Description,
		doc.Content,
		doc.ContentHash,
		boolToInt(doc.Injection),
		timeToUnixMillis(doc.UpdatedAt),
		timeToUnixMillis(doc.UpdatedAt),
		storepkg.FormatTimestamp(doc.UpdatedAt),
	); err != nil {
		return fmt.Errorf("memory: upsert catalog entry %q: %w", doc.Filename, err)
	}
	return nil
}

func replaceCatalogChunksTx(ctx context.Context, tx catalogWriteExecutor, doc catalogDocument) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_chunks WHERE file_id = ?`, doc.ID); err != nil {
		return fmt.Errorf("memory: delete catalog chunks for %q: %w", doc.Filename, err)
	}
	for _, chunk := range catalogChunksForDocument(doc) {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO memory_chunks (
				id, file_id, content, content_hash, start_line, end_line, indexed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			chunk.id,
			doc.ID,
			chunk.content,
			chunk.contentHash,
			chunk.startLine,
			chunk.endLine,
			timeToUnixMillis(doc.UpdatedAt),
		); err != nil {
			return fmt.Errorf("memory: insert catalog chunk for %q: %w", doc.Filename, err)
		}
	}
	return nil
}
