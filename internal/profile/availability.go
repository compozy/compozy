package profile

import (
	"context"
	"errors"
	"strings"
)

// EnsureAvailableName rejects profiles that cannot safely own new runtime work.
func (m *Manager) EnsureAvailableName(ctx context.Context, name string) error {
	if ctx == nil {
		return errors.New("profile: availability context is required")
	}
	profile, err := getProfileByName(ctx, m.store.DB(), strings.TrimSpace(name))
	if err != nil {
		return err
	}
	return ensureAvailable(ctx, m.store.DB(), profile, true)
}

// AvailableProfileID resolves an active profile name to its stable owner ID.
func (m *Manager) AvailableProfileID(ctx context.Context, name string) (string, error) {
	if ctx == nil {
		return "", errors.New("profile: availability context is required")
	}
	profile, err := getProfileByName(ctx, m.store.DB(), strings.TrimSpace(name))
	if err != nil {
		return "", err
	}
	if err := ensureAvailable(ctx, m.store.DB(), profile, true); err != nil {
		return "", err
	}
	return profile.ID, nil
}

// EnsureAvailableID rejects profiles that cannot safely own new runtime work.
func (m *Manager) EnsureAvailableID(ctx context.Context, profileID string) error {
	if ctx == nil {
		return errors.New("profile: availability context is required")
	}
	profile, err := getProfileByID(ctx, m.store.DB(), strings.TrimSpace(profileID))
	if err != nil {
		return err
	}
	return ensureAvailable(ctx, m.store.DB(), profile, true)
}
