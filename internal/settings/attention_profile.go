package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	profilepkg "github.com/compozy/compozy/internal/profile"
)

func (s *service) getAttentionSection(ctx context.Context, req SectionRequest) (SectionEnvelope, error) {
	s.attentionMu.RLock()
	defer s.attentionMu.RUnlock()

	scope, profileName, profileID, err := s.resolveAttentionProfile(ctx, req.Scope, req.ProfileName)
	if err != nil {
		return SectionEnvelope{}, fmt.Errorf("settings: get attention profile: %w", err)
	}
	cfg, _, err := s.loadConfig(ctx, ScopeUser, "", "")
	if err != nil {
		return SectionEnvelope{}, fmt.Errorf("settings: load attention config: %w", err)
	}
	mutedWorkspaces, err := s.attentionWorkspaceMutes.ListAttentionWorkspaceMutes(ctx, profileID)
	if err != nil {
		return SectionEnvelope{}, fmt.Errorf("settings: load attention workspace mutes: %w", err)
	}
	section := buildAttentionSection(&cfg, mutedWorkspaces)
	return SectionEnvelope{
		Section:         SectionAttention,
		Scope:           scope,
		ProfileName:     profileName,
		AvailableScopes: []ScopeKind{ScopeUser, ScopeProfile},
		Attention:       &section,
	}, nil
}

func (s *service) resolveAttentionProfile(
	ctx context.Context,
	scope ScopeKind,
	profileName string,
) (ScopeKind, string, string, error) {
	if ctx == nil {
		return "", "", "", errors.New("settings: attention profile context is required")
	}
	if scope == "" {
		scope = ScopeUser
	}
	profileName = strings.TrimSpace(profileName)
	switch scope {
	case ScopeUser:
		if profileName == "" {
			profileName = compozyconfig.DefaultProfileDirName
		}
		if profileName != compozyconfig.DefaultProfileDirName {
			return "", "", "", conflictError(
				fmt.Errorf("settings: user attention scope selects profile %q", compozyconfig.DefaultProfileDirName),
			)
		}
	case ScopeProfile:
		if profileName == "" || profileName == compozyconfig.DefaultProfileDirName {
			return "", "", "", validationError(
				errors.New("settings: a non-default profile is required for profile attention scope"),
			)
		}
	default:
		return "", "", "", conflictError(
			fmt.Errorf("settings: attention does not support %q scope", scope),
		)
	}
	profileID, err := s.profileResolver.AvailableProfileID(ctx, profileName)
	if err != nil {
		switch {
		case errors.Is(err, profilepkg.ErrNotFound):
			return "", "", "", notFoundError(err)
		case errors.Is(err, profilepkg.ErrArchived), errors.Is(err, profilepkg.ErrUnavailable):
			return "", "", "", conflictError(err)
		default:
			return "", "", "", fmt.Errorf("settings: resolve attention profile %q: %w", profileName, err)
		}
	}
	return scope, profileName, profileID, nil
}
