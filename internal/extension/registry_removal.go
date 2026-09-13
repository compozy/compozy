package extensionpkg

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// RemovalState retains profile-owned records deleted by extension removal cascades.
type RemovalState struct {
	installations []Installation
	enablement    map[string]bool
	markers       []removalProfileMarker
}

type removalProfileMarker struct {
	name, profileID, createdAt string
}

// SnapshotRemovalState captures dependent profile records before uninstall.
func (r *Registry) SnapshotRemovalState(ctx context.Context, name string) (RemovalState, error) {
	if err := r.checkReady("snapshot extension removal"); err != nil {
		return RemovalState{}, err
	}
	name, err := normalizeExtensionName(name)
	if err != nil {
		return RemovalState{}, err
	}
	enablement, err := r.removalEnablement(ctx, name)
	if err != nil {
		return RemovalState{}, err
	}
	markers, err := r.removalProfileMarkers(ctx, name)
	if err != nil {
		return RemovalState{}, err
	}
	installations, err := r.Installations(ctx, name)
	if err != nil {
		return RemovalState{}, err
	}
	return RemovalState{enablement: enablement, markers: markers, installations: installations}, nil
}

func (r *Registry) removalEnablement(ctx context.Context, name string) (map[string]bool, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT profile_id, enabled FROM extension_profile_enablement WHERE extension_name = ?`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var profileID string
		var enabled bool
		if err := rows.Scan(&profileID, &enabled); err != nil {
			return nil, err
		}
		result[profileID] = enabled
	}
	return result, rows.Err()
}

func (r *Registry) removalProfileMarkers(ctx context.Context, name string) ([]removalProfileMarker, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT profile_name, created_profile_id, created_at FROM extension_profile_markers WHERE extension_name = ? ORDER BY profile_name`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []removalProfileMarker
	for rows.Next() {
		var marker removalProfileMarker
		if err := rows.Scan(&marker.name, &marker.profileID, &marker.createdAt); err != nil {
			return nil, err
		}
		result = append(result, marker)
	}
	return result, rows.Err()
}

// RestoreRemovalState restores profile records atomically before runtime publication resumes.
func (r *Registry) RestoreRemovalState(ctx context.Context, name string, snapshot RemovalState) error {
	if err := r.checkReady("restore extension removal state"); err != nil {
		return err
	}
	name, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	err = store.ExecuteWrite(ctx, r.db, func(ctx context.Context, tx *store.WriteTx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM extensions WHERE name = ?)`, name).
			Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return &ExtensionNotFoundError{Name: name}
		}
		if err := replaceInstallations(ctx, tx, name, snapshot.installations); err != nil {
			return err
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM extension_profile_enablement WHERE extension_name = ?`,
			name,
		); err != nil {
			return err
		}
		for profileID, enabled := range snapshot.enablement {
			// An independently removed Profile must not be resurrected by extension compensation.
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO extension_profile_enablement(extension_name, profile_id, enabled) SELECT ?, id, ? FROM profiles WHERE id = ?`,
				name,
				enabled,
				profileID,
			); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM extension_profile_markers WHERE extension_name = ?`,
			name,
		); err != nil {
			return err
		}
		for _, marker := range snapshot.markers {
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO extension_profile_markers(extension_name, profile_name, created_profile_id, created_at) VALUES (?, ?, ?, ?)`,
				name,
				marker.name,
				marker.profileID,
				marker.createdAt,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("extension: restore profile state for %q: %w", name, err)
	}
	r.invalidateEnabledBundledNames()
	return nil
}
