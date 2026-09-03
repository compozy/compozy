package daemon

import (
	"context"

	"errors"
	"fmt"

	"path/filepath"
	"slices"

	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"

	"github.com/compozy/compozy/internal/skills"

	toolspkg "github.com/compozy/compozy/internal/tools"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (n *daemonNativeTools) skillsFor(
	ctx context.Context,
	scope toolspkg.Scope,
	workspaceID string,
) ([]*skills.Skill, error) {
	if n.deps.Skills == nil {
		return nil, errors.New("daemon: skills registry is required")
	}
	workspaceID = nativeCallerWorkspaceInput(workspaceID, scope)
	agentName := strings.TrimSpace(scope.AgentName)
	agentDef, hasSessionAgent, err := n.sessionAgentDefinition(scope.SessionID)
	if err != nil {
		return nil, err
	}
	resolved, err := n.nativeSkillWorkspace(ctx, scope.ProfileID, workspaceID)
	if err != nil {
		return nil, err
	}
	if workspaceID == "" {
		if hasSessionAgent {
			return n.skillsForSessionAgent(ctx, resolved, agentDef, scope.SessionID)
		}
		if agentName != "" {
			return n.deps.Skills.ForAgentSession(ctx, resolved, agentName, scope.SessionID)
		}
		return n.deps.Skills.ForWorkspace(ctx, resolved)
	}
	if hasSessionAgent {
		return n.skillsForSessionAgent(ctx, resolved, agentDef, scope.SessionID)
	}
	if agentName != "" {
		return n.deps.Skills.ForAgentSession(ctx, resolved, agentName, scope.SessionID)
	}
	return n.deps.Skills.ForWorkspace(ctx, resolved)
}

func (n *daemonNativeTools) nativeSkillWorkspace(
	ctx context.Context,
	profileID string,
	workspaceID string,
) (*workspacepkg.ResolvedWorkspace, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" && workspaceID == "" {
		return nil, nil
	}
	profileName := compozyconfig.DefaultProfileDirName
	if n.deps.Profiles != nil {
		resolvedName, err := n.deps.Profiles.ProfileName(ctx, profileID)
		if err != nil {
			return nil, fmt.Errorf("daemon: resolve native skill profile: %w", err)
		}
		profileName = strings.TrimSpace(resolvedName)
	}
	if workspaceID != "" {
		if n.deps.WorkspaceResolver == nil {
			return nil, errors.New("daemon: workspace resolver is required for workspace skills")
		}
		profileResolver, ok := n.deps.WorkspaceResolver.(workspacepkg.ProfileRuntimeResolver)
		if !ok {
			return nil, errors.New("daemon: profile-aware workspace resolver is required for workspace skills")
		}
		resolved, err := profileResolver.ResolveForProfile(ctx, workspaceID, profileName)
		if err != nil {
			return nil, err
		}
		return &resolved, nil
	}
	config, err := compozyconfig.LoadForHome(n.deps.HomePaths, compozyconfig.WithProfile(profileName))
	if err != nil {
		return nil, fmt.Errorf("daemon: load native skill profile config: %w", err)
	}
	return &workspacepkg.ResolvedWorkspace{
		ProfileID: profileID, ProfileName: profileName,
		ProfileRoot: filepath.Join(n.deps.HomePaths.ProfilesDir, profileName), Config: config,
	}, nil
}

func (n *daemonNativeTools) skillsForSessionAgent(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
	agent compozyconfig.AgentDef,
	sessionID string,
) ([]*skills.Skill, error) {
	return resolveSessionAgentProjection(sessionAgentProjection[[]*skills.Skill]{
		agent:                 agent,
		hasConcreteDefinition: true,
		workspace:             workspace,
		agentResolver:         func() session.AgentResolver { return n.deps.AgentResolver },
		byName: func(agentName string) ([]*skills.Skill, error) {
			return n.deps.Skills.ForAgentSession(ctx, workspace, agentName, sessionID)
		},
		byDefinition: func(resolvedAgent compozyconfig.AgentDef) ([]*skills.Skill, error) {
			return n.deps.Skills.ForAgentDefSession(ctx, workspace, resolvedAgent, sessionID)
		},
	})
}

type sessionAgentDefinitionReader interface {
	SessionAgentDefinition(id string) (compozyconfig.AgentDef, bool)
}

func (n *daemonNativeTools) sessionAgentDefinition(sessionID string) (compozyconfig.AgentDef, bool, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return compozyconfig.AgentDef{}, false, nil
	}
	reader, ok := n.deps.Sessions.(sessionAgentDefinitionReader)
	if !ok || reader == nil {
		return compozyconfig.AgentDef{}, false, errors.New(
			"daemon: concrete session agent definition is unavailable",
		)
	}
	agent, ok := reader.SessionAgentDefinition(trimmedID)
	if !ok {
		return compozyconfig.AgentDef{}, false, fmt.Errorf(
			"daemon: session %q agent definition is unavailable",
			trimmedID,
		)
	}
	return agent, true, nil
}

