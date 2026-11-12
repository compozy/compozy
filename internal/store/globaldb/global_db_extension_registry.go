package globaldb

import (
	"context"
	"database/sql"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ExtensionRegistryRepo maps extension queries without exposing generated types.
type ExtensionRegistryRepo struct{ queries *sqlcgen.Queries }

// NewExtensionRegistryRepo shares the registry's existing database handle.
func NewExtensionRegistryRepo(db *sql.DB) *ExtensionRegistryRepo {
	return &ExtensionRegistryRepo{queries: sqlcgen.New(db)}
}

// ExtensionBindingScope identifies one persisted credential cell, including inactive bindings.
type ExtensionBindingScope struct {
	ProfileID   string
	WorkspaceID string
}

// EnvBindingScopes returns all credential owners in deterministic order for orphan retirement.
func (r *ExtensionRegistryRepo) EnvBindingScopes(ctx context.Context, name string) ([]ExtensionBindingScope, error) {
	rows, err := r.queries.ListExtensionEnvBindingScopes(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]ExtensionBindingScope, 0, len(rows))
	for _, row := range rows {
		result = append(result, ExtensionBindingScope{ProfileID: row.ProfileID, WorkspaceID: row.WorkspaceID})
	}
	return result, nil
}

// ExtensionToolVisibility captures current installation reach and profile enablement.
type ExtensionToolVisibility struct {
	Enabled                 bool
	Installed               bool
	InstallationWorkspaceID string
	DevLinked               bool
	DevGeneration           string
}

// ToolVisibility reads fresh authority for one tool-catalog population in a single statement.
func (r *ExtensionRegistryRepo) ToolVisibility(
	ctx context.Context, profileID, workspaceID string,
) (map[string]ExtensionToolVisibility, error) {
	rows, err := r.queries.ListExtensionToolVisibility(ctx, sqlcgen.ListExtensionToolVisibilityParams{
		ProfileID: profileID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]ExtensionToolVisibility, len(rows))
	for _, row := range rows {
		result[row.Name] = ExtensionToolVisibility{
			Enabled: row.Enabled, Installed: row.Installed,
			InstallationWorkspaceID: row.InstallationWorkspaceID,
			DevLinked:               row.DevLinked, DevGeneration: row.DevGeneration,
		}
	}
	return result, nil
}

// WorkspaceInstallations returns attached names across profiles for workspace inventory.
func (r *ExtensionRegistryRepo) WorkspaceInstallations(ctx context.Context, workspaceID string) ([]string, error) {
	return r.queries.ListExtensionWorkspaceInstallations(ctx, workspaceID)
}
