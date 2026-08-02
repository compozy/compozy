package extensionpkg

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

var ErrExtensionNetworkConfirmationRequired = errors.New("extension: network participation confirmation required")

// NetworkConfirmation is the confirmation tuple for one exact extension instance.
type NetworkConfirmation struct {
	Digest      string
	ConfirmedBy string
	ConfirmedAt time.Time
}

// NetworkConfirmationRequiredError carries the current digest the caller must confirm.
type NetworkConfirmationRequiredError struct {
	CurrentDigest string
}

func (e *NetworkConfirmationRequiredError) Error() string {
	if e == nil {
		return ErrExtensionNetworkConfirmationRequired.Error()
	}
	return fmt.Sprintf("%s: %s", ErrExtensionNetworkConfirmationRequired, strings.TrimSpace(e.CurrentDigest))
}

func (e *NetworkConfirmationRequiredError) Unwrap() error {
	return ErrExtensionNetworkConfirmationRequired
}

// NetworkConfirmation returns valid state only while the persisted tuple matches the current artifact.
func (r *Registry) NetworkConfirmation(key InstanceKey) (NetworkConfirmation, error) {
	if err := r.checkReady("read extension network confirmation"); err != nil {
		return NetworkConfirmation{}, err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return NetworkConfirmation{}, err
	}
	stored, err := r.storedNetworkConfirmation(key)
	if err != nil {
		return NetworkConfirmation{}, err
	}
	current, err := r.currentNetworkRequirementDigest(key)
	if err != nil {
		return NetworkConfirmation{}, err
	}
	if stored.Digest != current {
		return NetworkConfirmation{Digest: current}, nil
	}
	return stored, nil
}

