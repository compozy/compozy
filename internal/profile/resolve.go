package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (m *Manager) Resolve(ctx context.Context, in ResolveInput) (Resolution, error) {
	if ctx == nil {
		return Resolution{}, errors.New("profile: resolve context is required")
	}
	if err := in.Lens.Validate(); err != nil {
		return Resolution{}, err
	}
	flag := strings.TrimSpace(in.Flag)
	env := strings.TrimSpace(in.Env)
	sessionProfileID := strings.TrimSpace(in.SessionProfileID)
	if sessionProfileID != "" {
		profile, err := getProfileByID(ctx, m.store.DB(), sessionProfileID)
		if err != nil {
			return Resolution{}, err
		}
		if (flag != "" && flag != profile.Name) || (env != "" && env != profile.Name) {
			return Resolution{}, domainError(
				"profile_session_conflict",
				fmt.Sprintf("session is bound to profile %q", profile.Name),
				"drop the profile flag or environment override; the session decides",
				ErrSessionConflict,
			)
		}
		if err := ensureAvailable(ctx, m.store.DB(), profile, true); err != nil {
			return Resolution{}, err
		}
		return Resolution{Profile: profile, Source: ResolutionSourceSession}, nil
	}
	if flag != "" {
		return m.resolveExplicit(ctx, flag, ResolutionSourceFlag)
	}
	if env != "" {
		return m.resolveExplicit(ctx, env, ResolutionSourceEnv)
	}
	selection, found, err := m.selections.Get(ctx, in.Lens.Kind, in.Lens.WorkspaceID)
	if err != nil {
		return Resolution{}, err
	}
	if !found {
		return m.resolveDefault(ctx, ResolutionNoteNoRememberedChoice)
	}
	remembered, err := getProfileByID(ctx, m.store.DB(), selection.ProfileID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return m.resolveDefault(ctx, ResolutionNoteNoRememberedChoice)
		}
		return Resolution{}, err
	}
	if remembered.State == StateArchived {
		return m.resolveDefault(ctx, ResolutionNoteArchivedRememberedFallback)
	}
	if err := ensureAvailable(ctx, m.store.DB(), remembered, false); err != nil {
		return Resolution{}, err
	}
	return Resolution{Profile: remembered, Source: ResolutionSourceRemembered}, nil
}

func (m *Manager) resolveExplicit(
	ctx context.Context,
	name string,
	source ResolutionSource,
) (Resolution, error) {
	profile, err := getProfileByName(ctx, m.store.DB(), name)
	if err != nil {
		return Resolution{}, err
	}
	if err := ensureAvailable(ctx, m.store.DB(), profile, true); err != nil {
		return Resolution{}, err
	}
	return Resolution{Profile: profile, Source: source}, nil
}

func (m *Manager) resolveDefault(ctx context.Context, note ResolutionNote) (Resolution, error) {
	profile, err := getProfileByID(ctx, m.store.DB(), defaultProfileID)
	if err != nil {
		return Resolution{}, err
	}
	if err := ensureAvailable(ctx, m.store.DB(), profile, true); err != nil {
		return Resolution{}, err
	}
	return Resolution{Profile: profile, Source: ResolutionSourceDefault, Note: note}, nil
}

func ensureAvailable(ctx context.Context, q queryer, profile Profile, rejectArchived bool) error {
	if rejectArchived && profile.State == StateArchived {
		return domainError(
			"profile_archived",
			fmt.Sprintf("profile %q is archived", profile.Name),
			"run compozy profile unarchive "+profile.Name,
			ErrArchived,
		)
	}
	var operationID string
	err := q.QueryRowContext(
		ctx,
		`SELECT id FROM profile_lifecycle_ops WHERE profile_id = ? AND status <> 'done' ORDER BY created_at LIMIT 1`,
		profile.ID,
	).Scan(&operationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("profile: check availability for %q: %w", profile.Name, err)
	}
	return domainError(
		"profile_unavailable",
		fmt.Sprintf("profile %q is reserved by lifecycle operation %s", profile.Name, operationID),
		"run compozy profile ops and retry the operation",
		ErrUnavailable,
	)
}
