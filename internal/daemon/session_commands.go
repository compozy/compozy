package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type sessionCommandSkills interface {
	CommandCandidatesForAgentSession(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
		sessionID string,
	) ([]skillspkg.CommandCandidate, error)
	CommandCandidatesForAgentDefSession(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agent compozyconfig.AgentDef,
		sessionID string,
	) ([]skillspkg.CommandCandidate, error)
	LoadContent(ctx context.Context, skill *skillspkg.Skill) (string, error)
}

// sessionCommandService projects a session's authoritative slash-command catalog (builtins,
// ACP-advertised agent commands, and source-qualified skill commands) and expands invoked skill
// markers into their content, implementing session.CommandService.
type sessionCommandService struct {
	registry          sessionCommandSkills
	agentResolver     func() session.AgentResolver
	workspaceResolver func() promptSkillsWorkspaceResolver
	profileNames      session.ProfileNameResolver
}

var _ session.CommandService = (*sessionCommandService)(nil)

// newSessionCommandService takes agentResolver as a lazy provider — mirroring
// workspaceResolver's shape — because the composition root wires this service before some
// dependencies (e.g. the agent resource catalog) finish booting; evaluating the provider at
// call time instead of capturing its value at construction time avoids permanently binding to a
// not-yet-populated dependency.
func newSessionCommandService(
	registry sessionCommandSkills,
	agentResolver func() session.AgentResolver,
	workspaceResolver func() promptSkillsWorkspaceResolver,
	profileNames session.ProfileNameResolver,
) *sessionCommandService {
	return &sessionCommandService{
		registry:          registry,
		agentResolver:     agentResolver,
		workspaceResolver: workspaceResolver,
		profileNames:      profileNames,
	}
}

func (s *sessionCommandService) Catalog(
	ctx context.Context,
	info *session.Info,
	agent compozyconfig.AgentDef,
) (commandpkg.Catalog, error) {
	catalog, _, err := s.project(ctx, info, agent)
	return catalog, err
}

func (s *sessionCommandService) Expand(
	ctx context.Context,
	info *session.Info,
	agent compozyconfig.AgentDef,
	invocations []commandpkg.Invocation,
	message string,
) (string, error) {
	if len(invocations) == 0 {
		return message, nil
	}
	_, skillsByCommandID, err := s.project(ctx, info, agent)
	if err != nil {
		return "", err
	}
	invoked := make([]commandpkg.InvokedSkill, 0, len(invocations))
	for _, invocation := range invocations {
		candidate, ok := skillsByCommandID[invocation.Ref.CommandID]
		if !ok || !sameSkillSource(invocation.Ref.Source, candidate) {
			return "", fmt.Errorf("%w: %s", commandpkg.ErrUnavailable, invocation.Token)
		}
		content, loadErr := s.registry.LoadContent(ctx, candidate.Skill)
		if loadErr != nil {
			return "", fmt.Errorf("daemon: load invoked skill %q: %w", invocation.Ref.CommandID, loadErr)
		}
		if warning := firstCriticalSkillWarning(skillspkg.VerifyContent(content)); warning != nil {
			return "", fmt.Errorf("daemon: verify invoked skill %q: %s", invocation.Ref.CommandID, warning.Message)
		}
		invoked = append(invoked, commandpkg.InvokedSkill{Ref: invocation.Ref, Content: content})
	}
	block, err := commandpkg.BuildInvokedSkillsBlock(invoked)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(block) == "" {
		return message, nil
	}
	return block + "\n\n" + message, nil
}

func (s *sessionCommandService) project(
	ctx context.Context,
	info *session.Info,
	agent compozyconfig.AgentDef,
) (commandpkg.Catalog, map[string]skillspkg.CommandCandidate, error) {
	if info == nil {
		return commandpkg.Catalog{}, nil, errors.New("daemon: session info is required")
	}
	agents := commandAgentSpecs(info)
	if s == nil || s.registry == nil {
		catalog, err := commandpkg.BuildCatalog(commandpkg.DefaultBuiltins(), agents, nil)
		return catalog, nil, err
	}

	candidates, err := s.commandSkillCandidates(ctx, info, agent)
	if err != nil {
		return commandpkg.Catalog{}, nil, err
	}
	skillSpecs, skillsByCommandID, err := projectCommandSkillCandidates(candidates)
	if err != nil {
		return commandpkg.Catalog{}, nil, err
	}
	catalog, err := commandpkg.BuildCatalog(commandpkg.DefaultBuiltins(), agents, skillSpecs)
	if err != nil {
		return commandpkg.Catalog{}, nil, err
	}
	return catalog, availableCommandSkills(catalog, skillsByCommandID), nil
}

