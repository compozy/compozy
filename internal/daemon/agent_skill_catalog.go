package daemon

import (
	"context"
	"errors"
	"fmt"

	"strings"

	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type resourceAgentCatalog struct {
	catalog          *resourceCatalog[aghconfig.AgentDef]
	soulCatalog      *resourceCatalog[soul.ResourceSpec]
	heartbeatCatalog *resourceCatalog[heartbeat.ResourceSpec]
}

var _ session.AgentArtifactResolver = (*resourceAgentCatalog)(nil)
var _ heartbeat.PolicyResolver = (*resourceAgentCatalog)(nil)

type agentSidecarCatalogs struct {
	soul      *resourceCatalog[soul.ResourceSpec]
	heartbeat *resourceCatalog[heartbeat.ResourceSpec]
}

func agentCatalogDependency(
	catalog *resourceCatalog[aghconfig.AgentDef],
	sidecars ...agentSidecarCatalogs,
) *resourceAgentCatalog {
	if catalog == nil {
		return nil
	}
	dependency := &resourceAgentCatalog{catalog: catalog}
	if len(sidecars) > 0 {
		dependency.soulCatalog = sidecars[0].soul
		dependency.heartbeatCatalog = sidecars[0].heartbeat
	}
	return dependency
}

func (c *resourceAgentCatalog) ResolveAgent(
	name string,
	resolved *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		return aghconfig.AgentDef{}, errors.New("session: agent name is required")
	}
	if c == nil || c.catalog == nil {
		return resolveAgentFromWorkspaceSnapshot(target, resolved)
	}
	if record, ok := c.lookupAgentRecord(target, resolved); ok {
		return cloneAgentDef(record.Spec), nil
	}
	if resolved != nil {
		return resolveAgentFromWorkspaceSnapshot(target, resolved)
	}
	return aghconfig.AgentDef{}, fmt.Errorf("%w: %s", workspacepkg.ErrAgentNotAvailable, target)
}

func (c *resourceAgentCatalog) lookupAgentRecord(
	target string,
	resolved *workspacepkg.ResolvedWorkspace,
) (resources.Record[aghconfig.AgentDef], bool) {
	if c == nil || c.catalog == nil {
		return resources.Record[aghconfig.AgentDef]{}, false
	}

	workspaceID := ""
	if resolved != nil {
		workspaceID = strings.TrimSpace(resolved.ID)
	}
	var (
		globalKey      string
		globalAgent    resources.Record[aghconfig.AgentDef]
		globalFound    bool
		workspaceKey   string
		workspaceAgent resources.Record[aghconfig.AgentDef]
		workspaceFound bool
	)

	c.catalog.mu.RLock()
	defer c.catalog.mu.RUnlock()
	for _, record := range c.catalog.records {
		if strings.TrimSpace(record.Spec.Name) != target {
			continue
		}

		sortKey := agentRecordSortKey(record)
		switch record.Scope.Kind.Normalize() {
		case resources.ResourceScopeKindGlobal:
			if !globalFound || sortKey > globalKey {
				globalKey = sortKey
				globalAgent = record
				globalFound = true
			}
		case resources.ResourceScopeKindWorkspace:
			if workspaceID == "" || strings.TrimSpace(record.Scope.ID) != workspaceID {
				continue
			}
			if !workspaceFound || sortKey > workspaceKey {
				workspaceKey = sortKey
				workspaceAgent = record
				workspaceFound = true
			}
		}
	}

	if workspaceFound {
		return c.catalog.cloneRecord(workspaceAgent), true
	}
	if globalFound {
		return c.catalog.cloneRecord(globalAgent), true
	}
	return resources.Record[aghconfig.AgentDef]{}, false
}

func resolveAgentFromWorkspaceSnapshot(
	target string,
	resolved *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	if resolved == nil {
		return aghconfig.AgentDef{}, errors.New("session: resolved workspace is required")
	}
	for _, agent := range resolved.Agents {
		if strings.TrimSpace(agent.Name) == target {
			return cloneAgentDef(agent), nil
		}
	}
	return aghconfig.AgentDef{}, fmt.Errorf("%w: %s", workspacepkg.ErrAgentNotAvailable, target)
}

