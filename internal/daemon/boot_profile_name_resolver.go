package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"

	profilepkg "github.com/compozy/compozy/internal/profile"
)

type bootProfileNameResolver struct {
	state *bootState
}

type bootProfileCatalog struct {
	state *bootState
}

func (c bootProfileCatalog) List(ctx context.Context) ([]profilepkg.WithCounts, error) {
	if c.state == nil || c.state.profiles == nil {
		return nil, nil
	}
	return c.state.profiles.List(ctx)
}

func (r bootProfileNameResolver) ProfileName(ctx context.Context, profileID string) (string, error) {
	if r.state == nil || r.state.profiles == nil {
		return "", errors.New("daemon: profile manager is not booted")
	}
	return r.state.profiles.ProfileName(ctx, profileID)
}

// resolveRuntimeProfileWorkspace resolves resources using the durable work owner.
func resolveRuntimeProfileWorkspace(
	ctx context.Context,
	workspaces workspacepkg.RuntimeResolver,
	profiles session.ProfileNameResolver,
	ref, profileID string,
) (workspacepkg.ResolvedWorkspace, error) {
	if strings.TrimSpace(profileID) == "" {
		return workspaces.Resolve(ctx, ref)
	}
	name, err := resolvePromptSkillsProfileName(ctx, profiles, profileID)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	resolver, ok := workspaces.(workspacepkg.ProfileRuntimeResolver)
	if !ok {
		return workspacepkg.ResolvedWorkspace{}, errors.New(
			"daemon: runtime workspace resolver does not support profile layers",
		)
	}
	return resolver.ResolveForProfile(ctx, ref, name)
}
