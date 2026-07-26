package memory

import (
	"context"

	"fmt"

	"strings"

	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
)

func (c *catalog) deleteDocument(
	ctx context.Context,
	scope memcontract.Scope,
	workspaceID string,
	agentName string,
	agentTier memcontract.AgentTier,
	filename string,
) (err error) {
	return c.withCatalogWriteTx(ctx, "catalog document delete", func(tx *storepkg.WriteTx) error {
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM memory_chunks
			 WHERE file_id IN (
				SELECT id FROM memory_catalog_entries
				WHERE scope = ? AND workspace_id = ? AND agent_name = ? AND agent_tier = ? AND filename = ?
			 )`,
			string(scope.Normalize()),
			strings.TrimSpace(workspaceID),
			strings.TrimSpace(agentName),
			string(agentTier.Normalize()),
			strings.TrimSpace(filename),
		); err != nil {
			return fmt.Errorf("memory: delete catalog chunks for %q: %w", filename, err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM memory_catalog_entries
			 WHERE scope = ? AND workspace_id = ? AND agent_name = ? AND agent_tier = ? AND filename = ?`,
			string(scope.Normalize()),
			strings.TrimSpace(workspaceID),
			strings.TrimSpace(agentName),
			string(agentTier.Normalize()),
			strings.TrimSpace(filename),
		); err != nil {
			return fmt.Errorf("memory: delete catalog entry %q: %w", filename, err)
		}
		if err := c.upsertCatalogIdentityStateTx(
			ctx,
			tx,
			newCatalogIdentity(scope, workspaceID, agentName, agentTier),
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

func (c *catalog) setLastReindex(ctx context.Context, when time.Time) error {
	if when.IsZero() {
		when = c.now()
	}
	if err := c.upsertState(
		ctx,
		catalogStateKeyLastReindex,
		storepkg.FormatTimestamp(when.UTC()),
	); err != nil {
		return fmt.Errorf("memory: persist catalog reindex timestamp: %w", err)
	}
	return nil
}

func (c *catalog) lastReindex(ctx context.Context) (*time.Time, error) {
	raw, ok, err := c.stateValue(ctx, catalogStateKeyLastReindex)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	parsed, err := storepkg.ParseTimestamp(raw)
	if err != nil {
		return nil, fmt.Errorf("memory: parse catalog reindex timestamp %q: %w", raw, err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func (c *catalog) listEntries(ctx context.Context, filters []catalogFilter) ([]catalogDocument, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}

	rows, err := db.QueryContext(
		ctx,
		`SELECT id, scope, workspace_id, agent_name, agent_tier, filename, type, name,
			description, content, content_hash, updated_at
		 FROM memory_catalog_entries
		 ORDER BY updated_at DESC, filename ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: list catalog entries: %w", err)
	}
	defer func() {
		// rows.Err() or scanErr above reports any actionable read failure after we drain this SELECT result set.
		_ = rows.Close()
	}()

	entries := make([]catalogDocument, 0)
	for rows.Next() {
		entry, scanErr := scanCatalogEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if !catalogFiltersAllow(filters, entry.Scope, entry.WorkspaceID) {
			continue
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate catalog entries: %w", err)
	}
	return entries, nil
}

func (c *catalog) search(
	ctx context.Context,
	query string,
	scope memcontract.Scope,
	workspaceID string,
	limit int,
) ([]memcontract.SearchResult, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}

	match, err := buildCatalogMatchQuery(query)
	if err != nil {
		return nil, err
	}
	limit = clampSearchLimit(limit)

	base := strings.Join([]string{
		catalogSelectValue,
		catalogEScopePath,
		catalogEWorkspaceIDPath,
		catalogEFilenamePath,
		catalogETypePath,
		catalogENamePath,
		`  e.description,`,
		`  e.updated_at,`,
		`  -bm25(memory_catalog_fts) AS score,`,
		`  snippet(memory_catalog_fts, 2, '[', ']', '...', 18) AS snippet`,
		`FROM memory_catalog_fts`,
		`JOIN memory_catalog_entries e ON e.rowid = memory_catalog_fts.rowid`,
		`WHERE memory_catalog_fts MATCH ?`,
	}, "\n")

	args := []any{match}
	base, args = appendCatalogScopeFilter(base, args, scope, workspaceID)
	base += "\nORDER BY bm25(memory_catalog_fts) ASC, e.updated_at DESC, e.filename ASC\nLIMIT ?"
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, fmt.Errorf("memory: search catalog: %w", err)
	}
	defer func() {
		// rows.Err() or scanErr above reports any actionable read failure after we drain this SELECT result set.
		_ = rows.Close()
	}()

	results := make([]memcontract.SearchResult, 0, limit)
	for rows.Next() {
		result, scanErr := scanSearchResult(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate search results: %w", err)
	}
	return results, nil
}