func commandAgentSpecs(info *session.Info) []commandpkg.AgentSpec {
	agents := make([]commandpkg.AgentSpec, 0, len(info.AdvertisedCommands))
	for _, advertised := range info.AdvertisedCommands {
		hint := ""
		if advertised.Input != nil {
			hint = advertised.Input.Hint
		}
		agents = append(agents, commandpkg.AgentSpec{
			Name:        advertised.Name,
			Description: advertised.Description,
			InputHint:   hint,
			SourceID:    info.AgentName,
		})
	}
	return agents
}

// commandSkillCandidates projects the source-qualified skill commands available to one session.
// Concrete package-owned snapshots can be projected directly. Source-backed agents are refreshed
// by name so current authored diagnostics remain authoritative; extension-published agents fall
// back to the daemon catalog only when the name-based registry reports that the agent is absent.
func (s *sessionCommandService) commandSkillCandidates(
	ctx context.Context,
	info *session.Info,
	agent compozyconfig.AgentDef,
) ([]skillspkg.CommandCandidate, error) {
	workspace, err := resolvePromptSkillsWorkspace(
		ctx,
		s.workspaceResolver,
		s.profileNames,
		info.ProfileID,
		info.WorkspaceID,
		info.Workspace,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve command workspace: %w", err)
	}
	hasAgentSnapshot := strings.TrimSpace(agent.Name) != ""
	agent.Name = firstTrimmed(agent.Name, info.AgentName)
	candidates, candidateErr := resolveSessionAgentProjection(sessionAgentProjection[[]skillspkg.CommandCandidate]{
		agent:                 agent,
		hasConcreteDefinition: hasAgentSnapshot,
		workspace:             workspace,
		agentResolver:         s.agentResolver,
		byName: func(agentName string) ([]skillspkg.CommandCandidate, error) {
			return s.registry.CommandCandidatesForAgentSession(ctx, workspace, agentName, info.ID)
		},
		byDefinition: func(resolvedAgent compozyconfig.AgentDef) ([]skillspkg.CommandCandidate, error) {
			return s.registry.CommandCandidatesForAgentDefSession(ctx, workspace, resolvedAgent, info.ID)
		},
	})
	if candidateErr != nil {
		return nil, fmt.Errorf("daemon: project command skills: %w", candidateErr)
	}
	return candidates, nil
}

func projectCommandSkillCandidates(
	candidates []skillspkg.CommandCandidate,
) ([]commandpkg.SkillSpec, map[string]skillspkg.CommandCandidate, error) {
	skillSpecs := make([]commandpkg.SkillSpec, 0, len(candidates))
	skillsByCommandID := make(map[string]skillspkg.CommandCandidate, len(candidates))
	for _, candidate := range candidates {
		if candidate.Skill == nil {
			continue
		}
		spec := commandpkg.SkillSpec{
			Name:        candidate.Skill.Meta.Name,
			Description: candidate.Skill.Meta.Description,
			Source: commandpkg.Source{
				Kind:   candidate.SourceKind,
				ID:     candidate.SourceID,
				Key:    candidate.SourceKey,
				Scope:  candidate.Scope,
				Origin: candidate.Origin,
			},
			Available: candidate.Available,
			Qualified: candidate.Qualified,
		}
		descriptor, descriptorErr := commandpkg.BuildCatalog(nil, nil, []commandpkg.SkillSpec{spec})
		if descriptorErr != nil {
			return nil, nil, descriptorErr
		}
		if len(descriptor.Commands) == 1 {
			skillsByCommandID[descriptor.Commands[0].ID] = candidate
		}
		skillSpecs = append(skillSpecs, spec)
	}
	return skillSpecs, skillsByCommandID, nil
}

func availableCommandSkills(
	catalog commandpkg.Catalog,
	skillsByCommandID map[string]skillspkg.CommandCandidate,
) map[string]skillspkg.CommandCandidate {
	availableSkills := make(map[string]skillspkg.CommandCandidate, len(skillsByCommandID))
	for _, descriptor := range catalog.Commands {
		if descriptor.Skill == nil {
			continue
		}
		if candidate, ok := skillsByCommandID[descriptor.ID]; ok {
			availableSkills[descriptor.ID] = candidate
		}
	}
	return availableSkills
}

func sameSkillSource(source commandpkg.Source, candidate skillspkg.CommandCandidate) bool {
	if candidate.Skill == nil {
		return false
	}
	return commandpkg.Slug(source.Kind) == commandpkg.Slug(candidate.SourceKind) &&
		commandpkg.Slug(source.ID) == commandpkg.Slug(candidate.SourceID) &&
		strings.TrimSpace(source.Key) == candidate.SourceKey &&
		strings.TrimSpace(source.Scope) == candidate.Scope &&
		strings.TrimSpace(source.Origin) == candidate.Origin
}

func firstCriticalSkillWarning(warnings []skillspkg.Warning) *skillspkg.Warning {
	for index := range warnings {
		if warnings[index].Severity == skillspkg.SeverityCritical {
			return &warnings[index]
		}
	}
	return nil
}