func (c *resourceAgentCatalog) ResolveAgentArtifacts(
	name string,
	resolved *workspacepkg.ResolvedWorkspace,
) (session.AgentArtifacts, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		return session.AgentArtifacts{}, errors.New("session: agent name is required")
	}
	if c == nil || c.catalog == nil {
		agent, err := resolveAgentFromWorkspaceSnapshot(target, resolved)
		if err != nil {
			return session.AgentArtifacts{}, err
		}
		return session.AgentArtifacts{Agent: agent}, nil
	}
	record, ok := c.lookupAgentRecord(target, resolved)
	if !ok {
		agent, err := resolveAgentFromWorkspaceSnapshot(target, resolved)
		if err != nil {
			return session.AgentArtifacts{}, err
		}
		return session.AgentArtifacts{Agent: agent}, nil
	}
	artifacts := session.AgentArtifacts{
		Agent:        cloneAgentDef(record.Spec),
		ResourceID:   strings.TrimSpace(record.ID),
		OwnerKind:    string(record.Owner.Kind.Normalize()),
		OwnerID:      strings.TrimSpace(record.Owner.ID),
		Scope:        record.Scope.Normalize(),
		PackageOwned: record.Owner.Kind.Normalize() == bundlepkg.BundleActivationOwnerKind,
	}
	if c.soulCatalog != nil {
		if spec, ok := c.lookupSoulForAgent(record); ok {
			artifacts.SoulSourcePath = spec.SourcePath
			artifacts.SoulBody = spec.Body
		}
	}
	if c.heartbeatCatalog != nil {
		if spec, ok := c.lookupHeartbeatForAgent(record); ok {
			artifacts.HeartbeatSourcePath = spec.SourcePath
			artifacts.HeartbeatBody = spec.Body
		}
	}
	return artifacts, nil
}

func (c *resourceAgentCatalog) ResolveHeartbeatPolicy(
	ctx context.Context,
	target heartbeat.AuthoringTarget,
) (heartbeat.ResolvedPolicy, bool, error) {
	if ctx == nil {
		return heartbeat.ResolvedPolicy{}, false, errors.New("daemon: heartbeat policy context is required")
	}
	if err := ctx.Err(); err != nil {
		return heartbeat.ResolvedPolicy{}, false, err
	}
	config := target.Config
	if config == (aghconfig.HeartbeatConfig{}) {
		config = aghconfig.DefaultHeartbeatConfig()
	}
	workspace := &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      strings.TrimSpace(target.WorkspaceID),
			RootDir: strings.TrimSpace(target.WorkspaceRoot),
		},
		Config: aghconfig.Config{
			Agents: aghconfig.AgentsConfig{Heartbeat: config},
		},
	}
	artifacts, err := c.ResolveAgentArtifacts(target.AgentName, workspace)
	if err != nil {
		if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			return heartbeat.ResolvedPolicy{}, false, nil
		}
		return heartbeat.ResolvedPolicy{}, false, err
	}
	if !artifacts.PackageOwned {
		return heartbeat.ResolvedPolicy{}, false, nil
	}
	sourcePath := strings.TrimSpace(artifacts.HeartbeatSourcePath)
	if strings.TrimSpace(artifacts.HeartbeatBody) == "" {
		policy, emptyErr := heartbeat.Empty(config, sourcePath)
		return policy, true, emptyErr
	}
	policy, parseErr := heartbeat.Parse(ctx, heartbeat.ParseRequest{
		SourcePath:    sourcePath,
		WorkspaceRoot: strings.TrimSpace(target.WorkspaceRoot),
		Content:       []byte(artifacts.HeartbeatBody),
		Config:        config,
	})
	return policy, true, parseErr
}

func (c *resourceAgentCatalog) lookupSoulForAgent(
	agent resources.Record[aghconfig.AgentDef],
) (soul.ResourceSpec, bool) {
	if c == nil || c.soulCatalog == nil {
		return soul.ResourceSpec{}, false
	}
	var (
		bestKey string
		best    soul.ResourceSpec
		found   bool
	)
	for _, record := range c.soulCatalog.Snapshot() {
		if !sidecarRecordMatchesAgent(record.Scope, record.Owner, record.Spec.AgentResourceID, agent) {
			continue
		}
		sortKey := sidecarRecordSortKey(record)
		if !found || sortKey > bestKey {
			bestKey = sortKey
			best = cloneSoulResourceSpec(record.Spec)
			found = true
		}
	}
	return best, found
}

func (c *resourceAgentCatalog) lookupHeartbeatForAgent(
	agent resources.Record[aghconfig.AgentDef],
) (heartbeat.ResourceSpec, bool) {
	if c == nil || c.heartbeatCatalog == nil {
		return heartbeat.ResourceSpec{}, false
	}
	var (
		bestKey string
		best    heartbeat.ResourceSpec
		found   bool
	)
	for _, record := range c.heartbeatCatalog.Snapshot() {
		if !sidecarRecordMatchesAgent(record.Scope, record.Owner, record.Spec.AgentResourceID, agent) {
			continue
		}
		sortKey := sidecarRecordSortKey(record)
		if !found || sortKey > bestKey {
			bestKey = sortKey
			best = cloneHeartbeatResourceSpec(record.Spec)
			found = true
		}
	}
	return best, found
}

func sidecarRecordMatchesAgent(
	scope resources.ResourceScope,
	owner resources.ResourceOwner,
	agentResourceID string,
	agent resources.Record[aghconfig.AgentDef],
) bool {
	return strings.TrimSpace(agentResourceID) == strings.TrimSpace(agent.ID) &&
		scope.Normalize() == agent.Scope.Normalize() &&
		owner.Normalize() == agent.Owner.Normalize()
}
