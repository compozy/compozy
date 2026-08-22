package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

func (m *Manager) Create(ctx context.Context, in CreateInput) (Profile, error) {
	name, err := normalizeName(in.Name)
	if err != nil {
		return Profile{}, err
	}
	color, icon, emoji, err := normalizeIdentity(in.Color, in.Icon, in.Emoji)
	if err != nil {
		return Profile{}, err
	}
	if in.Activate != nil {
		if err := in.Activate.Validate(); err != nil {
			return Profile{}, err
		}
	}
	return m.create(ctx, name, color, icon, emoji, in.Activate, nil)
}

func (m *Manager) CreateDeclared(ctx context.Context, in DeclaredInput) (Profile, error) {
	extension := strings.TrimSpace(in.Extension)
	if extension == "" {
		return Profile{}, fmt.Errorf("%w: declaring extension is required", ErrInvalidInput)
	}
	name, err := normalizeName(in.Name)
	if err != nil {
		return Profile{}, err
	}
	color, icon, emoji, err := normalizeIdentity(in.Seed.Color, in.Seed.Icon, in.Seed.Emoji)
	if err != nil {
		return Profile{}, err
	}
	in.Seed.Color, in.Seed.Icon, in.Seed.Emoji = color, icon, emoji
	return m.create(ctx, name, color, icon, emoji, nil, &declaredCreation{
		extension: extension,
		seed:      in.Seed,
	})
}

type declaredCreation struct {
	extension string
	seed      DeclaredSeed
}

// HasDeclaredMarker reports whether one installed extension has already bound
// or created the named profile during its current installation lifetime.
func (m *Manager) HasDeclaredMarker(ctx context.Context, extension, name string) (bool, error) {
	if ctx == nil {
		return false, errors.New("profile: declared marker context is required")
	}
	var exists bool
	err := m.store.DB().QueryRowContext(
		ctx,
		`SELECT EXISTS(
			SELECT 1 FROM extension_profile_markers
			WHERE extension_name = ? AND profile_name = ?
		)`,
		strings.TrimSpace(extension),
		strings.TrimSpace(name),
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("profile: inspect declared profile marker: %w", err)
	}
	return exists, nil
}

func (m *Manager) create(
	ctx context.Context,
	name, color, icon, emoji string,
	activate *Lens,
	declared *declaredCreation,
) (Profile, error) {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	if declared != nil {
		marked, markerProfile, err := m.declaredMarkerProfile(ctx, declared.extension, name)
		if err != nil {
			return Profile{}, err
		}
		if marked {
			return markerProfile, nil
		}
		existing, err := getProfileByName(ctx, m.store.DB(), name)
		if err == nil {
			if err := m.markDeclaredBinding(ctx, declared.extension, name, existing.ID); err != nil {
				return Profile{}, err
			}
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Profile{}, err
		}
	}
	profileID, err := m.newProfileID()
	if err != nil {
		return Profile{}, err
	}
	opID, err := m.newOperationID()
	if err != nil {
		return Profile{}, err
	}
	revision, err := fingerprint(struct {
		Name, Color, Icon, Emoji string
		Declared                 bool
	}{name, color, icon, emoji, declared != nil})
	if err != nil {
		return Profile{}, err
	}
	createdAt := m.now().UTC()
	created := Profile{
		ID: profileID, Name: name, Color: color, Icon: icon, Emoji: emoji,
		State: StateActive, CreatedAt: createdAt,
	}
	err = m.write(ctx, "create profile", func(exec globaldb.ProfileWriteExecutor) error {
		if err := ensureNameAvailable(ctx, exec, name, ""); err != nil {
			return err
		}
		_, err := exec.ExecContext(
			ctx,
			`INSERT INTO profiles (id, name, color, icon, emoji, state, created_at, archived_at)
			 VALUES (?, ?, ?, ?, ?, 'active', ?, NULL)`,
			profileID, name, color, nullableString(icon), nullableString(emoji), formatTimestamp(createdAt),
		)
		if err != nil {
			return mapNameConstraint(err, name)
		}
		steps := []lifecycleStep{{Seq: 0, Action: "mkdir_profile", PathNew: m.profileDir(name)}}
		if declared != nil {
			steps = append(steps, lifecycleStep{Seq: 1, Action: "write_declared_seed", PathNew: m.profileDir(name)})
		}
		if err := m.insertOperation(
			ctx, exec, opID, "create", profileID, "", name, revision,
			steps,
		); err != nil {
			return err
		}
		if declared != nil {
			if err := m.persistDeclaredSeed(ctx, exec, opID, profileID, name, *declared); err != nil {
				return err
			}
		}
		if activate != nil {
			_, err := exec.ExecContext(
				ctx,
				`INSERT INTO profile_selections (lens, workspace_id, profile_id, updated_at)
				 VALUES (?, ?, ?, ?)
				 ON CONFLICT(lens, workspace_id) DO UPDATE SET
				 profile_id = excluded.profile_id, updated_at = excluded.updated_at`,
				activate.Kind, strings.TrimSpace(activate.WorkspaceID), profileID, formatTimestamp(createdAt),
			)
			if err != nil {
				return fmt.Errorf("profile: activate created profile %q: %w", name, err)
			}
		}
		return nil
	})
	if err != nil {
		return Profile{}, err
	}
	if err := m.finalizeOperation(context.WithoutCancel(ctx), opID, false); err != nil {
		return Profile{}, err
	}
	m.recordEvent("profile.created", created, opID)
	if activate != nil {
		m.recordEvent("profile.selection_changed", created, opID)
	}
	return created, nil
}

