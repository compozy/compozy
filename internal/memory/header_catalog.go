package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// CatalogHeaders lists every header for one bound storage identity without the prompt scan cap.
func (s *Store) CatalogHeaders(ctx context.Context, scope memcontract.Scope) ([]memcontract.Header, error) {
	if ctx == nil {
		return nil, errors.New("memory: header catalog context is required")
	}
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate(); err != nil {
		return nil, wrapValidationError("list header catalog", string(scope), err)
	}
	workspaceRoot, workspaceID, err := s.catalogWorkspaceForScope(ctx, normalizedScope)
	if err != nil {
		return nil, err
	}
	identity := newCatalogIdentity(
		normalizedScope,
		workspaceID,
		s.catalogAgentName(normalizedScope),
		s.catalogAgentTier(normalizedScope),
	)

	if s.catalog == nil {
		headers, err := s.scan(normalizedScope, 0)
		if err != nil {
			return nil, err
		}
		applyHeaderCatalogIdentity(headers, identity)
		return headers, nil
	}

	if err := s.ensureCatalogFilterReady(ctx, catalogFilter{
		scope:         identity.scope,
		workspaceRoot: workspaceRoot,
		workspaceID:   identity.workspaceID,
	}); err != nil {
		return nil, err
	}
	return s.catalog.listHeaders(ctx, identity)
}

func (c *catalog) listHeaders(
	ctx context.Context,
	identity catalogIdentity,
) (headers []memcontract.Header, err error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return []memcontract.Header{}, nil
	}

	rows, err := db.QueryContext(
		ctx,
		`SELECT scope, workspace_id, agent_name, agent_tier, filename, type, name, description, mtime_ms
		 FROM memory_catalog_entries
		 WHERE scope = ? AND workspace_id = ? AND agent_name = ? AND agent_tier = ?
		 ORDER BY mtime_ms DESC, filename ASC`,
		string(identity.scope),
		identity.workspaceID,
		identity.agentName,
		string(identity.agentTier),
	)
	if err != nil {
		return nil, fmt.Errorf("memory: list lean catalog headers: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("memory: close lean catalog header rows: %w", closeErr))
		}
	}()

	headers = make([]memcontract.Header, 0)
	for rows.Next() {
		header, scanErr := scanCatalogHeader(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		headers = append(headers, header)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate lean catalog headers: %w", err)
	}
	return headers, nil
}

func scanCatalogHeader(scanner interface{ Scan(dest ...any) error }) (memcontract.Header, error) {
	var (
		header       memcontract.Header
		scopeRaw     string
		agentTierRaw string
		typeRaw      string
		mtimeMillis  int64
	)
	if err := scanner.Scan(
		&scopeRaw,
		&header.WorkspaceID,
		&header.AgentName,
		&agentTierRaw,
		&header.Filename,
		&typeRaw,
		&header.Name,
		&header.Description,
		&mtimeMillis,
	); err != nil {
		return memcontract.Header{}, fmt.Errorf("memory: scan lean catalog header: %w", err)
	}
	header.Scope = memcontract.Scope(scopeRaw).Normalize()
	header.AgentTier = memcontract.AgentTier(agentTierRaw).Normalize()
	header.Type = memcontract.Type(typeRaw).Normalize()
	header.ModTime = timeFromUnixMillis(mtimeMillis)
	if err := header.Validate(); err != nil {
		return memcontract.Header{}, fmt.Errorf(
			"memory: validate lean catalog header %q: %w",
			strings.TrimSpace(header.Filename),
			err,
		)
	}
	return header, nil
}

func applyHeaderCatalogIdentity(headers []memcontract.Header, identity catalogIdentity) {
	for index := range headers {
		headers[index].Scope = identity.scope
		headers[index].WorkspaceID = identity.workspaceID
		headers[index].AgentName = identity.agentName
		headers[index].AgentTier = identity.agentTier
	}
}
