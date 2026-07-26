package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/skills"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func workspaceHookDeclarations(
	ctx context.Context,
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	logger *slog.Logger,
) ([]hookspkg.HookDecl, error) {
	workspaces, err := registeredWorkspaces(ctx, registry, workspaceResolver, logger)
	if err != nil {
		return nil, err
	}

	decls := make([]hookspkg.HookDecl, 0, len(workspaces))
	for idx := range workspaces {
		resolved := &workspaces[idx]
		workspaceDecls, err := aghconfig.HookDeclarations(resolved.Config.Hooks, resolved.Agents)
		if err != nil {
			return nil, fmt.Errorf("daemon: load hook declarations for workspace %q: %w", resolved.WorkspaceID, err)
		}
		decls = append(decls, scopeWorkspaceHookDecls(workspaceDecls, resolved)...)
	}

	return decls, nil
}

func registeredWorkspaces(
	ctx context.Context,
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	logger *slog.Logger,
) ([]workspacepkg.ResolvedWorkspace, error) {
	if registry == nil || workspaceResolver == nil {
		return nil, nil
	}

	workspaces, err := registry.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list workspaces for hooks rebuild: %w", err)
	}
	slices.SortFunc(workspaces, func(left, right workspacepkg.Workspace) int {
		return strings.Compare(strings.TrimSpace(left.ID), strings.TrimSpace(right.ID))
	})

	resolvedWorkspaces := make([]workspacepkg.ResolvedWorkspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		resolved, err := workspaceResolver.Resolve(ctx, workspace.ID)
		switch {
		case err == nil:
			resolvedWorkspaces = append(resolvedWorkspaces, resolved)
		case errors.Is(err, workspacepkg.ErrWorkspaceNotFound), errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
			if logger != nil {
				logger.Warn(
					"daemon: skipped workspace while rebuilding hooks",
					hooksBridgeWorkspaceIDKey, workspace.ID,
					"workspace_root", workspace.RootDir,
					"error", err,
				)
			}
		default:
			return nil, fmt.Errorf("daemon: resolve workspace %q for hooks rebuild: %w", workspace.ID, err)
		}
	}

	return resolvedWorkspaces, nil
}

func filterHookDeclsBySource(decls []hookspkg.HookDecl, source hookspkg.HookSource) []hookspkg.HookDecl {
	filtered := make([]hookspkg.HookDecl, 0, len(decls))
	for _, decl := range decls {
		if decl.Source != source {
			continue
		}
		filtered = append(filtered, cloneDaemonHookDecl(decl))
	}
	return filtered
}

func scopeWorkspaceHookDecls(
	decls []hookspkg.HookDecl,
	resolved *workspacepkg.ResolvedWorkspace,
) []hookspkg.HookDecl {
	scoped := make([]hookspkg.HookDecl, 0, len(decls))
	for _, decl := range decls {
		cloned := cloneDaemonHookDecl(decl)
		if resolved != nil {
			if strings.TrimSpace(cloned.WorkingDir) == "" {
				cloned.WorkingDir = strings.TrimSpace(resolved.RootDir)
			}
			if hookspkg.MatcherFieldAllowedForEvent(cloned.Event, hooksBridgeWorkspaceIDKey) {
				cloned.Matcher.WorkspaceID = strings.TrimSpace(resolved.WorkspaceID)
			}
			if hookspkg.MatcherFieldAllowedForEvent(cloned.Event, "workspace_root") {
				cloned.Matcher.WorkspaceRoot = strings.TrimSpace(resolved.RootDir)
			}
		}
		scoped = append(scoped, cloned)
	}
	return scoped
}

func scopeWorkspaceAgentHookDecls(
	decls []hookspkg.HookDecl,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) []hookspkg.HookDecl {
	scoped := scopeWorkspaceHookDecls(decls, resolved)
	for idx := range scoped {
		if hookspkg.MatcherFieldAllowedForEvent(scoped[idx].Event, "agent_name") {
			scoped[idx].Matcher.AgentName = strings.TrimSpace(agentName)
		}
	}
	return scoped
}

func cloneDaemonHookDecl(src hookspkg.HookDecl) hookspkg.HookDecl {
	cloned := src
	cloned.Args = append([]string(nil), src.Args...)
	cloned.Env = cloneStringMap(src.Env)
	cloned.SecretEnv = cloneStringMap(src.SecretEnv)
	cloned.Metadata = cloneStringMap(src.Metadata)
	if src.Matcher.ToolReadOnly != nil {
		value := *src.Matcher.ToolReadOnly
		cloned.Matcher.ToolReadOnly = &value
	}
	return cloned
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}

func marketplaceHookAllowlist(values []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}

	return allowed
}

func marketplaceHookAllowed(skill *skills.Skill, allowedMarketplaceHooks map[string]struct{}) bool {
	if skill == nil {
		return false
	}

	switch skill.Source {
	case skills.SourceBundled, skills.SourceUser, skills.SourceAdditional, skills.SourceWorkspace:
		return true
	case skills.SourceMarketplace:
		for _, key := range marketplaceHookConsentKeys(skill) {
			if _, ok := allowedMarketplaceHooks[key]; ok {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func marketplaceHookConsentKeys(skill *skills.Skill) []string {
	if skill == nil || skill.Provenance == nil {
		return nil
	}

	keys := make([]string, 0, 3)
	if slug := strings.TrimSpace(skill.Provenance.Slug); slug != "" {
		keys = append(keys, slug)
		if registry := strings.TrimSpace(skill.Provenance.Registry); registry != "" {
			keys = append(keys, registry+":"+slug)
		}
	}
	if hash := strings.TrimSpace(skill.Provenance.Hash); hash != "" {
		keys = append(keys, hash)
	}

	return keys
}
