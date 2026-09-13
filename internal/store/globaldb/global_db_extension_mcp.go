package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/extensionmcp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/vault"
)

var _ extensionmcp.Store = (*ExtensionMCPRepo)(nil)

func (g *GlobalDB) ExtensionMCPStore() extensionmcp.Store { return g.ExtensionMCP }

func (r *ExtensionMCPRepo) List(ctx context.Context, profileID, workspaceID string) ([]extensionmcp.Record, error) {
	if err := r.checkReady(ctx, "list extension MCP overrides"); err != nil {
		return nil, err
	}
	return listExtensionMCPRecords(ctx, r.queries, extensionMCPProfileID(profileID), strings.TrimSpace(workspaceID))
}

// Reserve commits a new allocation once; an existing allocation is never renamed.
func (r *ExtensionMCPRepo) Reserve(
	ctx context.Context, target extensionmcp.Target, requested string, occupied []string,
) (record extensionmcp.Record, err error) {
	if err := r.checkReady(ctx, "allocate extension MCP runtime name"); err != nil {
		return record, err
	}
	target, err = normalizeExtensionMCPTarget(target)
	if err != nil {
		return record, err
	}
	requested = strings.TrimSpace(requested)
	if requested != "" {
		if err := compozyconfig.ValidateMCPServerName(requested); err != nil {
			return record, err
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return record, fmt.Errorf("store: begin MCP allocation: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: rollback MCP allocation: %w", rollbackErr))
		}
	}()
	queries := sqlcgen.New(tx)
	records, err := listExtensionMCPRecords(ctx, queries, target.ProfileID, target.WorkspaceID)
	if err != nil {
		return record, err
	}
	blocked := make(map[string]bool, len(records)+len(occupied))
	for _, name := range occupied {
		blocked[strings.TrimSpace(name)] = true
	}
	for _, current := range records {
		if current.Target == target {
			if blocked[current.RuntimeName] || (requested != "" && requested != current.RuntimeName) {
				return record, extensionmcp.ErrNameTaken
			}
			return current, nil
		}
	}
	for _, current := range records {
		blocked[current.RuntimeName] = true
	}
	name, err := allocateExtensionMCPName(target, requested, blocked)
	if err != nil {
		return record, err
	}
	record = extensionmcp.Record{Target: target, RuntimeName: name, UpdatedAt: r.now().UTC()}
	if err := queries.InsertExtensionMCPAllocation(ctx, sqlcgen.InsertExtensionMCPAllocationParams{
		Extension: target.Extension, Profile: target.ProfileID, WorkspaceID: target.WorkspaceID,
		Server: target.ServerName, RuntimeName: name, UpdatedAt: store.FormatTimestamp(record.UpdatedAt),
	}); err != nil {
		return extensionmcp.Record{}, fmt.Errorf("store: insert MCP allocation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return extensionmcp.Record{}, fmt.Errorf("store: commit MCP allocation: %w", err)
	}
	return record, nil
}

func allocateExtensionMCPName(target extensionmcp.Target, requested string, occupied map[string]bool) (string, error) {
	if requested != "" {
		if occupied[requested] {
			return "", extensionmcp.ErrNameTaken
		}
		return requested, nil
	}
	if !occupied[target.ServerName] {
		return target.ServerName, nil
	}
	base := target.Extension + "." + target.ServerName
	if !occupied[base] {
		return base, nil
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s.%d", base, suffix)
		if !occupied[candidate] {
			return candidate, nil
		}
	}
}

// Update edits only override fields and leaves the allocation untouched.
func (r *ExtensionMCPRepo) Update(
	ctx context.Context,
	target extensionmcp.Target,
	override extensionmcp.Override,
) error {
	if err := r.checkReady(ctx, "update extension MCP override"); err != nil {
		return err
	}
	target, err := normalizeExtensionMCPTarget(target)
	if err != nil {
		return err
	}
	if err := override.Validate(); err != nil {
		return err
	}
	env, err := json.Marshal(override.Env)
	if err != nil {
		return err
	}
	headers, err := json.Marshal(override.Headers)
	if err != nil {
		return err
	}
	count, err := r.queries.UpdateExtensionMCPOverride(ctx, sqlcgen.UpdateExtensionMCPOverrideParams{
		Extension: target.Extension, Profile: target.ProfileID, WorkspaceID: target.WorkspaceID,
		Server: target.ServerName, EnvJson: string(env), HeadersJson: string(headers), Url: override.URL,
		UpdatedAt: store.FormatTimestamp(r.now().UTC()),
	})
	if err != nil {
		return fmt.Errorf("store: update MCP override: %w", err)
	}
	if count == 0 {
		return extensionmcp.ErrNotFound
	}
	return nil
}

func (r *ExtensionMCPRepo) DeleteInstance(ctx context.Context, extension, profileID, workspaceID string) error {
	if err := r.checkReady(ctx, "delete extension MCP overrides"); err != nil {
		return err
	}
	if strings.TrimSpace(extension) == "" {
		return errors.New("store: extension name is required")
	}
	return r.queries.DeleteExtensionMCPOverrides(ctx, sqlcgen.DeleteExtensionMCPOverridesParams{
		Extension: strings.TrimSpace(
			extension,
		),
		Profile:     extensionMCPProfileID(profileID),
		WorkspaceID: strings.TrimSpace(workspaceID),
	})
}

func listExtensionMCPRecords(
	ctx context.Context,
	queries *sqlcgen.Queries,
	profileID, workspaceID string,
) ([]extensionmcp.Record, error) {
	rows, err := queries.ListExtensionMCPOverrides(
		ctx,
		sqlcgen.ListExtensionMCPOverridesParams{Profile: profileID, WorkspaceID: workspaceID},
	)
	if err != nil {
		return nil, fmt.Errorf("store: list MCP overrides: %w", err)
	}
	return decodeExtensionMCPRecords(rows)
}

