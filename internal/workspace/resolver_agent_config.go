package workspace

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/sandbox"
)

type cachedAgentConfig struct {
	resolved   ResolvedAgentConfig
	snapshots  map[string]filesnap.Snapshot
	lastAccess time.Time
}

type workspaceAgentState struct {
	agents      []compozyconfig.AgentDef
	diagnostics []AgentDiagnostic
	sandbox     sandbox.Resolved
}

// ResolveAgentConfig resolves current configuration and agents without discovering skills.
func (r *Resolver) ResolveAgentConfig(ctx context.Context, ref, profileName string) (ResolvedAgentConfig, error) {
	profileName = strings.TrimSpace(profileName)
	profileID := ""
	if profileName != "" {
		var err error
		profileName, profileID, err = r.resolveProfileIdentity(ctx, profileName)
		if err != nil {
			return ResolvedAgentConfig{}, err
		}
	}
	ws, identity, err := r.resolveRegistration(ctx, ref)
	if err != nil {
		return ResolvedAgentConfig{}, err
	}
	scan, err := r.scanAgentConfig(ctx, ws, profileName)
	if err != nil {
		return ResolvedAgentConfig{}, err
	}
	cacheKey := workspaceProfileCacheKey(ws.ID, profileName)
	now := r.now()
	r.mu.Lock()
	r.evictExpiredLocked(now)
	cached := r.agentConfigCache[cacheKey]
	if cached != nil && sameWorkspaceRuntimeInputs(cached.resolved.Workspace, ws) &&
		filesnap.Equal(cached.snapshots, scan.snapshots) {
		cached.lastAccess = now
		resolved := cloneResolvedAgentConfig(&cached.resolved)
		r.mu.Unlock()
		resolved.Workspace = cloneWorkspace(ws)
		resolved.WorkspaceID, resolved.ProfileID = identity.WorkspaceID, profileID
		return resolved, nil
	}
	agentConfigGeneration := r.agentConfigGeneration
	r.mu.Unlock()
	resolved, err := r.buildResolvedAgentConfig(ctx, ws, scan.agents, profileName)
	if err != nil {
		return ResolvedAgentConfig{}, err
	}
	resolved.WorkspaceID, resolved.ProfileID = identity.WorkspaceID, profileID
	r.mu.Lock()
	r.evictExpiredLocked(now)
	if agentConfigGeneration == r.agentConfigGeneration {
		r.agentConfigCache[cacheKey] = &cachedAgentConfig{
			resolved: cloneResolvedAgentConfig(&resolved), snapshots: cloneSnapshots(scan.snapshots), lastAccess: now,
		}
	}
	r.mu.Unlock()
	return resolved, nil
}

// buildResolvedAgentConfig applies the full resolver's configuration, agent, and sandbox validation without skills.
func (r *Resolver) buildResolvedAgentConfig(
	ctx context.Context,
	ws Workspace,
	agents []agentCandidate,
	profileName string,
) (ResolvedAgentConfig, error) {
	cfg, err := r.loadWorkspaceConfig(ws.RootDir, profileName)
	if err != nil {
		return ResolvedAgentConfig{}, fmt.Errorf("workspace: load config for %q: %w", ws.RootDir, err)
	}
	state, err := buildWorkspaceAgentState(ctx, ws, &cfg, agents)
	if err != nil {
		return ResolvedAgentConfig{}, err
	}
	return ResolvedAgentConfig{
		Workspace: cloneWorkspace(ws), ProfileName: profileName,
		Config: compozyconfig.CloneConfig(&cfg), Agents: cloneAgentDefs(state.agents),
	}, nil
}

// scanAgentConfig captures configuration and agent dependencies without traversing skill roots.
func (r *Resolver) scanAgentConfig(ctx context.Context, ws Workspace, profileName string) (workspaceScan, error) {
	if err := checkContext(ctx); err != nil {
		return workspaceScan{}, err
	}
	scan := workspaceScan{snapshots: make(map[string]filesnap.Snapshot)}
	declarations, err := r.scanWorkspaceDependencies(ws, profileName, scan.snapshots)
	if err != nil {
		return workspaceScan{}, err
	}
	scan.profileDeclarations = declarations
	for _, root := range compozyconfig.WorkspaceDiscoveryRoots(ws.RootDir, ws.AdditionalDirs, r.homePaths, profileName) {
		if err := checkContext(ctx); err != nil {
			return workspaceScan{}, err
		}
		if err := scanAgentSource(root, scan.snapshots, &scan.agents); err != nil {
			return workspaceScan{}, err
		}
	}
	return scan, nil
}

// buildWorkspaceAgentState applies the workspace default and validates sandbox and discovered agent definitions.
func buildWorkspaceAgentState(
	ctx context.Context,
	ws Workspace,
	cfg *compozyconfig.Config,
	candidates []agentCandidate,
) (workspaceAgentState, error) {
	applyDefaultAgentOverride(cfg, ws.DefaultAgent)
	resolvedSandbox, err := resolveWorkspaceSandbox(ws, cfg)
	if err != nil {
		return workspaceAgentState{}, fmt.Errorf("workspace: resolve sandbox for %q: %w", ws.ID, err)
	}
	agents, diagnostics, err := loadAgents(ctx, candidates)
	if err != nil {
		return workspaceAgentState{}, err
	}
	return workspaceAgentState{agents: agents, diagnostics: diagnostics, sandbox: resolvedSandbox}, nil
}

// resolveProfileIdentity validates the profile name and rechecks its current availability and durable identity.
func (r *Resolver) resolveProfileIdentity(ctx context.Context, name string) (string, string, error) {
	profileName := strings.TrimSpace(name)
	if err := compozyconfig.ValidateResourceProfileName(profileName); err != nil {
		return "", "", fmt.Errorf("workspace: resolve profile resources: %w", err)
	}
	if r.profileAvailability == nil {
		return profileName, "", nil
	}
	profileID, err := r.profileAvailability.AvailableProfileID(ctx, profileName)
	if err != nil {
		return "", "", fmt.Errorf("workspace: resolve profile resources: %w", err)
	}
	return profileName, profileID, nil
}

// cloneResolvedAgentConfig isolates mutable workspace, configuration, and agent data from cached snapshots.
func cloneResolvedAgentConfig(src *ResolvedAgentConfig) ResolvedAgentConfig {
	return ResolvedAgentConfig{
		Workspace: cloneWorkspace(src.Workspace), WorkspaceID: src.WorkspaceID,
		ProfileID: src.ProfileID, ProfileName: src.ProfileName,
		Config: compozyconfig.CloneConfig(&src.Config), Agents: cloneAgentDefs(src.Agents),
	}
}

// sameWorkspaceRuntimeInputs compares only workspace fields that affect resolved runtime behavior.
func sameWorkspaceRuntimeInputs(left, right Workspace) bool {
	return strings.TrimSpace(left.DefaultAgent) == strings.TrimSpace(right.DefaultAgent) &&
		strings.TrimSpace(left.SandboxRef) == strings.TrimSpace(right.SandboxRef) &&
		strings.TrimSpace(left.RootDir) == strings.TrimSpace(right.RootDir) &&
		slices.Equal(left.AdditionalDirs, right.AdditionalDirs)
}

// evictAgentConfigCacheLocked removes inactive narrow snapshots while the resolver mutex is held.
func (r *Resolver) evictAgentConfigCacheLocked(cutoff time.Time) {
	for key, cached := range r.agentConfigCache {
		if cached.lastAccess.Before(cutoff) {
			delete(r.agentConfigCache, key)
		}
	}
}
