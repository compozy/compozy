package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

func (m *Manager) resolveSessionStartRuntime(
	ctx context.Context,
	spec *sessionStartSpec,
) (sessionStartRuntime, error) {
	artifacts, err := m.resolveWorkspaceAgentArtifactsForSession(spec.agentName, spec.sessionType, &spec.workspace)
	if err != nil {
		return sessionStartRuntime{}, fmt.Errorf("session: resolve workspace agent %q: %w", spec.agentName, err)
	}
	agentDef := artifacts.Agent
	resolved, err := spec.workspace.Config.ResolveSessionAgentWithRuntime(agentDef, spec.provider, spec.model)
	if err != nil {
		return sessionStartRuntime{}, fmt.Errorf("session: resolve session agent %q: %w", spec.agentName, err)
	}
	if err := spec.validateRuntimeOverrides(); err != nil {
		return sessionStartRuntime{}, fmt.Errorf("session: validate runtime overrides for %q: %w", spec.sessionID, err)
	}
	if err := m.validateExplicitModel(ctx, spec, resolved); err != nil {
		return sessionStartRuntime{}, fmt.Errorf("session: validate model for %q: %w", spec.sessionID, err)
	}
	if err := spec.applyResolvedReasoningEffort(resolved); err != nil {
		return sessionStartRuntime{}, err
	}
	if err := spec.applyAllowedToolsOverride(&resolved, m.toolsetCatalog); err != nil {
		return sessionStartRuntime{}, fmt.Errorf("session: apply allowed tools for %q: %w", spec.sessionID, err)
	}
	return sessionStartRuntime{
		agent:               resolved,
		agentDef:            aghconfig.CloneAgentDef(agentDef),
		networkCapabilities: networkPeerCapabilities(agentDef.Capabilities),
	}, nil
}

func (m *Manager) prepareAcceptedSessionRuntime(
	ctx context.Context,
	spec *sessionStartSpec,
	runtime *sessionStartRuntime,
	updatedAt time.Time,
) error {
	if runtime == nil {
		return fmt.Errorf("session: accepted runtime is required")
	}
	artifacts, err := m.resolveWorkspaceAgentArtifactsForSession(spec.agentName, spec.sessionType, &spec.workspace)
	if err != nil {
		return fmt.Errorf("session: resolve workspace agent %q: %w", spec.agentName, err)
	}
	if err := m.prepareSessionStartSoul(ctx, spec, artifacts, updatedAt); err != nil {
		return fmt.Errorf("session: prepare soul for %q: %w", spec.sessionID, err)
	}

	agentDef := aghconfig.CloneAgentDef(runtime.agentDef)
	startupCtx := spec.startupPromptContext(updatedAt)
	if strings.TrimSpace(startupCtx.AgentName) == "" {
		startupCtx.AgentName = strings.TrimSpace(agentDef.Name)
	}
	if strings.TrimSpace(startupCtx.Provider) == "" {
		startupCtx.Provider = strings.TrimSpace(runtime.agent.Provider)
	}
	startupPrompt, err := m.startupPrompt(
		ctx,
		spec.startupSessionContext(updatedAt),
		startupCtx,
		agentDef,
		&spec.workspace,
	)
	if err != nil {
		return fmt.Errorf("session: assemble startup prompt for %q: %w", spec.sessionID, err)
	}
	if m.startupOverlay != nil {
		startupPrompt, err = m.startupOverlay.Apply(ctx, startupCtx, startupPrompt)
		if err != nil {
			return fmt.Errorf("session: apply startup prompt overlay: %w", err)
		}
	}
	agentDef.Prompt = startupPrompt
	if overlay := strings.TrimSpace(spec.promptOverlay); overlay != "" {
		if strings.TrimSpace(agentDef.Prompt) == "" {
			agentDef.Prompt = overlay
		} else {
			agentDef.Prompt = strings.TrimSpace(agentDef.Prompt) + "\n\n" + overlay
		}
	}
	runtime.agentDef = aghconfig.CloneAgentDef(agentDef)
	runtime.agent.Prompt = agentDef.Prompt

	runtime.mcpServers, err = m.sessionMCPServers(ctx, spec, runtime.agent, agentDef)
	if err != nil {
		return fmt.Errorf("session: resolve MCP servers for %q: %w", spec.sessionID, err)
	}
	return nil
}

func (s *sessionStartSpec) validateRuntimeOverrides() error {
	providerOverride := strings.TrimSpace(s.provider)
	modelOverride := strings.TrimSpace(s.model)
	reasoningEffort := strings.TrimSpace(s.reasoningEffort)
	if modelOverride != "" && providerOverride == "" {
		return fmt.Errorf("%w: provider is required when model is set", ErrInvalidRuntimeOverride)
	}
	if reasoningEffort == "" {
		return nil
	}
	if providerOverride == "" {
		return fmt.Errorf("%w: provider is required when reasoning_effort is set", ErrInvalidRuntimeOverride)
	}
	return ValidateReasoningEffort(reasoningEffort)
}

func (s *sessionStartSpec) startLogger(m *Manager) *slog.Logger {
	logger := slog.Default()
	if m != nil && m.logger != nil {
		logger = m.logger
	}
	return logger.With(
		"session_id", strings.TrimSpace(s.sessionID),
		"agent_name", strings.TrimSpace(s.agentName),
		"provider", strings.TrimSpace(s.provider),
		"workspace_id", strings.TrimSpace(s.workspace.ID),
	)
}