func decodeExtensionMCPRecords(rows []sqlcgen.ExtensionMcpOverride) ([]extensionmcp.Record, error) {
	var err error
	records := make([]extensionmcp.Record, 0, len(rows))
	for _, row := range rows {
		record := extensionmcp.Record{
			Target: extensionmcp.Target{
				Extension:   row.Extension,
				ProfileID:   row.Profile,
				WorkspaceID: row.WorkspaceID,
				ServerName:  row.Server,
			},
			RuntimeName: row.RuntimeName,
		}
		if err := json.Unmarshal([]byte(row.EnvJson), &record.Env); err != nil {
			return nil, fmt.Errorf("store: decode MCP override env: %w", err)
		}
		if err := json.Unmarshal([]byte(row.HeadersJson), &record.Headers); err != nil {
			return nil, fmt.Errorf("store: decode MCP override headers: %w", err)
		}
		record.URL = row.Url
		record.UpdatedAt, err = store.ParseTimestamp(row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func extensionMCPProfileID(profileID string) string {
	if value := strings.TrimSpace(profileID); value != "" {
		return value
	}
	return store.DefaultProfileID
}

func normalizeExtensionMCPTarget(target extensionmcp.Target) (extensionmcp.Target, error) {
	target.Extension = strings.TrimSpace(target.Extension)
	target.ProfileID = extensionMCPProfileID(target.ProfileID)
	target.WorkspaceID = strings.TrimSpace(target.WorkspaceID)
	target.ServerName = strings.TrimSpace(target.ServerName)
	if err := vault.ValidateMCPOwner("extension:" + target.Extension); err != nil {
		return target, err
	}
	if err := compozyconfig.ValidateMCPServerName(target.ServerName); err != nil {
		return target, err
	}
	return target, nil
}

func (r *ExtensionMCPRepo) ListAll(ctx context.Context) ([]extensionmcp.Record, error) {
	if err := r.checkReady(ctx, "list allocated extension MCP names"); err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAllExtensionMCPOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list allocated extension MCP names: %w", err)
	}
	return decodeExtensionMCPRecords(rows)
}

func (r *ExtensionMCPRepo) DeleteWorkspace(ctx context.Context, extension, workspaceID string) error {
	if err := r.checkReady(ctx, "retire extension MCP allocations"); err != nil {
		return err
	}
	if strings.TrimSpace(extension) == "" {
		return errors.New("store: extension name is required")
	}
	return r.queries.DeleteExtensionMCPWorkspace(ctx, sqlcgen.DeleteExtensionMCPWorkspaceParams{
		Extension: strings.TrimSpace(extension), WorkspaceID: strings.TrimSpace(workspaceID),
	})
}

// DeleteTargets compensates only allocations introduced by a failed lifecycle mutation.
func (r *ExtensionMCPRepo) DeleteTargets(ctx context.Context, targets []extensionmcp.Target) error {
	if err := r.checkReady(ctx, "compensate extension MCP allocations"); err != nil {
		return err
	}
	return store.ExecuteWrite(ctx, r.db, func(ctx context.Context, tx *store.WriteTx) error {
		return deleteExtensionMCPTargets(ctx, sqlcgen.New(tx), targets)
	})
}

// RetireTargets commits exact allocation retirement and its lifecycle event together.
func (r *ExtensionMCPRepo) RetireTargets(
	ctx context.Context,
	targets []extensionmcp.Target,
	summary store.EventSummary,
) error {
	if err := r.checkReady(ctx, "commit scoped extension MCP retirement"); err != nil {
		return err
	}
	observe := &ObserveRepo{repoBase: r.repoBase}
	if err := observe.prepareEventSummary(ctx, &summary); err != nil {
		return err
	}
	return store.ExecuteWrite(ctx, r.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		if err := deleteExtensionMCPTargets(ctx, queries, targets); err != nil {
			return err
		}
		return insertEventSummary(ctx, queries, summary)
	})
}

func deleteExtensionMCPTargets(ctx context.Context, queries *sqlcgen.Queries, targets []extensionmcp.Target) error {
	for _, target := range targets {
		value, err := normalizeExtensionMCPTarget(target)
		if err != nil {
			return err
		}
		if err := queries.DeleteExtensionMCPAllocation(ctx, sqlcgen.DeleteExtensionMCPAllocationParams{
			Extension:   value.Extension,
			Profile:     value.ProfileID,
			WorkspaceID: value.WorkspaceID,
			Server:      value.ServerName,
		}); err != nil {
			return err
		}
	}
	return nil
}

// RetireWorkspace commits the unlink event and all profile allocations in the same transaction.
func (r *ExtensionMCPRepo) RetireWorkspace(
	ctx context.Context,
	extension, workspaceID string,
	summary store.EventSummary,
) error {
	if err := r.checkReady(ctx, "commit extension MCP retirement"); err != nil {
		return err
	}
	if strings.TrimSpace(extension) == "" {
		return errors.New("store: extension name is required")
	}
	observe := &ObserveRepo{repoBase: r.repoBase}
	if err := observe.prepareEventSummary(ctx, &summary); err != nil {
		return err
	}
	return store.ExecuteWrite(ctx, r.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		if err := queries.DeleteExtensionMCPWorkspace(ctx, sqlcgen.DeleteExtensionMCPWorkspaceParams{
			Extension: strings.TrimSpace(extension), WorkspaceID: strings.TrimSpace(workspaceID),
		}); err != nil {
			return err
		}
		return insertEventSummary(ctx, queries, summary)
	})
}
