package core

import (
	"context"

	"fmt"

	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (h *BaseHandlers) resolveAuthoredAgentTarget(
	ctx context.Context,
	workspaceRef string,
	agentName string,
) (authoredAgentTarget, error) {
	name := strings.TrimSpace(agentName)
	if name == "" {
		return authoredAgentTarget{}, newAuthoredValidationError("agent_name is required")
	}
	ref := strings.TrimSpace(workspaceRef)
	if ref == "" {
		return authoredAgentTarget{}, newAuthoredValidationError("workspace_id is required")
	}
	if h.Workspaces == nil {
		return authoredAgentTarget{}, workspacepkg.ErrWorkspaceResolverUnavailable
	}
	resolved, err := h.Workspaces.Resolve(ctx, ref)
	if err != nil {
		return authoredAgentTarget{}, err
	}
	root := strings.TrimSpace(resolved.RootDir)
	if root == "" {
		return authoredAgentTarget{}, workspacepkg.ErrWorkspaceRootMissing
	}
	return authoredAgentTarget{
		workspaceID:        strings.TrimSpace(resolved.WorkspaceID),
		sessionWorkspaceID: strings.TrimSpace(resolved.ID),
		workspaceRoot:      root,
		agentName:          name,
		agentPath:          authoredAgentPath(&resolved, name),
		soulConfig:         resolved.Config.Agents.Soul,
		heartbeatConfig:    resolved.Config.Agents.Heartbeat,
	}.withAgentArtifacts(h.AgentCatalog, name, &resolved), nil
}

func (t authoredAgentTarget) withAgentArtifacts(
	catalog AgentCatalog,
	agentName string,
	resolved *workspacepkg.ResolvedWorkspace,
) authoredAgentTarget {
	if catalog == nil {
		return t
	}
	resolver, ok := catalog.(session.AgentArtifactResolver)
	if !ok {
		return t
	}
	artifacts, err := resolver.ResolveAgentArtifacts(agentName, resolved)
	if err != nil {
		return t
	}
	if sourcePath := strings.TrimSpace(artifacts.Agent.SourcePath); sourcePath != "" {
		t.agentPath = sourcePath
	}
	t.packageOwned = artifacts.PackageOwned
	t.soulSourcePath = strings.TrimSpace(artifacts.SoulSourcePath)
	t.soulBody = artifacts.SoulBody
	t.heartbeatSourcePath = strings.TrimSpace(artifacts.HeartbeatSourcePath)
	t.heartbeatBody = artifacts.HeartbeatBody
	return t
}

func (t authoredAgentTarget) soulAuthoringTarget() soul.AuthoringTarget {
	return soul.AuthoringTarget{
		WorkspaceID:   t.storageWorkspaceID(),
		WorkspaceRoot: authoredContextSourceRoot(t.workspaceRoot, t.agentPath),
		AgentName:     t.agentName,
		AgentPath:     t.agentPath,
		Config:        t.soulConfig,
		ConfigSource:  "workspace",
	}
}

func (t authoredAgentTarget) heartbeatAuthoringTarget() heartbeat.AuthoringTarget {
	return heartbeat.AuthoringTarget{
		WorkspaceID:   t.storageWorkspaceID(),
		WorkspaceRoot: authoredContextSourceRoot(t.workspaceRoot, t.agentPath),
		AgentName:     t.agentName,
		AgentPath:     t.agentPath,
		Config:        t.heartbeatConfig,
	}
}

func (t authoredAgentTarget) storageWorkspaceID() string {
	if id := strings.TrimSpace(t.sessionWorkspaceID); id != "" {
		return id
	}
	return strings.TrimSpace(t.workspaceID)
}

func authoredContextSourceRoot(workspaceRoot string, agentPath string) string {
	root := strings.TrimSpace(workspaceRoot)
	source := strings.TrimSpace(agentPath)
	if source == "" || !filepath.IsAbs(source) || pathWithinRoot(root, source) {
		return root
	}
	if derived := trustedRootFromAgentSourcePath(source); derived != "" {
		return derived
	}
	return root
}

func trustedRootFromAgentSourcePath(agentPath string) string {
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

func pathWithinRoot(root string, sourcePath string) bool {
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

func (t authoredAgentTarget) soulConfigProvenance() soul.ConfigProvenance {
	provenance, err := soul.NewConfigProvenance(t.soulConfig, "workspace")
	if err != nil {
		return soul.ConfigProvenance{}
	}
	return provenance
}

func (t authoredAgentTarget) resolvePackageOwnedSoul(
	ctx context.Context,
	body *string,
) (soul.ResolvedSoul, error) {
	sourcePath := t.packageOwnedSoulSourcePath()
	if body != nil {
		resolved, err := soul.Parse(ctx, soul.ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: t.workspaceRoot,
			Content:       []byte(*body),
			Config:        t.soulConfig,
		})
		if err != nil {
			return resolved, err
		}
		return resolved, nil
	}
	if strings.TrimSpace(t.soulBody) == "" {
		return soul.Empty(t.soulConfig, sourcePath)
	}
	resolved, err := soul.Parse(ctx, soul.ParseRequest{
		SourcePath:    sourcePath,
		WorkspaceRoot: t.workspaceRoot,
		Content:       []byte(t.soulBody),
		Config:        t.soulConfig,
	})
	if err != nil {
		return resolved, err
	}
	return resolved, nil
}

func (t authoredAgentTarget) resolvePackageOwnedHeartbeat(
	ctx context.Context,
	body *string,
) (heartbeat.ResolvedPolicy, error) {
	sourcePath := t.packageOwnedHeartbeatSourcePath()
	if body != nil {
		policy, err := heartbeat.Parse(ctx, heartbeat.ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: t.workspaceRoot,
			Content:       []byte(*body),
			Config:        t.heartbeatConfig,
		})
		if err != nil {
			return policy, err
		}
		return policy, nil
	}
	if strings.TrimSpace(t.heartbeatBody) == "" {
		return heartbeat.Empty(t.heartbeatConfig, sourcePath)
	}
	policy, err := heartbeat.Parse(ctx, heartbeat.ParseRequest{
		SourcePath:    sourcePath,
		WorkspaceRoot: t.workspaceRoot,
		Content:       []byte(t.heartbeatBody),
		Config:        t.heartbeatConfig,
	})
	if err != nil {
		return policy, err
	}
	return policy, nil
}

func (t authoredAgentTarget) rejectPackageOwnedSoulMutation() error {
	if !t.packageOwned {
		return nil
	}
	return fmt.Errorf("%w: package-owned SOUL.md is read-only", soul.ErrAuthoringConflict)
}

func (t authoredAgentTarget) rejectPackageOwnedHeartbeatMutation() error {
	if !t.packageOwned {
		return nil
	}
	return fmt.Errorf("%w: package-owned HEARTBEAT.md is read-only", heartbeat.ErrAuthoringConflict)
}

func (t authoredAgentTarget) packageOwnedSoulSourcePath() string {
	if sourcePath := strings.TrimSpace(t.soulSourcePath); sourcePath != "" {
		return sourcePath
	}
	return authoredSidecarPath(t.agentPath, soul.FileName)
}

func (t authoredAgentTarget) packageOwnedHeartbeatSourcePath() string {
	if sourcePath := strings.TrimSpace(t.heartbeatSourcePath); sourcePath != "" {
		return sourcePath
	}
	return authoredSidecarPath(t.agentPath, heartbeat.FileName)
}
