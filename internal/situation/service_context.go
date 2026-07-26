package situation

import (
	"context"

	"errors"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/session"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// ContextForStartup assembles the bounded context available before the agent driver starts.
func (s *Service) ContextForStartup(
	ctx context.Context,
	startup session.StartupPromptContext,
	agent aghconfig.AgentDef,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
) (contract.AgentContextPayload, error) {
	if s == nil {
		return contract.AgentContextPayload{}, nil
	}
	if err := checkContext(ctx); err != nil {
		return contract.AgentContextPayload{}, err
	}

	workspaceSnapshot, err := s.resolveWorkspace(ctx, startup.WorkspaceID, startup.Workspace, resolvedWorkspace)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}

	agentName := firstTrimmed(startup.AgentName, agent.Name)
	resolvedAgent, agentDef, err := s.resolveAgent(agentName, startup.Provider, agent, workspaceSnapshot)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}

	workspaceSection := workspacePayload(workspaceSnapshot, startup.WorkspaceID, startup.Workspace)
	capabilities, err := s.capabilitiesSection(ctx, workspaceSnapshot, agentDef)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}
	limits, err := s.limits(ctx, workspaceSection.ID)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}

	payload := contract.AgentContextPayload{
		Self: contract.AgentIdentityPayload{
			SessionID: strings.TrimSpace(startup.SessionID),
			AgentName: agentName,
			Provider:  firstTrimmed(resolvedAgent.Provider, startup.Provider, agent.Provider),
			Model:     firstTrimmed(resolvedAgent.Model, agent.Model),
		},
		Workspace:    workspaceSection,
		Session:      startupSessionPayload(startup),
		Capabilities: capabilities,
		Limits:       limits,
		Provenance:   s.provenance(),
	}

	normalized := contract.NormalizeAgentContextPayload(&payload)
	return normalized, nil
}

// ContextForSession assembles the bounded context for an active session.
func (s *Service) ContextForSession(
	ctx context.Context,
	info *session.Info,
) (contract.AgentContextPayload, error) {
	if s == nil {
		return contract.AgentContextPayload{}, nil
	}
	if info == nil {
		return contract.AgentContextPayload{}, errors.New("situation: session info is required")
	}
	if err := checkContext(ctx); err != nil {
		return contract.AgentContextPayload{}, err
	}

	workspaceSnapshot, err := s.resolveWorkspace(ctx, info.WorkspaceID, info.Workspace, nil)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}
	resolvedAgent, agentDef, err := s.resolveAgent(
		info.AgentName,
		info.Provider,
		aghconfig.AgentDef{},
		workspaceSnapshot,
	)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}

	workspaceSection := workspacePayload(workspaceSnapshot, info.WorkspaceID, info.Workspace)
	soulSection, err := s.soulPayload(ctx, info)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}
	capabilities, err := s.capabilitiesSection(ctx, workspaceSnapshot, agentDef)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}
	limits, err := s.limits(ctx, workspaceSection.ID)
	if err != nil {
		return contract.AgentContextPayload{}, err
	}

	payload := contract.AgentContextPayload{
		Self: contract.AgentIdentityPayload{
			SessionID: strings.TrimSpace(info.ID),
			AgentName: strings.TrimSpace(info.AgentName),
			Provider:  firstTrimmed(info.Provider, resolvedAgent.Provider, agentDef.Provider),
			Model:     firstTrimmed(resolvedAgent.Model, agentDef.Model),
		},
		Workspace:    workspaceSection,
		Session:      sessionPayload(info),
		Soul:         soulSection,
		Capabilities: capabilities,
		Limits:       limits,
		Provenance:   s.provenance(),
	}

	if err := s.populateSessionRuntimeContext(ctx, info, workspaceSnapshot, workspaceSection.ID, &payload); err != nil {
		return contract.AgentContextPayload{}, err
	}

	normalized := contract.NormalizeAgentContextPayload(&payload)
	return normalized, nil
}

// PromptSection implements the legacy workspace-scoped prompt provider seam.
func (s *Service) PromptSection(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	payload, err := s.ContextForStartup(ctx, session.StartupPromptContext{
		WorkspaceID: workspaceID(workspace),
		Workspace:   workspaceRoot(workspace),
		SessionType: session.SessionTypeUser,
	}, aghconfig.AgentDef{}, workspace)
	if err != nil {
		return "", err
	}
	return RenderPrompt(&payload)
}

// PromptStartupSection renders the startup prompt section with full startup metadata.
func (s *Service) PromptStartupSection(
	ctx context.Context,
	startup session.StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	payload, err := s.ContextForStartup(ctx, startup, agent, workspace)
	if err != nil {
		return "", err
	}
	return RenderPrompt(&payload)
}

// Augment prefixes a live prompt with fresh situation context.
func (s *Service) Augment(
	ctx context.Context,
	sess *session.Session,
	message string,
) (string, error) {
	if s == nil || sess == nil {
		return message, nil
	}
	info := sess.Info()
	payload, err := s.ContextForSession(ctx, info)
	if err != nil {
		return "", err
	}
	sections, err := renderSections(&payload)
	if err != nil {
		return "", err
	}
	sections = s.promptSections.compact(info, sections)
	rendered, err := renderPromptFromSections(sections)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(rendered) == "" {
		return message, nil
	}
	if strings.TrimSpace(message) == "" {
		return rendered, nil
	}
	return rendered + "\n\n" + message, nil
}
