package extensionpkg

import (
	"context"
	"encoding/json"

	"fmt"

	"path/filepath"
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (h *HostAPIHandler) handleSessionsHealthGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionHealthGetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	health, err := h.hostAPISessionHealthPayload(ctx, params.WorkspaceID, params.SessionID)
	if err != nil {
		return nil, err
	}
	return apicontract.SessionHealthResponse{Health: health}, nil
}

func (h *HostAPIHandler) handleSessionsStatusGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPISessionStatusGetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	health, err := h.hostAPISessionHealthPayload(ctx, params.WorkspaceID, params.SessionID)
	if err != nil {
		return nil, err
	}
	response := apicontract.SessionStatusResponse{
		SessionID:           health.SessionID,
		WorkspaceID:         health.WorkspaceID,
		AgentName:           health.AgentName,
		State:               health.State,
		Health:              health.Health,
		ActivePrompt:        health.ActivePrompt,
		Attachable:          health.Attachable,
		EligibleForWake:     health.EligibleForWake,
		IneligibilityReason: health.IneligibilityReason,
		UpdatedAt:           health.UpdatedAt,
	}
	if h.heartbeatStatus == nil {
		return response, nil
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, health.WorkspaceID, health.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	status, err := h.heartbeatStatus.Status(ctx, heartbeat.StatusRequest{
		Target:    target.heartbeatAuthoringTarget(),
		SessionID: health.SessionID,
	})
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	converted, err := apicontract.HeartbeatStatusResponseFromResult(&status)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	response.WakeState = converted.WakeState
	return response, nil
}

func (h *HostAPIHandler) resolveHostAPIAuthoredAgentTarget(
	ctx context.Context,
	workspaceRef string,
	agentName string,
) (hostAPIAuthoredAgentTarget, error) {
	name := strings.TrimSpace(agentName)
	if name == "" {
		return hostAPIAuthoredAgentTarget{}, fmt.Errorf("%w: agent_name is required", errHostAPIAuthoredValidation)
	}
	ref := strings.TrimSpace(workspaceRef)
	if ref == "" {
		return hostAPIAuthoredAgentTarget{}, fmt.Errorf("%w: workspace_id is required", errHostAPIAuthoredValidation)
	}
	if h.workspaces == nil {
		return hostAPIAuthoredAgentTarget{}, workspacepkg.ErrWorkspaceResolverUnavailable
	}
	resolved, err := h.workspaces.Resolve(ctx, ref)
	if err != nil {
		return hostAPIAuthoredAgentTarget{}, err
	}
	root := strings.TrimSpace(resolved.RootDir)
	if root == "" {
		return hostAPIAuthoredAgentTarget{}, workspacepkg.ErrWorkspaceRootMissing
	}
	workspaceID, err := hostAPIResolvedWorkspaceRegistrationID(&resolved)
	if err != nil {
		return hostAPIAuthoredAgentTarget{}, err
	}
	return hostAPIAuthoredAgentTarget{
		workspaceID:     workspaceID,
		workspaceRoot:   root,
		agentName:       name,
		agentPath:       hostAPIAuthoredAgentPath(&resolved, name),
		soulConfig:      resolved.Config.Agents.Soul,
		heartbeatConfig: resolved.Config.Agents.Heartbeat,
	}, nil
}

func (t hostAPIAuthoredAgentTarget) soulAuthoringTarget() soul.AuthoringTarget {
	return soul.AuthoringTarget{
		WorkspaceID:   t.workspaceID,
		WorkspaceRoot: t.workspaceRoot,
		AgentName:     t.agentName,
		AgentPath:     t.agentPath,
		Config:        t.soulConfig,
		ConfigSource:  hostAPIAuthoredContextWorkspaceKey,
	}
}

func (t hostAPIAuthoredAgentTarget) heartbeatAuthoringTarget() heartbeat.AuthoringTarget {
	return heartbeat.AuthoringTarget{
		WorkspaceID:   t.workspaceID,
		WorkspaceRoot: t.workspaceRoot,
		AgentName:     t.agentName,
		AgentPath:     t.agentPath,
		Config:        t.heartbeatConfig,
	}
}

func (t hostAPIAuthoredAgentTarget) soulConfigProvenance() soul.ConfigProvenance {
	provenance, err := soul.NewConfigProvenance(t.soulConfig, hostAPIAuthoredContextWorkspaceKey)
	if err != nil {
		return soul.ConfigProvenance{}
	}
	return provenance
}

func hostAPIAuthoredAgentPath(workspace *workspacepkg.ResolvedWorkspace, agentName string) string {
	name := strings.TrimSpace(agentName)
	if workspace == nil {
		return ""
	}
	for _, agent := range workspace.Agents {
		if strings.TrimSpace(agent.Name) == name && strings.TrimSpace(agent.SourcePath) != "" {
			return strings.TrimSpace(agent.SourcePath)
		}
	}
	if root := strings.TrimSpace(workspace.RootDir); root != "" && name != "" {
		return filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, name, "AGENT.md")
	}
	return ""
}
