package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/skills"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func chainDeclarationProviders(providers ...hookspkg.DeclarationProvider) hookspkg.DeclarationProvider {
	return func(ctx context.Context) ([]hookspkg.HookDecl, error) {
		chained := make([]hookspkg.HookDecl, 0, len(providers))
		for idx, provider := range providers {
			if provider == nil {
				continue
			}

			decls, err := provider(ctx)
			if err != nil {
				return nil, fmt.Errorf("daemon: load hook declarations from provider %d: %w", idx+1, err)
			}
			chained = append(chained, decls...)
		}
		return chained, nil
	}
}

func extensionDeclarationProvider(getRuntime func() extensionRuntime) hookspkg.DeclarationProvider {
	return func(ctx context.Context) ([]hookspkg.HookDecl, error) {
		if getRuntime == nil {
			return nil, nil
		}

		runtime := getRuntime()
		if runtime == nil {
			return nil, nil
		}
		decls, err := runtime.HookDeclarations(ctx)
		if err != nil {
			return nil, fmt.Errorf("daemon: load hook declarations from extension runtime: %w", err)
		}
		return decls, nil
	}
}

func configDeclarationProvider(
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	logger *slog.Logger,
) hookspkg.DeclarationProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context) ([]hookspkg.HookDecl, error) {
		decls, err := workspaceHookDeclarations(ctx, registry, workspaceResolver, logger)
		if err != nil {
			return nil, err
		}
		return filterHookDeclsBySource(decls, hookspkg.HookSourceConfig), nil
	}
}

func agentDeclarationProvider(
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	logger *slog.Logger,
) hookspkg.DeclarationProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context) ([]hookspkg.HookDecl, error) {
		decls, err := workspaceHookDeclarations(ctx, registry, workspaceResolver, logger)
		if err != nil {
			return nil, err
		}
		return filterHookDeclsBySource(decls, hookspkg.HookSourceAgentDefinition), nil
	}
}

func skillDeclarationProvider(
	skillsRegistry *skills.Registry,
	registry Registry,
	workspaceResolver workspacepkg.RuntimeResolver,
	allowedMarketplaceHooks []string,
	logger *slog.Logger,
) hookspkg.DeclarationProvider {
	if logger == nil {
		logger = slog.Default()
	}
	allowed := marketplaceHookAllowlist(allowedMarketplaceHooks)

	return func(ctx context.Context) ([]hookspkg.HookDecl, error) {
		if skillsRegistry == nil || registry == nil || workspaceResolver == nil {
			return nil, nil
		}

		workspaces, err := registeredWorkspaces(ctx, registry, workspaceResolver, logger)
		if err != nil {
			return nil, err
		}

		decls := make([]hookspkg.HookDecl, 0, len(workspaces))
		for idx := range workspaces {
			resolved := &workspaces[idx]
			if len(resolved.Agents) == 0 {
				activeSkills, err := skillsRegistry.ForWorkspace(ctx, resolved)
				if err != nil {
					return nil, fmt.Errorf(
						"daemon: resolve active skills for workspace %q: %w",
						resolved.WorkspaceID,
						err,
					)
				}
				decls = appendWorkspaceSkillHookDecls(decls, activeSkills, allowed, resolved, logger)
				continue
			}

			for _, agent := range resolved.Agents {
				activeSkills, err := activeSkillsForHookDeclarations(
					ctx,
					skillsRegistry,
					resolved,
					agent.Name,
					logger,
				)
				if err != nil {
					return nil, fmt.Errorf(
						"daemon: resolve active skills for workspace %q agent %q: %w",
						resolved.WorkspaceID,
						agent.Name,
						err,
					)
				}

				decls = appendAgentSkillHookDecls(decls, activeSkills, allowed, resolved, agent.Name, logger)
			}
		}

		return decls, nil
	}
}

func appendWorkspaceSkillHookDecls(
	decls []hookspkg.HookDecl,
	activeSkills []*skills.Skill,
	allowed map[string]struct{},
	resolved *workspacepkg.ResolvedWorkspace,
	logger *slog.Logger,
) []hookspkg.HookDecl {
	for _, skill := range activeSkills {
		if !marketplaceHookAllowed(skill, allowed) {
			logBlockedSkillHook(logger, skill, resolved, "")
			continue
		}
		decls = append(decls, scopeWorkspaceHookDecls(skill.Hooks, resolved)...)
	}
	return decls
}

func appendAgentSkillHookDecls(
	decls []hookspkg.HookDecl,
	activeSkills []*skills.Skill,
	allowed map[string]struct{},
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	logger *slog.Logger,
) []hookspkg.HookDecl {
	for _, skill := range activeSkills {
		if !marketplaceHookAllowed(skill, allowed) {
			logBlockedSkillHook(logger, skill, resolved, agentName)
			continue
		}
		decls = append(decls, scopeWorkspaceAgentHookDecls(skill.Hooks, resolved, agentName)...)
	}
	return decls
}

func logBlockedSkillHook(
	logger *slog.Logger,
	skill *skills.Skill,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) {
	if logger == nil || skill == nil {
		return
	}
	attrs := []any{
		"skill_name", skill.Meta.Name,
		hooksBridgeWorkspaceIDKey, resolvedHookWorkspaceID(resolved),
		"source", skills.SkillSourceName(skill.Source),
	}
	if agentName != "" {
		attrs = append(attrs, "agent_name", agentName)
	}
	logger.Warn("daemon: blocked hook", attrs...)
}

func activeSkillsForHookDeclarations(
	ctx context.Context,
	skillsRegistry *skills.Registry,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	logger *slog.Logger,
) ([]*skills.Skill, error) {
	activeSkills, err := skillsRegistry.ForAgent(ctx, resolved, agentName)
	if err == nil {
		return activeSkills, nil
	}
	if errors.Is(err, skills.ErrAgentLocalInvalid) {
		if logger != nil {
			logger.Warn(
				"daemon: skipped invalid agent-local skills while rebuilding hooks",
				hooksBridgeWorkspaceIDKey, resolvedHookWorkspaceID(resolved),
				"agent_name", agentName,
				"error", err,
			)
		}
		return nil, nil
	}
	return nil, err
}

func resolvedHookWorkspaceID(resolved *workspacepkg.ResolvedWorkspace) string {
	if resolved == nil {
		return ""
	}
	if workspaceID := strings.TrimSpace(resolved.WorkspaceID); workspaceID != "" {
		return workspaceID
	}
	return strings.TrimSpace(resolved.ID)
}
