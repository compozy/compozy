package extensionpkg

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// ErrExtensionGatewayConfirmationRequired reports stale consent; GatewayConfirmationRequiredError carries its digest.
var ErrExtensionGatewayConfirmationRequired = errors.New("extension: gateway permissions confirmation required")

// GatewayConfirmation is the confirmation tuple for one exact extension instance.
type GatewayConfirmation struct {
	Digest      string
	ConfirmedBy string
	ConfirmedAt time.Time
}

// GatewayConfirmationRequiredError carries the current digest the caller must confirm.
type GatewayConfirmationRequiredError struct {
	CurrentDigest string
}

func (e *GatewayConfirmationRequiredError) Error() string {
	if e == nil {
		return ErrExtensionGatewayConfirmationRequired.Error()
	}
	return fmt.Sprintf("%s: %s", ErrExtensionGatewayConfirmationRequired, strings.TrimSpace(e.CurrentDigest))
}

func (e *GatewayConfirmationRequiredError) Unwrap() error {
	return ErrExtensionGatewayConfirmationRequired
}

// GatewayConfirmation returns valid state only while the persisted tuple matches the current artifact.
func (r *Registry) GatewayConfirmation(key InstanceKey) (GatewayConfirmation, error) {
	if err := r.checkReady("read extension gateway confirmation"); err != nil {
		return GatewayConfirmation{}, err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return GatewayConfirmation{}, err
	}
	stored, err := r.storedGatewayConfirmation(key)
	if err != nil {
		return GatewayConfirmation{}, err
	}
	current, err := r.currentGatewayRequirementDigest(key)
	if err != nil {
		return GatewayConfirmation{}, err
	}
	if stored.Digest != current {
		return GatewayConfirmation{Digest: current}, nil
	}
	return stored, nil
}

