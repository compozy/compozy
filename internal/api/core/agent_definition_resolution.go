package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type resolvedAgentDefinition struct {
	Entry              AgentCatalogEntry
	OperationWorkspace string
	WorkspaceRoot      string
	Config             aghconfig.Config
}

type agentDefinitionMutationTarget struct {
	Path        string
	Origin      contract.AgentOrigin
	WorkspaceID string
	Config      aghconfig.Config
}

func (h *BaseHandlers) resolveAgentDefinition(
	ctx context.Context,
	workspaceRef string,
	name string,
) (resolvedAgentDefinition, error) {
	target := aghconfig.NormalizeAgentName(name)
	if target == "" {
		return resolvedAgentDefinition{}, errors.Join(
			errAgentDefinitionInvalid,
			errors.New("agent name is required"),
		)
	}

	workspaceRef = strings.TrimSpace(workspaceRef)
	if workspaceRef != "" {
		if h.Workspaces == nil {
			return resolvedAgentDefinition{}, fmt.Errorf(
				"api: %w",
				workspacepkg.ErrWorkspaceResolverUnavailable,
			)
		}
		resolved, err := h.Workspaces.Resolve(ctx, workspaceRef)
		if err != nil {
			return resolvedAgentDefinition{}, err
		}
		for _, agent := range resolved.Agents {
			if aghconfig.NormalizeAgentName(agent.Name) != target {
				continue
			}
			entry := h.agentCatalogEntryFromDef(agent, resolved.ID)
			return resolvedAgentDefinition{
				Entry:              entry,
				OperationWorkspace: strings.TrimSpace(resolved.ID),
				WorkspaceRoot:      strings.TrimSpace(resolved.RootDir),
				Config:             resolved.Config,
			}, nil
		}
		return resolvedAgentDefinition{}, fmt.Errorf(
			"api: agent %q is not available in workspace %q: %w",
			target,
			workspaceRef,
			workspacepkg.ErrAgentNotAvailable,
		)
	}

	if h.AgentCatalog != nil {
		entry, err := h.AgentCatalog.GetAgent(ctx, target)
		if err != nil {
			return resolvedAgentDefinition{}, err
		}
		return resolvedAgentDefinition{
			Entry:  entry,
			Config: h.Config,
		}, nil
	}

	agent, err := h.AgentLoader(target, h.HomePaths)
	if err != nil {
		return resolvedAgentDefinition{}, err
	}
	return resolvedAgentDefinition{
		Entry:  h.agentCatalogEntryFromDef(agent, ""),
		Config: h.Config,
	}, nil
}

func (h *BaseHandlers) duplicateAgentTarget(
	ctx context.Context,
	req contract.DuplicateAgentRequest,
	source *resolvedAgentDefinition,
) (agentDefinitionMutationTarget, error) {
	scope := req.Scope
	if scope == "" {
		scope = contract.AgentCreateScope(source.Entry.Origin)
	}
	targetName := aghconfig.NormalizeAgentName(req.Name)
	switch scope {
	case contract.AgentCreateScopeGlobal:
		return agentDefinitionMutationTarget{
			Path:   filepath.Join(h.HomePaths.AgentsDir, targetName),
			Origin: contract.AgentOriginGlobal,
			Config: h.Config,
		}, nil
	case contract.AgentCreateScopeWorkspace:
		workspaceRef := strings.TrimSpace(req.Workspace)
		if workspaceRef == "" && source.Entry.Origin == contract.AgentOriginWorkspace {
			if source.WorkspaceRoot == "" {
				return agentDefinitionMutationTarget{}, errors.Join(
					errAgentDefinitionInvalid,
					errors.New("source workspace root is unavailable"),
				)
			}
			return agentDefinitionMutationTarget{
				Path: filepath.Join(
					source.WorkspaceRoot,
					aghconfig.DirName,
					aghconfig.AgentsDirName,
					targetName,
				),
				Origin:      contract.AgentOriginWorkspace,
				WorkspaceID: source.OperationWorkspace,
				Config:      source.Config,
			}, nil
		}
		if workspaceRef == "" {
			return agentDefinitionMutationTarget{}, errors.Join(
				errAgentDefinitionInvalid,
				errors.New("workspace is required for workspace-scoped duplicate"),
			)
		}
		if h.Workspaces == nil {
			return agentDefinitionMutationTarget{}, fmt.Errorf(
				"api: %w",
				workspacepkg.ErrWorkspaceResolverUnavailable,
			)
		}
		resolved, err := h.Workspaces.Resolve(ctx, workspaceRef)
		if err != nil {
			return agentDefinitionMutationTarget{}, err
		}
		return agentDefinitionMutationTarget{
			Path: filepath.Join(
				resolved.RootDir,
				aghconfig.DirName,
				aghconfig.AgentsDirName,
				targetName,
			),
			Origin:      contract.AgentOriginWorkspace,
			WorkspaceID: strings.TrimSpace(resolved.ID),
			Config:      resolved.Config,
		}, nil
	default:
		return agentDefinitionMutationTarget{}, errors.Join(
			errAgentDefinitionInvalid,
			fmt.Errorf("scope must be %q or %q", contract.AgentCreateScopeGlobal, contract.AgentCreateScopeWorkspace),
		)
	}
}

func (h *BaseHandlers) agentDefinitionAgentsRoot(resolved *resolvedAgentDefinition) (string, error) {
	switch resolved.Entry.Origin {
	case contract.AgentOriginGlobal:
		return h.HomePaths.AgentsDir, nil
	case contract.AgentOriginWorkspace:
		if resolved.WorkspaceRoot == "" {
			return "", errors.Join(
				errAgentDefinitionInvalid,
				errors.New("source workspace root is unavailable"),
			)
		}
		return filepath.Join(resolved.WorkspaceRoot, aghconfig.DirName, aghconfig.AgentsDirName), nil
	default:
		return "", errors.Join(
			errAgentDefinitionInvalid,
			fmt.Errorf("unsupported agent origin %q", resolved.Entry.Origin),
		)
	}
}

func (h *BaseHandlers) globalAgentTwinExists(name string, effectiveSourcePath string) bool {
	path := filepath.Join(
		h.HomePaths.AgentsDir,
		aghconfig.NormalizeAgentName(name),
		aghconfig.AgentDefinitionFileName,
	)
	if filepath.Clean(path) == filepath.Clean(effectiveSourcePath) {
		return false
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		h.Logger.Debug("api: global agent twin is unavailable for disclosure", "path", path, "error", err)
		return false
	}
	if !info.Mode().IsRegular() {
		return false
	}
	agent, err := aghconfig.LoadAgentDefFile(path)
	if err != nil {
		h.Logger.Debug("api: global agent twin is invalid for disclosure", "path", path, "error", err)
		return false
	}
	return aghconfig.NormalizeAgentName(agent.Name) == aghconfig.NormalizeAgentName(name)
}