func (n *daemonNativeTools) resolveSkill(
	ctx context.Context,
	scope toolspkg.Scope,
	workspaceID string,
	name string,
) (*skills.Skill, error) {
	trimmedName := strings.TrimSpace(name)
	workspaceID = nativeCallerWorkspaceInput(workspaceID, scope)
	skillList, err := n.skillsFor(ctx, scope, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, skill := range skillList {
		if skill != nil && skill.Meta.Name == trimmedName {
			return skill, nil
		}
	}
	definitionErr, found, err := n.invalidSkillDefinition(ctx, scope, workspaceID, trimmedName)
	if err != nil {
		return nil, err
	}
	if found {
		return nil, skillViewDefinitionError(toolspkg.ToolIDSkillView, definitionErr)
	}
	return nil, nativeCommandNotFoundError(
		toolspkg.ToolIDSkillView,
		fmt.Sprintf("skill %q not found", trimmedName),
	)
}

func (n *daemonNativeTools) invalidSkillDefinition(
	ctx context.Context,
	scope toolspkg.Scope,
	workspaceID string,
	name string,
) (error, bool, error) {
	registry, ok := n.deps.Skills.(nativeSkillDiagnosticsRegistry)
	if !ok {
		return nil, false, nil
	}
	resolved, err := n.nativeSkillWorkspace(ctx, scope.ProfileID, workspaceID)
	if err != nil {
		return nil, false, err
	}
	diagnostics, err := registry.SkillDiagnostics(ctx, resolved, strings.TrimSpace(scope.AgentName))
	if err != nil {
		return nil, false, err
	}
	index := slices.IndexFunc(diagnostics, func(diagnostic skills.SkillDiagnostic) bool {
		return diagnostic.Name == name && diagnostic.Failure != nil &&
			diagnostic.Failure.Code == skills.SkillFailureCodeDefinitionInvalid
	})
	if index < 0 {
		return nil, false, nil
	}
	diagnostic := diagnostics[index]
	message := strings.TrimSpace(diagnostic.Failure.Message)
	if message == "" {
		message = fmt.Sprintf("skills: invalid definition %q", diagnostic.Path)
	}
	return errors.New(message), true, nil
}

func (n *daemonNativeTools) workspaceID(ctx context.Context, ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", nil
	}
	if n.deps.Workspaces == nil {
		return trimmed, nil
	}
	workspace, err := n.deps.Workspaces.Get(ctx, trimmed)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(workspace.ID), nil
}

type nativeAuthoredAgentTarget struct {
	workspaceID     string
	workspaceRoot   string
	agentName       string
	agentPath       string
	heartbeatConfig compozyconfig.HeartbeatConfig
}

func (n *daemonNativeTools) authoredAgentTarget(
	ctx context.Context,
	toolID toolspkg.ToolID,
	workspaceRef string,
	agentName string,
) (nativeAuthoredAgentTarget, error) {
	workspaceID, err := requiredNativeString(toolID, nativeWorkspaceInputKey, workspaceRef)
	if err != nil {
		return nativeAuthoredAgentTarget{}, err
	}
	name, err := requiredNativeString(toolID, "agent_name", agentName)
	if err != nil {
		return nativeAuthoredAgentTarget{}, err
	}
	if n.deps.WorkspaceResolver == nil {
		return nativeAuthoredAgentTarget{}, errors.New("daemon: workspace resolver is required")
	}
	resolved, err := n.deps.WorkspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return nativeAuthoredAgentTarget{}, err
	}
	root := strings.TrimSpace(resolved.RootDir)
	if root == "" {
		return nativeAuthoredAgentTarget{}, workspacepkg.ErrWorkspaceRootMissing
	}
	resolvedWorkspaceID, err := nativeResolvedNetworkWorkspaceID(&resolved)
	if err != nil {
		return nativeAuthoredAgentTarget{}, err
	}
	return nativeAuthoredAgentTarget{
		workspaceID:     resolvedWorkspaceID,
		workspaceRoot:   root,
		agentName:       name,
		agentPath:       nativeAuthoredAgentPath(&resolved, name),
		heartbeatConfig: resolved.Config.Agents.Heartbeat,
	}, nil
}

func (t nativeAuthoredAgentTarget) heartbeatAuthoringTarget() heartbeat.AuthoringTarget {
	return heartbeat.AuthoringTarget{
		WorkspaceID:   t.workspaceID,
		WorkspaceRoot: nativeAuthoredSourceRoot(t.workspaceRoot, t.agentPath),
		AgentName:     t.agentName,
		AgentPath:     t.agentPath,
		Config:        t.heartbeatConfig,
	}
}

func nativeAuthoredSourceRoot(workspaceRoot string, agentPath string) string {
	root := strings.TrimSpace(workspaceRoot)
	source := strings.TrimSpace(agentPath)
	if source == "" || !filepath.IsAbs(source) || nativePathWithinRoot(root, source) {
		return root
	}
	if derived := nativeTrustedRootFromAgentSourcePath(source); derived != "" {
		return derived
	}
	return root
}

func nativeTrustedRootFromAgentSourcePath(agentPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(agentPath))
	if !strings.EqualFold(filepath.Base(cleaned), "AGENT.md") {
		return ""
	}
	agentDir := filepath.Dir(cleaned)
	agentsDir := filepath.Dir(agentDir)
	if filepath.Base(agentsDir) != compozyconfig.AgentsDirName {
		return ""
	}
	root := filepath.Dir(agentsDir)
	if filepath.Base(root) == compozyconfig.DirName {
		return filepath.Dir(root)
	}
	return root
}

func nativePathWithinRoot(root string, sourcePath string) bool {
	trimmedRoot := strings.TrimSpace(root)
	trimmedSource := strings.TrimSpace(sourcePath)
	if trimmedRoot == "" || trimmedSource == "" {
		return false
	}
	absRoot, err := filepath.Abs(filepath.Clean(trimmedRoot))
	if err != nil {
		return false
	}
	sourceForRoot := filepath.Clean(trimmedSource)
	if !filepath.IsAbs(sourceForRoot) {
		sourceForRoot = filepath.Join(absRoot, sourceForRoot)
	}
	absSource, err := filepath.Abs(sourceForRoot)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absSource)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func nativeAuthoredAgentPath(workspace *workspacepkg.ResolvedWorkspace, agentName string) string {
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
		return filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, name, "AGENT.md")
	}
	return ""
}
