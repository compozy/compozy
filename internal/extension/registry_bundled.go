package extensionpkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
)

// ReconcileBundledProvenance persists the canonical first-party trust evidence
// for an already installed bundled extension without changing its lifecycle state.
func (r *Registry) ReconcileBundledProvenance(manifest *Manifest) error {
	if err := r.checkReady("reconcile bundled extension provenance"); err != nil {
		return err
	}
	if manifest == nil {
		return errors.New("extension: bundled manifest is required")
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	info, err := r.Get(manifest.Name)
	if err != nil {
		return err
	}
	if info.Source != SourceBundled {
		return fmt.Errorf("extension: %q is not a bundled extension", info.Name)
	}

	provenance := extensionInstallProvenance(
		SourceBundled,
		info.ManifestPath,
		info.Checksum,
		extensionPermissions(manifest),
		info.InstalledAt,
	)
	provenanceJSON, err := json.Marshal(provenance)
	if err != nil {
		return fmt.Errorf("extension: marshal bundled provenance for %q: %w", info.Name, err)
	}
	currentJSON, err := json.Marshal(info.Provenance)
	if err != nil {
		return fmt.Errorf("extension: marshal current provenance for %q: %w", info.Name, err)
	}
	if bytes.Equal(currentJSON, provenanceJSON) {
		return nil
	}
	slog.Info(
		"reconciling bundled extension provenance",
		"extension", info.Name,
		"previous_installed_from", info.Provenance.InstalledFrom,
		"previous_registry_tier", info.Provenance.RegistryTier,
	)
	result, err := r.db.ExecContext(
		registryContext(),
		`UPDATE extensions SET provenance_json = ? WHERE name = ? AND source = ?`,
		string(provenanceJSON),
		info.Name,
		SourceBundled.String(),
	)
	if err != nil {
		return fmt.Errorf("extension: reconcile bundled provenance for %q: %w", info.Name, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("extension: inspect bundled provenance reconciliation for %q: %w", info.Name, err)
	}
	if affected > 1 {
		return fmt.Errorf("extension: reconcile bundled provenance for %q affected %d rows", info.Name, affected)
	}
	return nil
}
