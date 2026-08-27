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

const nativeNetworkChannelUpdateRequiredFields = "purpose, fanout_policy, or coordinator_peer_id"

type networkChannelCreateInput struct {
	WorkspaceID       string `json:"workspace"`
	Channel           string `json:"channel"`
	Purpose           string `json:"purpose"`
	FanoutPolicy      string `json:"fanout_policy,omitempty"`
	CoordinatorPeerID string `json:"coordinator_peer_id,omitempty"`
}

type networkChannelUpdateInput struct {
	WorkspaceID       string  `json:"workspace"`
	Channel           string  `json:"channel"`
	Purpose           *string `json:"purpose,omitempty"`
	FanoutPolicy      *string `json:"fanout_policy,omitempty"`
	CoordinatorPeerID *string `json:"coordinator_peer_id,omitempty"`
}

type agentCreateInput struct {
	Scope           string                             `json:"scope"`
	Workspace       string                             `json:"workspace,omitempty"`
	Name            string                             `json:"name"`
	Provider        string                             `json:"provider,omitempty"`
	Model           string                             `json:"model,omitempty"`
	ReasoningEffort string                             `json:"reasoning_effort,omitempty"`
	Speed           string                             `json:"speed,omitempty"`
	ACPOptions      []contract.AgentACPOptionSelection `json:"acp_options,omitempty"`
	Command         string                             `json:"command,omitempty"`
	Prompt          string                             `json:"prompt"`
	Permissions     string                             `json:"permissions,omitempty"`
	Tools           []string                           `json:"tools,omitempty"`
	Toolsets        []string                           `json:"toolsets,omitempty"`
	DenyTools       []string                           `json:"deny_tools,omitempty"`
	CategoryPath    []string                           `json:"category_path,omitempty"`
	DisabledSkills  []string                           `json:"disabled_skills,omitempty"`
}

func (n *daemonNativeTools) networkChannelCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkChannelCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	channel, err := nativeNetworkChannel(req.ToolID, input.Channel)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	purpose := strings.TrimSpace(input.Purpose)
	if purpose == "" {
		return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "purpose")
	}
	fanoutPolicy := store.NormalizeNetworkFanoutPolicy(input.FanoutPolicy)
	coordinatorPeerID := strings.TrimSpace(input.CoordinatorPeerID)
	if err := store.ValidateNetworkChannelFanoutConfiguration(fanoutPolicy, coordinatorPeerID); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	entry := store.NetworkChannelEntry{
		ProfileID:         scope.ProfileID,
		Channel:           channel,
		WorkspaceID:       workspaceID,
		Purpose:           purpose,
		FanoutPolicy:      fanoutPolicy,
		CoordinatorPeerID: coordinatorPeerID,
		CreatedBy:         strings.TrimSpace(scope.AgentName),
	}
	if err := n.deps.NetworkStore.CreateNetworkChannel(ctx, entry); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	return structuredNetworkResult(
		nativeNetworkChannelPayload(entry),
		"channel "+channel,
	)
}

func nativeNetworkChannelPayload(entry store.NetworkChannelEntry) map[string]any {
	return map[string]any{
		"channel":             strings.TrimSpace(entry.Channel),
		daemonWorkspaceIDKey:  strings.TrimSpace(entry.WorkspaceID),
		"purpose":             strings.TrimSpace(entry.Purpose),
		"fanout_policy":       store.NormalizeNetworkFanoutPolicy(entry.FanoutPolicy),
		"coordinator_peer_id": strings.TrimSpace(entry.CoordinatorPeerID),
	}
}

func (n *daemonNativeTools) networkChannelUpdate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkChannelUpdateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	channel, err := nativeNetworkChannel(req.ToolID, input.Channel)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.Purpose == nil && input.FanoutPolicy == nil && input.CoordinatorPeerID == nil {
		return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, nativeNetworkChannelUpdateRequiredFields)
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	ref := store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: channel}
	patch := store.NetworkChannelPatch{}
	if input.Purpose != nil {
		purpose := strings.TrimSpace(*input.Purpose)
		patch.Purpose = &purpose
	}
	if input.FanoutPolicy != nil {
		policy := strings.TrimSpace(*input.FanoutPolicy)
		fanoutPolicy := store.NormalizeNetworkFanoutPolicy(policy)
		if err := store.ValidateNetworkFanoutPolicy(fanoutPolicy); err != nil {
			return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
		}
		patch.FanoutPolicy = &fanoutPolicy
	}
	if input.CoordinatorPeerID != nil {
		coordinatorPeerID := strings.TrimSpace(*input.CoordinatorPeerID)
		patch.CoordinatorPeerID = &coordinatorPeerID
	}
	readScope := store.ReadScope{ProfileID: scope.ProfileID}
	if err := n.deps.NetworkStore.PatchNetworkChannel(ctx, readScope, ref, patch); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	entry, err := n.deps.NetworkStore.GetNetworkChannel(ctx, readScope, ref)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	return structuredNetworkResult(nativeNetworkChannelPayload(entry), "channel "+channel)
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
		return nativeNetworkInputError(id, err)
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