// ConfirmNetworkRequirement records actor attribution only for the digest currently on disk.
func (r *Registry) ConfirmNetworkRequirement(
	key InstanceKey,
	expectedDigest string,
	actor string,
	at time.Time,
) error {
	if err := r.checkReady("confirm extension network requirement"); err != nil {
		return err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	current, err := r.currentNetworkRequirementDigest(key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(expectedDigest) != current {
		return &NetworkConfirmationRequiredError{CurrentDigest: current}
	}
	if current == "" {
		return nil
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return errors.New("extension: network confirmation actor is required")
	}
	if at.IsZero() {
		return errors.New("extension: network confirmation time is required")
	}
	timestamp := store.FormatTimestamp(at.UTC())
	var result sql.Result
	if key.IsGlobal() {
		result, err = r.db.ExecContext(registryContext(), `
			UPDATE extensions
			SET network_requirement_digest = ?, network_confirmed_by = ?, network_confirmed_at = ?
			WHERE name = ?
		`, current, actor, timestamp, key.Name)
	} else {
		result, err = r.db.ExecContext(registryContext(), `
			UPDATE extension_dev_links
			SET network_requirement_digest = ?, network_confirmed_by = ?, network_confirmed_at = ?
			WHERE extension_name = ? AND workspace_id = ?
		`, current, actor, timestamp, key.Name, key.WorkspaceID)
	}
	if err != nil {
		return fmt.Errorf("extension: persist network confirmation for %q: %w", key.runtimeID(), err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("extension: inspect network confirmation for %q: %w", key.runtimeID(), err)
	}
	if affected == 0 {
		if key.IsGlobal() {
			return &ExtensionNotFoundError{Name: key.Name}
		}
		return fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	return nil
}

// ConfirmDevelopmentNetworkCandidate records consent for one inspected dev
// generation before its process is allowed to start. The caller must hold the
// extension lifecycle lock and must have derived digest from that immutable
// generation.
func (r *Registry) ConfirmDevelopmentNetworkCandidate(
	key InstanceKey,
	digest string,
	actor string,
	at time.Time,
) error {
	if err := r.checkReady("confirm development extension network candidate"); err != nil {
		return err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	if key.IsGlobal() {
		return errors.New("extension: development network candidate requires a workspace instance")
	}
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return errors.New("extension: development network candidate digest is required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return errors.New("extension: network confirmation actor is required")
	}
	if at.IsZero() {
		return errors.New("extension: network confirmation time is required")
	}
	result, err := r.db.ExecContext(registryContext(), `
		UPDATE extension_dev_links
		SET network_requirement_digest = ?, network_confirmed_by = ?, network_confirmed_at = ?
		WHERE extension_name = ? AND workspace_id = ?
	`, digest, actor, store.FormatTimestamp(at.UTC()), key.Name, key.WorkspaceID)
	if err != nil {
		return fmt.Errorf("extension: persist development network candidate for %q: %w", key.runtimeID(), err)
	}
	if err := rowsAffectedNotFound(result, key.Name); err != nil {
		return fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	return nil
}

// RestoreNetworkConfirmation restores a previously snapshotted confirmation
// tuple during lifecycle rollback. The caller must hold the lifecycle authority
// for the extension name.
func (r *Registry) RestoreNetworkConfirmation(key InstanceKey, confirmation NetworkConfirmation) error {
	if err := r.checkReady("restore extension network confirmation"); err != nil {
		return err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	var result sql.Result
	var err error
	if key.IsGlobal() {
		result, err = r.db.ExecContext(registryContext(), `
			UPDATE extensions
			SET network_requirement_digest = ?, network_confirmed_by = ?, network_confirmed_at = ?
			WHERE name = ?
		`, strings.TrimSpace(confirmation.Digest), nullableRegistryString(confirmation.ConfirmedBy),
			nullableRegistryTime(confirmation.ConfirmedAt), key.Name)
	} else {
		result, err = r.db.ExecContext(registryContext(), `
			UPDATE extension_dev_links
			SET network_requirement_digest = ?, network_confirmed_by = ?, network_confirmed_at = ?
			WHERE extension_name = ? AND workspace_id = ?
		`, strings.TrimSpace(confirmation.Digest), nullableRegistryString(confirmation.ConfirmedBy),
			nullableRegistryTime(confirmation.ConfirmedAt), key.Name, key.WorkspaceID)
	}
	if err != nil {
		return fmt.Errorf("extension: restore network confirmation for %q: %w", key.runtimeID(), err)
	}
	if err := rowsAffectedNotFound(result, key.Name); err != nil {
		if key.IsGlobal() {
			return err
		}
		return fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	return nil
}

func (r *Registry) storedNetworkConfirmation(key InstanceKey) (NetworkConfirmation, error) {
	var digest string
	var actor sql.NullString
	var timestamp sql.NullString
	var err error
	if key.IsGlobal() {
		err = r.db.QueryRowContext(registryContext(), `
			SELECT network_requirement_digest, network_confirmed_by, network_confirmed_at
			FROM extensions WHERE name = ?
		`, key.Name).Scan(&digest, &actor, &timestamp)
	} else {
		err = r.db.QueryRowContext(registryContext(), `
			SELECT network_requirement_digest, network_confirmed_by, network_confirmed_at
			FROM extension_dev_links WHERE extension_name = ? AND workspace_id = ?
		`, key.Name, key.WorkspaceID).Scan(&digest, &actor, &timestamp)
	}
	if errors.Is(err, sql.ErrNoRows) {
		if key.IsGlobal() {
			return NetworkConfirmation{}, &ExtensionNotFoundError{Name: key.Name}
		}
		return NetworkConfirmation{}, fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	if err != nil {
		return NetworkConfirmation{}, fmt.Errorf(
			"extension: read network confirmation for %q: %w",
			key.runtimeID(),
			err,
		)
	}
	confirmation := NetworkConfirmation{Digest: strings.TrimSpace(digest), ConfirmedBy: strings.TrimSpace(actor.String)}
	if timestamp.Valid {
		confirmation.ConfirmedAt, err = store.ParseTimestamp(timestamp.String)
		if err != nil {
			return NetworkConfirmation{}, fmt.Errorf(
				"extension: parse network confirmation for %q: %w",
				key.runtimeID(),
				err,
			)
		}
	}
	return confirmation, nil
}

func (r *Registry) currentNetworkRequirementDigest(key InstanceKey) (string, error) {
	var manifest *Manifest
	if key.IsGlobal() {
		info, err := r.Get(key.Name)
		if err != nil {
			return "", err
		}
		manifest, err = LoadManifest(filepath.Dir(info.ManifestPath))
		if err != nil {
			return "", err
		}
	} else {
		link, err := r.GetDevLink(key.Name, key.WorkspaceID)
		if err != nil {
			return "", err
		}
		verified, err := verifyDevGeneration(link.OriginPath, link.BundleGeneration)
		if err != nil {
			return "", err
		}
		manifest = verified.Manifest
	}
	return NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
}

func nullableRegistryString(value string) any {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return nil
}

func nullableRegistryTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return store.FormatTimestamp(value.UTC())
}
