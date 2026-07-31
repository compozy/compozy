package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionCreateInput struct {
	Workspace            string                 `json:"workspace,omitempty"`
	Agent                string                 `json:"agent"`
	Name                 string                 `json:"name,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
}

type sessionPromptInput struct {
	Workspace string                                  `json:"workspace,omitempty"`
	SessionID string                                  `json:"session_id"`
	Message   string                                  `json:"message"`
	Runtime   *contract.PromptRuntimeSelectionPayload `json:"runtime,omitempty"`
}

func (n *daemonNativeTools) sessionCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	agent, err := requiredNativeString(req.ToolID, "agent", input.Agent)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	acceptance, ok := n.deps.Sessions.(core.SessionAcceptanceManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: durable session acceptance is required")
	}
	info, err := acceptance.CreateAccepted(ctx, session.CreateAcceptedOpts{Session: session.CreateOpts{
		AgentName: agent,
		Name:      strings.TrimSpace(input.Name),
		Workspace: workspaceID,
		Type:      session.SessionTypeUser,
		NetworkParticipation: participation.CloneRequest(
			input.NetworkParticipation,
		),
	}})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

func (n *daemonNativeTools) sessionPrompt(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionPromptInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	message, err := requiredNativeString(req.ToolID, "message", input.Message)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, workspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := n.deps.Sessions.SendPrompt(ctx, sessionID, session.SendPromptOpts{
		Message: message,
		Runtime: core.PromptRuntimeSelectionFromPayload(input.Runtime),
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if result.Events != nil {
		for range result.Events {
			continue
		}
	}
	payload, err := core.PromptResultPayloadFromSession(result)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"prompt": payload}, payload.Status)
}
