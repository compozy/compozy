package extensionpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// InstallationScope selects an attachment; empty profile means all profiles, empty workspace means global.
type InstallationScope struct {
	ProfileID   string
	WorkspaceID string
}

func (s InstallationScope) normalize() InstallationScope {
	return InstallationScope{ProfileID: strings.TrimSpace(s.ProfileID), WorkspaceID: strings.TrimSpace(s.WorkspaceID)}
}

// Installation preserves one package attachment and its original creation time.
type Installation struct {
	Scope     InstallationScope
	CreatedAt time.Time
}

// WithInstallScope limits a new installation; replacements preserve attachments and enablement.
func WithInstallScope(scope InstallationScope) InstallOption {
	return func(config *installConfig) { config.scope = new(scope.normalize()) }
}

// Installations lists persisted attachments, including disabled installations.
func (r *Registry) Installations(ctx context.Context, name string) ([]Installation, error) {
	return r.listInstallations(ctx, name, false)
}

func (r *Registry) activeInstallations(ctx context.Context, name string) ([]Installation, error) {
	return r.listInstallations(ctx, name, true)
}

func (r *Registry) listInstallations(ctx context.Context, name string, activeOnly bool) ([]Installation, error) {
	if err := r.checkReady("list extension installations"); err != nil {
		return nil, err
	}
	name, err := normalizeExtensionName(name)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT profile_id, workspace_id, created_at
 FROM extension_installations AS installation WHERE extension_name = ?
 AND (NOT ? OR profile_id = '' OR EXISTS (
  SELECT 1 FROM profiles WHERE profiles.id = installation.profile_id AND state = 'active'
 )) ORDER BY workspace_id, profile_id`, name, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Installation, 0)
	for rows.Next() {
		var item Installation
		var createdAt string
		if err := rows.Scan(&item.Scope.ProfileID, &item.Scope.WorkspaceID, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt, err = store.ParseTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ResolveInstallation selects a workspace attachment before an inherited global
// attachment, then the exact profile before an all-profiles attachment.
func (r *Registry) ResolveInstallation(
	ctx context.Context,
	name string,
	scope InstallationScope,
) (Installation, error) {
	if err := r.checkReady("resolve extension installation"); err != nil {
		return Installation{}, err
	}
	name, err := normalizeExtensionName(name)
	if err != nil {
		return Installation{}, err
	}
	scope = scope.normalize()
	var result Installation
	var createdAt string
	err = r.db.QueryRowContext(ctx, `SELECT profile_id, workspace_id, created_at FROM extension_installations
 WHERE extension_name = ? AND profile_id IN ('', ?) AND workspace_id IN ('', ?)
 ORDER BY (workspace_id = ?) DESC, (profile_id = ?) DESC LIMIT 1`,
		name, scope.ProfileID, scope.WorkspaceID, scope.WorkspaceID, scope.ProfileID).
		Scan(&result.Scope.ProfileID, &result.Scope.WorkspaceID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Installation{}, &ExtensionNotFoundError{Name: name}
	}
	if err != nil {
		return Installation{}, err
	}
	result.CreatedAt, err = store.ParseTimestamp(createdAt)
	return result, err
}

// AttachInstallation adds one exact scope without changing the package or another
// attachment. Retrying the same attachment preserves its original timestamp.
func (r *Registry) AttachInstallation(ctx context.Context, name string, scope InstallationScope) error {
	if err := r.checkReady("attach extension installation"); err != nil {
		return err
	}
	name, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	scope = scope.normalize()
	return store.ExecuteWrite(ctx, r.db, func(ctx context.Context, tx *store.WriteTx) error {
		return insertInstallation(ctx, tx, name, Installation{Scope: scope, CreatedAt: r.now().UTC()})
	})
}

// DetachInstallation removes only the exact attachment. Package deletion belongs
// to the lifecycle coordinator after it retires the final installed instance.
func (r *Registry) DetachInstallation(ctx context.Context, name string, scope InstallationScope) error {
	if err := r.checkReady("detach extension installation"); err != nil {
		return err
	}
	name, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	scope = scope.normalize()
	return store.ExecuteWrite(ctx, r.db, func(ctx context.Context, tx *store.WriteTx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM extension_installations
  WHERE extension_name = ? AND profile_id = ? AND workspace_id = ?`, name, scope.ProfileID, scope.WorkspaceID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count == 0 {
			return &ExtensionNotFoundError{Name: name}
		}
		return nil
	})
}

func insertInstallation(ctx context.Context, tx *store.WriteTx, name string, item Installation) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO extension_installations
 (extension_name, profile_id, workspace_id, created_at) VALUES (?, ?, ?, ?)
 ON CONFLICT(extension_name, profile_id, workspace_id) DO NOTHING`,
		name, item.Scope.ProfileID, item.Scope.WorkspaceID, store.FormatTimestamp(item.CreatedAt))
	if err != nil {
		return fmt.Errorf("extension: attach installation for %q: %w", name, err)
	}
	return nil
}

func replaceInstallations(ctx context.Context, tx *store.WriteTx, name string, installations []Installation) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM extension_installations WHERE extension_name = ?`, name); err != nil {
		return err
	}
	for _, item := range installations {
		// Compensation cannot recreate a deleted workspace or profile.
		var ownerExists bool
		if err := tx.QueryRowContext(ctx, `SELECT
   (? = '' OR EXISTS (SELECT 1 FROM profiles WHERE id = ?)) AND
   (? = '' OR EXISTS (SELECT 1 FROM workspaces WHERE id = ?))`,
			item.Scope.ProfileID, item.Scope.ProfileID, item.Scope.WorkspaceID, item.Scope.WorkspaceID).
			Scan(&ownerExists); err != nil {
			return err
		}
		if !ownerExists {
			continue
		}
		if err := insertInstallation(ctx, tx, name, item); err != nil {
			return err
		}
	}
	return nil
}
