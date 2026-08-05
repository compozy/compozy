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

type sessionCommandService struct {
	registry          sessionCommandSkills
	workspaceResolver func() promptSkillsWorkspaceResolver
}

var _ session.CommandService = (*sessionCommandService)(nil)

func newSessionCommandService(
	registry sessionCommandSkills,
	workspaceResolver func() promptSkillsWorkspaceResolver,
) *sessionCommandService {
	return &sessionCommandService{registry: registry, workspaceResolver: workspaceResolver}
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
		candidate := skillsByCommandID[invocation.Ref.CommandID]
		if candidate == nil || !sameSkillSource(invocation.Ref.Source, candidate) {
			return "", fmt.Errorf("%w: %s", commandpkg.ErrUnavailable, invocation.Token)
		}
		content, loadErr := s.registry.LoadContent(ctx, candidate)
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
) (commandpkg.Catalog, map[string]*skillspkg.Skill, error) {
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

func (s *sessionCommandService) commandSkillCandidates(
	ctx context.Context,
	info *session.Info,
	agent compozyconfig.AgentDef,
) ([]skillspkg.CommandCandidate, error) {
	workspace, err := resolvePromptSkillsWorkspace(
		ctx,
		s.workspaceResolver,
		info.WorkspaceID,
		info.Workspace,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve command workspace: %w", err)
	}
	hasAgentSnapshot := strings.TrimSpace(agent.Name) != ""
	agent.Name = firstTrimmed(agent.Name, info.AgentName)
	var candidates []skillspkg.CommandCandidate
	if hasAgentSnapshot && strings.TrimSpace(agent.SourcePath) == "" {
		candidates, err = s.registry.CommandCandidatesForAgentDefSession(ctx, workspace, agent, info.ID)
	} else {
		candidates, err = s.registry.CommandCandidatesForAgentSession(ctx, workspace, agent.Name, info.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("daemon: project command skills: %w", err)
	}
	return candidates, nil
}

func projectCommandSkillCandidates(
	candidates []skillspkg.CommandCandidate,
) ([]commandpkg.SkillSpec, map[string]*skillspkg.Skill, error) {
	skillSpecs := make([]commandpkg.SkillSpec, 0, len(candidates))
	skillsByCommandID := make(map[string]*skillspkg.Skill, len(candidates))
	for _, candidate := range candidates {
		if candidate.Skill == nil {
			continue
		}
		spec := commandpkg.SkillSpec{
			Name:        candidate.Skill.Meta.Name,
			Description: candidate.Skill.Meta.Description,
			Source: commandpkg.Source{
				Kind:  candidate.SourceKind,
				ID:    candidate.SourceID,
				Key:   candidate.SourceKey,
				Scope: candidate.Scope,
			},
			Available: candidate.Available,
			Qualified: candidate.Qualified,
		}
		descriptor, descriptorErr := commandpkg.BuildCatalog(nil, nil, []commandpkg.SkillSpec{spec})
		if descriptorErr != nil {
			return nil, nil, descriptorErr
		}
		if len(descriptor.Commands) == 1 {
			skillsByCommandID[descriptor.Commands[0].ID] = candidate.Skill
		}
		skillSpecs = append(skillSpecs, spec)
	}
	return skillSpecs, skillsByCommandID, nil
}

func availableCommandSkills(
	catalog commandpkg.Catalog,
	skillsByCommandID map[string]*skillspkg.Skill,
) map[string]*skillspkg.Skill {
	availableSkills := make(map[string]*skillspkg.Skill, len(skillsByCommandID))
	for _, descriptor := range catalog.Commands {
		if descriptor.Skill == nil {
			continue
		}
		if skill := skillsByCommandID[descriptor.ID]; skill != nil {
			availableSkills[descriptor.ID] = skill
		}
	}
	return availableSkills
}

func sameSkillSource(source commandpkg.Source, skill *skillspkg.Skill) bool {
	if skill == nil {
		return false
	}
	expected := skillspkg.CommandSourceForSkill(skill)
	return commandpkg.Slug(source.Kind) == commandpkg.Slug(expected.Kind) &&
		commandpkg.Slug(source.ID) == commandpkg.Slug(expected.ID) &&
		strings.TrimSpace(source.Key) == expected.Key &&
		strings.TrimSpace(source.Scope) == expected.Scope
}

func firstCriticalSkillWarning(warnings []skillspkg.Warning) *skillspkg.Warning {
	for index := range warnings {
		if warnings[index].Severity == skillspkg.SeverityCritical {
			return &warnings[index]
		}
	}
	return nil
}
