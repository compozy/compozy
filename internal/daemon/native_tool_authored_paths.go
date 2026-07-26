package daemon

import (
	"context"

	"errors"
	"fmt"

	"path/filepath"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/compozy/agh/internal/skills"

	toolspkg "github.com/compozy/agh/internal/tools"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (n *daemonNativeTools) skillsFor(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	workspaceID string,
) ([]*skills.Skill, error) {
	if n.deps.Skills == nil {
		return nil, errors.New("daemon: skills registry is required")
	}
	workspaceID, err := nativeCallerWorkspaceInput(id, "workspace_id", workspaceID, scope)
	if err != nil {
		return nil, err
	}
	agentName := strings.TrimSpace(scope.AgentName)
	agentDef, hasSessionAgent, err := n.sessionAgentDefinition(scope.SessionID)
	if err != nil {
		return nil, err
	}
	if workspaceID == "" {
		if hasSessionAgent {
			return n.skillsForSessionAgent(ctx, nil, agentDef, scope.SessionID)
		}
		if agentName != "" {
			return n.deps.Skills.ForAgentSession(ctx, nil, agentName, scope.SessionID)
		}
		return n.deps.Skills.ForWorkspace(ctx, nil)
	}
	if n.deps.WorkspaceResolver == nil {
		return nil, errors.New("daemon: workspace resolver is required for workspace skills")
	}
	resolved, err := n.deps.WorkspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if hasSessionAgent {
		return n.skillsForSessionAgent(ctx, &resolved, agentDef, scope.SessionID)
	}
	if agentName != "" {
		return n.deps.Skills.ForAgentSession(ctx, &resolved, agentName, scope.SessionID)
	}
	return n.deps.Skills.ForWorkspace(ctx, &resolved)
}

func (n *daemonNativeTools) skillsForSessionAgent(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
	agent aghconfig.AgentDef,
	sessionID string,
) ([]*skills.Skill, error) {
	if strings.TrimSpace(agent.SourcePath) == "" {
		return n.deps.Skills.ForAgentDefSession(ctx, workspace, agent, sessionID)
	}
	return n.deps.Skills.ForAgentSession(ctx, workspace, agent.Name, sessionID)
}

type sessionAgentDefinitionReader interface {
	SessionAgentDefinition(id string) (aghconfig.AgentDef, bool)
}

func (n *daemonNativeTools) sessionAgentDefinition(sessionID string) (aghconfig.AgentDef, bool, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return aghconfig.AgentDef{}, false, nil
	}
	reader, ok := n.deps.Sessions.(sessionAgentDefinitionReader)
	if !ok || reader == nil {
		return aghconfig.AgentDef{}, false, errors.New(
			"daemon: concrete session agent definition is unavailable",
		)
	}
	agent, ok := reader.SessionAgentDefinition(trimmedID)
	if !ok {
		return aghconfig.AgentDef{}, false, fmt.Errorf(
			"daemon: session %q agent definition is unavailable",
			trimmedID,
		)
	}
	return agent, true, nil
}

func (n *daemonNativeTools) resolveSkill(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	workspaceID string,
	name string,
) (*skills.Skill, error) {
	trimmedName := strings.TrimSpace(name)
	workspaceID, err := nativeCallerWorkspaceInput(id, "workspace_id", workspaceID, scope)
	if err != nil {
		return nil, err
	}
	skillList, err := n.skillsFor(ctx, scope, id, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, skill := range skillList {
		if skill != nil && skill.Meta.Name == trimmedName {
			return skill, nil
		}
	}
	return nil, fmt.Errorf("daemon: skill %q not found", trimmedName)
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
	heartbeatConfig aghconfig.HeartbeatConfig
}

func (n *daemonNativeTools) authoredAgentTarget(
	ctx context.Context,
	toolID toolspkg.ToolID,
	workspaceRef string,
	agentName string,
) (nativeAuthoredAgentTarget, error) {
	workspaceID, err := requiredNativeString(toolID, "workspace_id", workspaceRef)
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
	if filepath.Base(agentsDir) != aghconfig.AgentsDirName {
		return ""
	}
	root := filepath.Dir(agentsDir)
	if filepath.Base(root) == aghconfig.DirName {
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
		return filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, name, "AGENT.md")
	}
	return ""
}