// ConfirmGatewayRequirement records actor attribution only for the digest currently on disk.
func (r *Registry) ConfirmGatewayRequirement(
	key InstanceKey,
	expectedDigest string,
	actor string,
	at time.Time,
) error {
	if err := r.checkReady("confirm extension gateway requirement"); err != nil {
		return err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	current, err := r.currentGatewayRequirementDigest(key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(expectedDigest) != current {
		return &GatewayConfirmationRequiredError{CurrentDigest: current}
	}
	if current == "" {
		return nil
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return errors.New("extension: gateway confirmation actor is required")
	}
	if at.IsZero() {
		return errors.New("extension: gateway confirmation time is required")
	}
	timestamp := store.FormatTimestamp(at.UTC())
	var result sql.Result
	if key.IsGlobal() {
		result, err = r.db.ExecContext(registryContext(), `
			UPDATE extensions
			SET gateway_requirement_digest = ?, gateway_confirmed_by = ?, gateway_confirmed_at = ?
			WHERE name = ?
		`, current, actor, timestamp, key.Name)
	} else {
		result, err = r.db.ExecContext(registryContext(), `
			UPDATE extension_dev_links
			SET gateway_requirement_digest = ?, gateway_confirmed_by = ?, gateway_confirmed_at = ?
			WHERE extension_name = ? AND workspace_id = ?
		`, current, actor, timestamp, key.Name, key.WorkspaceID)
	}
	if err != nil {
		return fmt.Errorf("extension: persist gateway confirmation for %q: %w", key.runtimeID(), err)
	}
	return inspectGatewayConfirmationUpdate(result, key, "gateway confirmation")
}

// ConfirmDevelopmentGatewayCandidate records consent for one inspected dev
// generation before its process is allowed to start. The caller must hold the
// extension lifecycle lock and must have derived digest from that immutable
// generation.
func (r *Registry) ConfirmDevelopmentGatewayCandidate(
	key InstanceKey,
	digest string,
	actor string,
	at time.Time,
) error {
	if err := r.checkReady("confirm development extension gateway candidate"); err != nil {
		return err
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	if key.IsGlobal() {
		return errors.New("extension: development gateway candidate requires a workspace instance")
	}
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return errors.New("extension: development gateway candidate digest is required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return errors.New("extension: gateway confirmation actor is required")
	}
	if at.IsZero() {
		return errors.New("extension: gateway confirmation time is required")
	}
	result, err := r.db.ExecContext(registryContext(), `
		UPDATE extension_dev_links
		SET gateway_requirement_digest = ?, gateway_confirmed_by = ?, gateway_confirmed_at = ?
		WHERE extension_name = ? AND workspace_id = ?
	`, digest, actor, store.FormatTimestamp(at.UTC()), key.Name, key.WorkspaceID)
	if err != nil {
		return fmt.Errorf("extension: persist development gateway candidate for %q: %w", key.runtimeID(), err)
	}
	return inspectGatewayConfirmationUpdate(result, key, "development gateway candidate")
}

// RestoreGatewayConfirmation restores a previously snapshotted confirmation
// tuple during lifecycle rollback. The caller must hold the lifecycle authority
// for the extension name.
func (r *Registry) RestoreGatewayConfirmation(key InstanceKey, confirmation GatewayConfirmation) error {
	if err := r.checkReady("restore extension gateway confirmation"); err != nil {
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
			SET gateway_requirement_digest = ?, gateway_confirmed_by = ?, gateway_confirmed_at = ?
			WHERE name = ?
		`, strings.TrimSpace(confirmation.Digest), nullableRegistryString(confirmation.ConfirmedBy),
			nullableRegistryTime(confirmation.ConfirmedAt), key.Name)
	} else {
		result, err = r.db.ExecContext(registryContext(), `
			UPDATE extension_dev_links
			SET gateway_requirement_digest = ?, gateway_confirmed_by = ?, gateway_confirmed_at = ?
			WHERE extension_name = ? AND workspace_id = ?
		`, strings.TrimSpace(confirmation.Digest), nullableRegistryString(confirmation.ConfirmedBy),
			nullableRegistryTime(confirmation.ConfirmedAt), key.Name, key.WorkspaceID)
	}
	if err != nil {
		return fmt.Errorf("extension: restore gateway confirmation for %q: %w", key.runtimeID(), err)
	}
	return inspectGatewayConfirmationUpdate(result, key, "gateway confirmation restoration")
}

func (r *Registry) storedGatewayConfirmation(key InstanceKey) (GatewayConfirmation, error) {
	var digest string
	var actor sql.NullString
	var timestamp sql.NullString
	var err error
	if key.IsGlobal() {
		err = r.db.QueryRowContext(registryContext(), `
			SELECT gateway_requirement_digest, gateway_confirmed_by, gateway_confirmed_at
			FROM extensions WHERE name = ?
		`, key.Name).Scan(&digest, &actor, &timestamp)
	} else {
		err = r.db.QueryRowContext(registryContext(), `
			SELECT gateway_requirement_digest, gateway_confirmed_by, gateway_confirmed_at
			FROM extension_dev_links WHERE extension_name = ? AND workspace_id = ?
		`, key.Name, key.WorkspaceID).Scan(&digest, &actor, &timestamp)
	}
	if errors.Is(err, sql.ErrNoRows) {
		if key.IsGlobal() {
			return GatewayConfirmation{}, &ExtensionNotFoundError{Name: key.Name}
		}
		return GatewayConfirmation{}, fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	if err != nil {
		return GatewayConfirmation{}, fmt.Errorf(
			"extension: read gateway confirmation for %q: %w",
			key.runtimeID(),
			err,
		)
	}
	confirmation := GatewayConfirmation{Digest: strings.TrimSpace(digest), ConfirmedBy: strings.TrimSpace(actor.String)}
	if timestamp.Valid {
		confirmation.ConfirmedAt, err = store.ParseTimestamp(timestamp.String)
		if err != nil {
			return GatewayConfirmation{}, fmt.Errorf(
				"extension: parse gateway confirmation for %q: %w",
				key.runtimeID(),
				err,
			)
		}
	}
	return confirmation, nil
}

func (r *Registry) currentGatewayRequirementDigest(key InstanceKey) (string, error) {
	var manifest *Manifest
	if key.IsGlobal() {
		info, err := r.Get(key.Name)
		if err != nil {
			return "", err
		}
		installDir, err := InstalledExtensionDir(info)
		if err != nil {
			return "", err
		}
		manifest, err = LoadManifest(installDir)
		if err != nil {
			return "", err
		}
	} else {
		link, err := r.GetDevLink(key.Name, key.WorkspaceID)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(link.GatewayRequirementDigest), nil
	}
	return GatewayRequirementDigest(manifest.Gateway)
}

func inspectGatewayConfirmationUpdate(result sql.Result, key InstanceKey, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("extension: inspect %s for %q: %w", operation, key.runtimeID(), err)
	}
	if affected != 0 {
		return nil
	}
	if key.IsGlobal() {
		return &ExtensionNotFoundError{Name: key.Name}
	}
	return fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
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
