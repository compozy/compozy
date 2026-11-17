package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type UserRuntimeResolver interface {
	ResolveUser(context.Context, string) (ResolvedWorkspace, error)
}

var _ UserRuntimeResolver = (*Resolver)(nil)

type profileNameResolver interface {
	ProfileName(context.Context, string) (string, error)
}

func (r *Resolver) ResolveUser(ctx context.Context, profileID string) (ResolvedWorkspace, error) {
	if err := checkContext(ctx); err != nil {
		return ResolvedWorkspace{}, err
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return ResolvedWorkspace{}, errors.New("workspace: User scope requires a profile id")
	}
	profiles, ok := r.profileAvailability.(profileNameResolver)
	if !ok {
		return ResolvedWorkspace{}, errors.New("workspace: profile name resolver is required for User scope")
	}
	profileName, err := profiles.ProfileName(ctx, profileID)
	if err != nil {
		return ResolvedWorkspace{}, fmt.Errorf("workspace: resolve User profile: %w", err)
	}
	cfg, err := r.loadProfileConfig("", profileName)
	if err != nil {
		return ResolvedWorkspace{}, fmt.Errorf("workspace: load User config: %w", err)
	}
	root, err := r.userWorkDir(cfg.UserScope.WorkDir)
	if err != nil {
		return ResolvedWorkspace{}, err
	}
	// Discovery has no workspace root even when the execution directory contains project configuration.
	scan, err := r.scanWorkspace(ctx, Workspace{}, profileName, &cfg.Skills)
	if err != nil {
		return ResolvedWorkspace{}, err
	}
	resolved, err := r.buildResolvedWorkspace(ctx, Workspace{RootDir: root}, scan, profileName, &cfg)
	if err != nil {
		return ResolvedWorkspace{}, err
	}
	resolved.Scope, resolved.ProfileID = ScopeUser, profileID
	return resolved, nil
}

func (r *Resolver) userWorkDir(value string) (string, error) {
	root := strings.TrimSpace(value)
	if root == "~" {
		root = r.operatorHomeDir
	} else if suffix, ok := strings.CutPrefix(root, "~/"); ok {
		root = filepath.Join(r.operatorHomeDir, suffix)
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("%w: work_dir must be absolute", ErrUserScopeWorkDirInvalid)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUserScopeWorkDirInvalid, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: work_dir must be a directory", ErrUserScopeWorkDirInvalid)
	}
	return filepath.Clean(root), nil
}
