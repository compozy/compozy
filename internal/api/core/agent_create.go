package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// CreateAgentFromRequest validates and persists one AGENT.md definition from a
// shared create-agent request. It is the single authoring path reused by the
// HTTP handler and the compozy__agent_create native tool.
func CreateAgentFromRequest(
	ctx context.Context,
	req contract.CreateAgentRequest,
	homePaths compozyconfig.HomePaths,
	globalConfig *compozyconfig.Config,
	workspaces WorkspaceService,
	transportName string,
) (compozyconfig.AgentDef, compozyconfig.Config, error) {
	draft, err := createAgentDraftFromRequest(req)
	if err != nil {
		return compozyconfig.AgentDef{}, compozyconfig.Config{}, err
	}
	target, err := createAgentDefinitionTargetFor(
		ctx, req, homePaths, globalConfig, workspaces, transportName,
	)
	if err != nil {
		return compozyconfig.AgentDef{}, compozyconfig.Config{}, err
	}
	if err := validateAgentDraftRuntime(draft, &target.Config); err != nil {
		return compozyconfig.AgentDef{}, compozyconfig.Config{}, err
	}
	agent, err := compozyconfig.CreateAgentDefFile(target.Path, draft, false)
	if err != nil {
		return compozyconfig.AgentDef{}, compozyconfig.Config{}, err
	}
	return agent, target.Config, nil
}

func createAgentDraftFromRequest(req contract.CreateAgentRequest) (compozyconfig.AgentDefinitionDraft, error) {
	agent := req.Agent
	agentName := compozyconfig.NormalizeAgentName(agent.Name)
	if agentName == "" {
		return compozyconfig.AgentDefinitionDraft{}, errors.Join(
			errCreateAgentRequestInvalid,
			errors.New("agent.name is required"),
		)
	}
	if err := compozyconfig.ValidateAuthoredAgentName(agentName); err != nil {
		return compozyconfig.AgentDefinitionDraft{}, errors.Join(
			errCreateAgentRequestInvalid,
			compozyconfig.ErrInvalidAgentDefinition,
			err,
		)
	}
	if strings.TrimSpace(agent.Prompt) == "" {
		return compozyconfig.AgentDefinitionDraft{}, errors.Join(
			errCreateAgentRequestInvalid,
			errors.New("agent.prompt is required"),
		)
	}
	if err := session.ValidateReasoningEffort(string(agent.ReasoningEffort)); err != nil {
		return compozyconfig.AgentDefinitionDraft{}, errors.Join(
			errCreateAgentRequestInvalid,
			compozyconfig.ErrInvalidAgentDefinition,
			err,
		)
	}
	disabledSkills := []string(nil)
	if agent.Skills != nil {
		disabledSkills = append([]string(nil), agent.Skills.Disabled...)
	}
	return compozyconfig.AgentDefinitionDraft{
		Name:            agentName,
		Provider:        agent.Provider,
		Command:         agent.Command,
		Model:           agent.Model,
		ReasoningEffort: string(agent.ReasoningEffort),
		Tools:           append([]string(nil), agent.Tools...),
		Toolsets:        append([]string(nil), agent.Toolsets...),
		DenyTools:       append([]string(nil), agent.DenyTools...),
		Permissions:     string(agent.Permissions),
		Skills:          compozyconfig.AgentSkillsConfig{Disabled: disabledSkills},
		CategoryPath:    append([]string(nil), agent.CategoryPath...),
		Prompt:          agent.Prompt,
	}, nil
}

func validateAgentDraftRuntime(draft compozyconfig.AgentDefinitionDraft, cfg *compozyconfig.Config) error {
	_, agent, err := compozyconfig.RenderAgentDefinition(draft)
	if err != nil {
		return err
	}
	if _, err := cfg.ResolveAgent(agent); err != nil {
		return errors.Join(
			errCreateAgentRequestInvalid,
			fmt.Errorf("agent runtime cannot be resolved in the target scope: %w", err),
		)
	}
	return nil
}

func createAgentDefinitionPathFor(
	ctx context.Context,
	req contract.CreateAgentRequest,
	homePaths compozyconfig.HomePaths,
	globalConfig *compozyconfig.Config,
	workspaces WorkspaceService,
	transportName string,
) (string, error) {
	target, err := createAgentDefinitionTargetFor(
		ctx, req, homePaths, globalConfig, workspaces, transportName,
	)
	if err != nil {
		return "", err
	}
	return target.Path, nil
}

type createAgentDefinitionTarget struct {
	Path        string
	WorkspaceID string
	Config      compozyconfig.Config
}

func createAgentDefinitionTargetFor(
	ctx context.Context,
	req contract.CreateAgentRequest,
	homePaths compozyconfig.HomePaths,
	globalConfig *compozyconfig.Config,
	workspaces WorkspaceService,
	transportName string,
) (createAgentDefinitionTarget, error) {
	name := compozyconfig.NormalizeAgentName(req.Agent.Name)
	switch req.Scope {
	case contract.AgentCreateScopeGlobal:
		var config compozyconfig.Config
		if globalConfig != nil {
			config = *globalConfig
		}
		return createAgentDefinitionTarget{
			Path:   filepath.Join(homePaths.AgentsDir, name, compozyconfig.AgentDefinitionFileName),
			Config: config,
		}, nil
	case contract.AgentCreateScopeWorkspace:
		workspaceRef := strings.TrimSpace(req.Workspace)
		if workspaceRef == "" {
			return createAgentDefinitionTarget{}, errors.Join(
				errCreateAgentRequestInvalid,
				errors.New("workspace is required for workspace-scoped agents"),
			)
		}
		if workspaces == nil {
			return createAgentDefinitionTarget{}, fmt.Errorf(
				"%s: %w",
				transportName,
				workspacepkg.ErrWorkspaceResolverUnavailable,
			)
		}
		resolved, err := workspaces.Resolve(ctx, workspaceRef)
		if err != nil {
			return createAgentDefinitionTarget{}, err
		}
		rootDir := strings.TrimSpace(resolved.RootDir)
		if rootDir == "" {
			return createAgentDefinitionTarget{}, fmt.Errorf(
				"%s: %w",
				transportName,
				workspacepkg.ErrWorkspaceRootMissing,
			)
		}
		return createAgentDefinitionTarget{
			Path: filepath.Join(
				rootDir,
				compozyconfig.DirName,
				compozyconfig.AgentsDirName,
				name,
				compozyconfig.AgentDefinitionFileName,
			),
			WorkspaceID: strings.TrimSpace(resolved.WorkspaceID),
			Config:      resolved.Config,
		}, nil
	default:
		return createAgentDefinitionTarget{}, errors.Join(
			errCreateAgentRequestInvalid,
			fmt.Errorf(
				"scope must be %q or %q",
				contract.AgentCreateScopeWorkspace,
				contract.AgentCreateScopeGlobal,
			),
		)
	}
}
