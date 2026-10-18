package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type agentCreateInput struct {
	Scope           string                               `json:"scope"`
	Workspace       string                               `json:"workspace,omitempty"`
	Name            string                               `json:"name"`
	Provider        string                               `json:"provider,omitempty"`
	Model           string                               `json:"model,omitempty"`
	ReasoningEffort string                               `json:"reasoning_effort,omitempty"`
	Speed           string                               `json:"speed,omitempty"`
	ACPOptions      []contract.AgentACPOptionSelection   `json:"acp_options,omitempty"`
	Command         string                               `json:"command,omitempty"`
	Prompt          string                               `json:"prompt"`
	Permissions     string                               `json:"permissions,omitempty"`
	Tools           []string                             `json:"tools,omitempty"`
	Toolsets        []string                             `json:"toolsets,omitempty"`
	DenyTools       []string                             `json:"deny_tools,omitempty"`
	CategoryPath    []string                             `json:"category_path,omitempty"`
	DisabledSkills  []string                             `json:"disabled_skills,omitempty"`
	FallbackChain   []contract.AgentFallbackRoutePayload `json:"fallback_chain,omitempty"`
}

func (n *daemonNativeTools) agentCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input agentCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	createReq, err := n.agentCreateRequest(req.ToolID, scope, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	profileName, err := n.nativeAgentCreateProfileName(ctx, createReq.Scope, scope.ProfileID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAgentCreateToolError(req.ToolID, err)
	}
	agentSkills := n.deps.agentSkills()
	if agentSkills == nil {
		return toolspkg.ToolResult{}, nativeAgentCreateToolError(
			req.ToolID,
			errors.New("daemon: agent definition sync is unavailable"),
		)
	}
	created, err := core.CreateAgentFromRequest(
		ctx,
		createReq,
		n.deps.HomePaths,
		&n.deps.Config,
		n.deps.Workspaces,
		string(req.ToolID),
		profileName,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAgentCreateToolError(req.ToolID, err)
	}
	agent := created.Agent
	if err := agentSkills.Sync(ctx); err != nil {
		syncErr := errors.Join(
			fmt.Errorf("daemon: sync created agent definition: %w", err),
			n.rollbackNativeAgentCreate(ctx, agent.SourcePath),
		)
		return toolspkg.ToolResult{}, nativeAgentCreateToolError(
			req.ToolID,
			syncErr,
		)
	}
	entry := core.AgentCatalogEntry{Def: agent, Origin: contract.AgentOriginGlobal}
	if createReq.Scope == contract.AgentCreateScopeWorkspace {
		entry.Origin = contract.AgentOriginWorkspace
		entry.WorkspaceID = created.WorkspaceID
	}
	payload := core.AgentPayloadFromEntryWithConfig(entry, &created.RuntimeConfig)
	return structuredResult(map[string]any{daemonAgentField: payload}, "agent "+payload.Name)
}

func (n *daemonNativeTools) nativeAgentCreateProfileName(
	ctx context.Context,
	scope contract.AgentCreateScope,
	profileID string,
) (string, error) {
	if scope != contract.AgentCreateScopeWorkspace {
		return "", nil
	}
	profileID = strings.TrimSpace(profileID)
	if n.deps.Profiles != nil {
		profileName, err := n.deps.Profiles.ProfileName(ctx, profileID)
		if err != nil {
			return "", fmt.Errorf("daemon: resolve agent create profile: %w", err)
		}
		profileName = strings.TrimSpace(profileName)
		if profileName == "" {
			return "", errors.New("daemon: resolved agent create profile is empty")
		}
		return profileName, nil
	}
	if profileID == "" || profileID == store.DefaultProfileID {
		return daemonDefaultProfileName, nil
	}
	return "", errors.New("daemon: profile lookup is unavailable for agent create")
}

func (n *daemonNativeTools) rollbackNativeAgentCreate(ctx context.Context, sourcePath string) error {
	agentsRoot := filepath.Dir(filepath.Dir(sourcePath))
	if err := compozyconfig.DeleteAgentDefinition(agentsRoot, sourcePath); err != nil {
		return fmt.Errorf("daemon: roll back created agent definition: %w", err)
	}
	agentSkills := n.deps.agentSkills()
	if agentSkills == nil {
		return errors.New("daemon: agent definition sync is unavailable")
	}
	if err := agentSkills.Sync(ctx); err != nil {
		return fmt.Errorf("daemon: reconcile catalog after agent create rollback: %w", err)
	}
	return nil
}