func (m *Manager) declaredMarkerProfile(
	ctx context.Context,
	extension, name string,
) (bool, Profile, error) {
	var profileID string
	err := m.store.DB().QueryRowContext(
		ctx,
		`SELECT created_profile_id FROM extension_profile_markers
		 WHERE extension_name = ? AND profile_name = ?`,
		extension,
		name,
	).Scan(&profileID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, Profile{}, nil
	}
	if err != nil {
		return false, Profile{}, fmt.Errorf("profile: read declared profile marker: %w", err)
	}
	current, currentErr := getProfileByName(ctx, m.store.DB(), name)
	if currentErr == nil {
		return true, current, nil
	}
	if !errors.Is(currentErr, ErrNotFound) {
		return false, Profile{}, currentErr
	}
	return true, Profile{ID: profileID, Name: name}, nil
}

func (m *Manager) persistDeclaredSeed(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	opID, profileID, name string,
	declared declaredCreation,
) error {
	encoded, err := json.Marshal(declared.seed)
	if err != nil {
		return fmt.Errorf("profile: encode declared seed: %w", err)
	}
	digest, err := fingerprint(json.RawMessage(encoded))
	if err != nil {
		return err
	}
	if _, err := exec.ExecContext(
		ctx,
		`INSERT INTO profile_lifecycle_op_seed
		 (op_id, color, icon, emoji, default_agent, default_provider, default_sandbox, declaration_digest)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		opID,
		declared.seed.Color,
		nullableString(declared.seed.Icon),
		nullableString(declared.seed.Emoji),
		nullableString(declared.seed.Defaults.Agent),
		nullableString(declared.seed.Defaults.Provider),
		nullableString(declared.seed.Defaults.Sandbox),
		digest,
	); err != nil {
		return fmt.Errorf("profile: persist declared seed: %w", err)
	}
	for _, ask := range declared.seed.CredentialAsks {
		provider, slot := strings.TrimSpace(ask.Provider), strings.TrimSpace(ask.Slot)
		if provider == "" || slot == "" {
			return fmt.Errorf("%w: declared credential provider and slot are required", ErrInvalidInput)
		}
		if _, err := exec.ExecContext(
			ctx,
			`INSERT INTO profile_lifecycle_op_credential_asks (op_id, provider, slot) VALUES (?, ?, ?)`,
			opID, provider, slot,
		); err != nil {
			return fmt.Errorf("profile: persist declared credential ask: %w", err)
		}
		if _, err := exec.ExecContext(
			ctx,
			`INSERT INTO profile_credential_requirements
			 (profile_id, provider, slot, source_extension, declaration_digest, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			profileID, provider, slot, declared.extension, digest, formatTimestamp(m.now()),
		); err != nil {
			return fmt.Errorf("profile: persist credential requirement: %w", err)
		}
	}
	if _, err := exec.ExecContext(
		ctx,
		`INSERT INTO extension_profile_markers
		 (extension_name, profile_name, created_profile_id, created_at) VALUES (?, ?, ?, ?)`,
		declared.extension, name, profileID, formatTimestamp(m.now()),
	); err != nil {
		return fmt.Errorf("profile: persist declared profile marker: %w", err)
	}
	return nil
}

func (m *Manager) markDeclaredBinding(ctx context.Context, extension, name, profileID string) error {
	return m.write(ctx, "mark declared profile binding", func(exec globaldb.ProfileWriteExecutor) error {
		_, err := exec.ExecContext(
			ctx,
			`INSERT INTO extension_profile_markers
			 (extension_name, profile_name, created_profile_id, created_at)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(extension_name, profile_name) DO NOTHING`,
			extension, name, profileID, formatTimestamp(m.now()),
		)
		if err != nil {
			return fmt.Errorf("profile: persist declared profile binding: %w", err)
		}
		return nil
	})
}