func (n *daemonNativeTools) agentCreateRequest(
	id toolspkg.ToolID,
	scope toolspkg.Scope,
	input agentCreateInput,
) (contract.CreateAgentRequest, error) {
	createReq := contract.CreateAgentRequest{
		Scope:     contract.AgentCreateScope(strings.TrimSpace(input.Scope)),
		Workspace: strings.TrimSpace(input.Workspace),
		Agent: contract.CreateAgentPayload{
			Name:            strings.TrimSpace(input.Name),
			Provider:        strings.TrimSpace(input.Provider),
			Command:         strings.TrimSpace(input.Command),
			Model:           strings.TrimSpace(input.Model),
			ReasoningEffort: contract.ReasoningEffort(strings.TrimSpace(input.ReasoningEffort)),
			Speed:           contract.Speed(strings.TrimSpace(input.Speed)),
			ACPOptions:      cloneNativeAgentACPOptions(input.ACPOptions),
			Prompt:          input.Prompt,
			Permissions:     contract.SettingsPermissionMode(strings.TrimSpace(input.Permissions)),
			Tools:           trimNativeStrings(input.Tools),
			Toolsets:        trimNativeStrings(input.Toolsets),
			DenyTools:       trimNativeStrings(input.DenyTools),
			CategoryPath:    trimNativeStrings(input.CategoryPath),
			FallbackChain:   cloneNativeAgentFallbackChain(input.FallbackChain),
		},
	}
	if len(input.DisabledSkills) > 0 {
		createReq.Agent.Skills = &contract.CreateAgentSkillsConfig{
			Disabled: trimNativeStrings(input.DisabledSkills),
		}
	}
	if createReq.Scope == contract.AgentCreateScopeWorkspace {
		workspaceRef := nativeCallerWorkspaceInput(createReq.Workspace, scope)
		if strings.TrimSpace(workspaceRef) == "" {
			return contract.CreateAgentRequest{}, nativeRequiredInputError(id, "workspace")
		}
		createReq.Workspace = workspaceRef
	}
	return createReq, nil
}

func cloneNativeAgentACPOptions(
	options []contract.AgentACPOptionSelection,
) []contract.AgentACPOptionSelection {
	if len(options) == 0 {
		return nil
	}
	cloned := make([]contract.AgentACPOptionSelection, len(options))
	for index, option := range options {
		cloned[index] = contract.AgentACPOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			cloned[index].BoolValue = new(*option.BoolValue)
		}
	}
	return cloned
}

func cloneNativeAgentFallbackChain(
	chain []contract.AgentFallbackRoutePayload,
) []contract.AgentFallbackRoutePayload {
	if len(chain) == 0 {
		return nil
	}
	cloned := make([]contract.AgentFallbackRoutePayload, len(chain))
	for index, route := range chain {
		cloned[index] = contract.AgentFallbackRoutePayload{
			Provider:        strings.TrimSpace(route.Provider),
			Model:           strings.TrimSpace(route.Model),
			ReasoningEffort: contract.ReasoningEffort(strings.TrimSpace(string(route.ReasoningEffort))),
			Speed:           contract.Speed(strings.TrimSpace(string(route.Speed))),
			ACPOptions:      cloneNativeAgentACPOptions(route.ACPOptions),
			Command:         strings.TrimSpace(route.Command),
		}
	}
	return cloned
}

func nativeAgentCreateToolError(id toolspkg.ToolID, err error) error {
	switch {
	case errors.Is(err, compozyconfig.ErrAgentNameReserved):
		return nativeReservedAgentNameToolError(id, err)
	case errors.Is(err, compozyconfig.ErrAgentDefinitionExists):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonConflictedID,
		)
	case errors.Is(err, compozyconfig.ErrInvalidAgentDefinition):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	default:
		return nativeInputError(id, err)
	}
}

func nativeReservedAgentNameToolError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeAgentNameReserved,
		id,
		err.Error(),
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonSchemaInvalid,
	)
}
