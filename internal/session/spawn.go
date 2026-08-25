package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	// DefaultSpawnMaxChildren is the MVP per-parent active child cap.
	DefaultSpawnMaxChildren = 5
	// DefaultSpawnMaxDepth is the MVP maximum child depth under a root session.
	DefaultSpawnMaxDepth = 1
	// DefaultSpawnRole is used when an agent omits the advisory child role.
	DefaultSpawnRole = "worker"
	// SpawnRoleMemoryExtractor marks daemon-owned extractor children.
	SpawnRoleMemoryExtractor = "memory-extractor"
	// SpawnRoleAutoTitle marks daemon-owned title generator children.
	SpawnRoleAutoTitle = "auto-title"
)

var (
	// ErrSpawnValidation reports a structurally invalid spawn request.
	ErrSpawnValidation = errors.New("session: spawn validation failed")
	// ErrSpawnPermissionDenied reports a failed permission narrowing check.
	ErrSpawnPermissionDenied = errors.New("session: spawn permission denied")
	// ErrSpawnLimitExceeded reports a depth, child, or workspace cap violation.
	ErrSpawnLimitExceeded = errors.New("session: spawn limit exceeded")
)

// SpawnOpts defines the safe child-session creation request accepted by the manager.
type SpawnOpts struct {
	ParentSessionID string
	// InheritedWorktreeID is daemon-owned structural context copied from the parent.
	// Public callers cannot select or override it.
	InheritedWorktreeID  string
	AgentName            string
	Provider             string
	Model                string
	ReasoningEffort      string
	Speed                speedpkg.Speed
	Name                 string
	Workspace            string
	WorkspacePath        string
	NetworkParticipation *participation.Request
	PromptOverlay        string
	SpawnRole            string
	TTL                  time.Duration
	AutoStopOnParent     bool
	NotifyCreator        bool
	NotifyCreatorSet     bool
	PermissionPolicy     store.SessionPermissionPolicy
	IdempotencyKey       string
	AllowStoppedParent   bool
	// DiscardStartFailure is reserved for ephemeral internal role attempts.
	DiscardStartFailure bool
}

type permissionCategory struct {
	name   string
	values func(store.SessionPermissionPolicy) []string
}

var knownPermissionCategories = []permissionCategory{
	{name: "tools", values: func(p store.SessionPermissionPolicy) []string { return p.Tools }},
	{name: "skills", values: func(p store.SessionPermissionPolicy) []string { return p.Skills }},
	{name: "mcp_servers", values: func(p store.SessionPermissionPolicy) []string { return p.MCPServers }},
	{name: "workspace_paths", values: func(p store.SessionPermissionPolicy) []string { return p.WorkspacePaths }},
	{name: "network_channels", values: func(p store.SessionPermissionPolicy) []string { return p.NetworkChannels }},
	{name: "sandbox_profiles", values: func(p store.SessionPermissionPolicy) []string {
		return p.SandboxProfiles
	}},
}

// Spawn creates a bounded child session after enforcing lineage, TTL, caps,
// workspace bounds, and permission narrowing.
func (m *Manager) Spawn(ctx context.Context, opts SpawnOpts) (*Session, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: spawn context is required")
	}

	m.spawnMu.Lock()
	defer m.spawnMu.Unlock()

	normalized, parent, lineage, err := m.prepareSpawn(ctx, opts)
	if err != nil {
		return nil, err
	}
	workspaceRef, workspacePath := spawnWorkspaceCreateRefs(parent, normalized)

	child, err := m.Create(ctx, CreateOpts{
		ProfileID:            strings.TrimSpace(parent.ProfileID),
		AgentName:            normalized.AgentName,
		Provider:             normalized.Provider,
		Model:                normalized.Model,
		ReasoningEffort:      normalized.ReasoningEffort,
		Speed:                normalized.Speed,
		Name:                 normalized.Name,
		Workspace:            workspaceRef,
		WorkspacePath:        workspacePath,
		Worktree:             normalized.InheritedWorktreeID,
		NetworkParticipation: spawnNetworkParticipation(normalized),
		NetworkAuthority: &participation.AuthorityScope{
			Enforced:   true,
			ChannelIDs: append([]string(nil), normalized.PermissionPolicy.NetworkChannels...),
		},
		PromptOverlay:       normalized.PromptOverlay,
		Type:                SessionTypeSpawned,
		Lineage:             lineage,
		ParentSoulDigest:    strings.TrimSpace(parent.SoulDigest),
		DiscardStartFailure: normalized.DiscardStartFailure,
	})
	if err != nil {
		return nil, err
	}
	if hookErr := m.dispatchSpawnCreated(ctx, parent, child.Info()); hookErr != nil {
		return child, fmt.Errorf("session: dispatch spawn created hooks for %q: %w", child.ID, hookErr)
	}
	return child, nil
}

func (m *Manager) prepareSpawn(
	ctx context.Context,
	opts SpawnOpts,
) (SpawnOpts, *Info, *store.SessionLineage, error) {
	normalized, err := normalizeSpawnOpts(opts)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}
	parent, err := m.spawnParent(ctx, normalized.ParentSessionID, normalized.AllowStoppedParent)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}
	if normalized.Speed == "" {
		normalized.Speed = parent.Speed
	}
	normalized.InheritedWorktreeID = strings.TrimSpace(parent.WorktreeID)
	normalized, err = m.validateSpawnWorkspace(ctx, parent, normalized)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}

	lineage, err := m.spawnLineage(ctx, parent, normalized)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}
	if err := ValidatePermissionSubset(parent.Lineage.PermissionPolicy, normalized.PermissionPolicy); err != nil {
		return SpawnOpts{}, nil, nil, err
	}

	normalized, lineage, err = m.dispatchSpawnPreCreate(ctx, parent, normalized, lineage)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}
	if strings.TrimSpace(normalized.InheritedWorktreeID) != strings.TrimSpace(parent.WorktreeID) {
		return SpawnOpts{}, nil, nil, spawnValidation("inherited worktree binding is immutable")
	}
	normalized, err = m.validateSpawnWorkspace(ctx, parent, normalized)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}
	if err := ValidatePermissionSubset(parent.Lineage.PermissionPolicy, normalized.PermissionPolicy); err != nil {
		return SpawnOpts{}, nil, nil, err
	}
	return normalized, parent, lineage, nil
}

func normalizeSpawnOpts(opts SpawnOpts) (SpawnOpts, error) {
	normalized := opts
	normalized.ParentSessionID = strings.TrimSpace(normalized.ParentSessionID)
	normalized.AgentName = strings.TrimSpace(normalized.AgentName)
	normalized.Provider = strings.TrimSpace(normalized.Provider)
	normalized.Model = strings.TrimSpace(normalized.Model)
	normalized.ReasoningEffort = strings.TrimSpace(normalized.ReasoningEffort)
	if normalized.Speed != "" {
		parsedSpeed, err := speedpkg.Parse(string(normalized.Speed))
		if err != nil {
			return SpawnOpts{}, spawnValidation(err.Error())
		}
		normalized.Speed = parsedSpeed
	}
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.Workspace = strings.TrimSpace(normalized.Workspace)
	normalized.WorkspacePath = strings.TrimSpace(normalized.WorkspacePath)
	if normalized.NetworkParticipation != nil {
		request, err := participation.NormalizeIntent(*normalized.NetworkParticipation)
		if err != nil {
			return SpawnOpts{}, spawnValidation(fmt.Sprintf("network_participation: %v", err))
		}
		normalized.NetworkParticipation = &request
	}
	normalized.PromptOverlay = strings.TrimSpace(normalized.PromptOverlay)
	normalized.SpawnRole = normalizeSpawnRole(normalized.SpawnRole)
	if IsInternalSpawnRole(normalized.SpawnRole) {
		normalized.NotifyCreator = false
		normalized.NotifyCreatorSet = true
	} else if !normalized.NotifyCreatorSet {
		normalized.NotifyCreator = true
	}
	normalized.PermissionPolicy = store.NormalizeSessionPermissionPolicy(normalized.PermissionPolicy)
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)

	switch {
	case normalized.ParentSessionID == "":
		return SpawnOpts{}, spawnValidation("parent_session_id is required")
	case normalized.AgentName == "":
		return SpawnOpts{}, spawnValidation("agent_name is required")
	case normalized.TTL <= 0:
		return SpawnOpts{}, spawnValidation("ttl is required and must be positive")
	case isCoordinatorSpawnRole(normalized.SpawnRole):
		return SpawnOpts{}, spawnValidation("coordinator spawn role is not supported in MVP")
	case normalized.AllowStoppedParent && !isMemoryExtractorSpawnRole(normalized.SpawnRole):
		return SpawnOpts{}, spawnValidation("allow_stopped_parent is restricted to memory extractor spawns")
	case normalized.AllowStoppedParent && normalized.AutoStopOnParent:
		return SpawnOpts{}, spawnValidation("allow_stopped_parent cannot use auto_stop_on_parent")
	default:
		return normalized, nil
	}
}

func (m *Manager) spawnParent(ctx context.Context, parentID string, allowStopped bool) (*Info, error) {
	parent, err := m.Status(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("%w: parent session %q: %w", ErrSpawnValidation, parentID, err)
	}
	if parent == nil {
		return nil, fmt.Errorf("%w: parent session %q returned nil status", ErrSpawnValidation, parentID)
	}
	if parent.State != StateActive {
		if allowStopped && parent.State == StateStopped {
			parent.Lineage = store.NormalizeSessionLineage(parent.ID, parent.Lineage)
			return parent, nil
		}
		return nil, fmt.Errorf("%w: parent session %q is %q", ErrSpawnValidation, parent.ID, parent.State)
	}
	parent.Lineage = store.NormalizeSessionLineage(parent.ID, parent.Lineage)
	return parent, nil
}

func (m *Manager) validateSpawnWorkspace(
	ctx context.Context,
	parent *Info,
	opts SpawnOpts,
) (SpawnOpts, error) {
	if parent == nil {
		return SpawnOpts{}, spawnValidation("parent session is required")
	}
	refs := make([]string, 0, 2)
	if opts.Workspace != "" {
		refs = append(refs, opts.Workspace)
	}
	if opts.WorkspacePath != "" {
		refs = append(refs, opts.WorkspacePath)
	}
	if len(refs) == 0 {
		return opts, nil
	}
	resolver, err := m.requireWorkspaceResolver()
	if err != nil {
		return SpawnOpts{}, err
	}
	targetWorkspaceID := ""
	for _, ref := range refs {
		resolved, resolveErr := resolver.Resolve(ctx, ref)
		if resolveErr != nil {
			return SpawnOpts{}, fmt.Errorf(
				"%w: resolve child workspace %q: %w",
				ErrSpawnValidation,
				ref,
				resolveErr,
			)
		}
		resolvedID := strings.TrimSpace(resolved.ID)
		if resolvedID == "" {
			return SpawnOpts{}, fmt.Errorf("%w: child workspace %q has no id", ErrSpawnValidation, ref)
		}
		if targetWorkspaceID != "" && targetWorkspaceID != resolvedID {
			return SpawnOpts{}, fmt.Errorf("%w: child workspace references disagree", ErrSpawnValidation)
		}
		targetWorkspaceID = resolvedID
	}
	parentWorkspaceID := strings.TrimSpace(parent.WorkspaceID)
	if parentWorkspaceID == "" && strings.TrimSpace(parent.Workspace) != "" {
		resolved, resolveErr := resolver.Resolve(ctx, parent.Workspace)
		if resolveErr != nil {
			return SpawnOpts{}, fmt.Errorf("%w: resolve parent workspace: %w", ErrSpawnValidation, resolveErr)
		}
		parentWorkspaceID = strings.TrimSpace(resolved.ID)
	}
	if parentWorkspaceID != targetWorkspaceID {
		policy := m.workspaceAccessPolicy()
		if policy == nil {
			return SpawnOpts{}, spawnWorkspaceAccessDenied(targetWorkspaceID, parentWorkspaceID)
		}
		decision, authorizeErr := policy.Authorize(ctx, workspaceaccess.Request{
			Actor: workspaceaccess.ActorRef{
				Kind:        workspaceaccess.ActorAgentSession,
				SessionID:   strings.TrimSpace(parent.ID),
				WorkspaceID: parentWorkspaceID,
				AgentName:   strings.TrimSpace(parent.AgentName),
			},
			TargetWorkspaceID: targetWorkspaceID,
			Seam:              workspaceaccess.SeamSpawn,
		})
		if authorizeErr != nil {
			return SpawnOpts{}, fmt.Errorf(
				"%w: authorize child workspace %q: %w",
				ErrSpawnValidation,
				targetWorkspaceID,
				authorizeErr,
			)
		}
		if !decision.Allowed {
			return SpawnOpts{}, spawnWorkspaceAccessDenied(targetWorkspaceID, parentWorkspaceID)
		}
	}
	opts.Workspace = targetWorkspaceID
	opts.WorkspacePath = ""
	return opts, nil
}

func spawnWorkspaceAccessDenied(targetWorkspaceID string, parentWorkspaceID string) error {
	return fmt.Errorf(
		"%w: child workspace %q is outside parent workspace %q: %s",
		ErrSpawnPermissionDenied,
		targetWorkspaceID,
		parentWorkspaceID,
		workspaceaccess.DenialHint,
	)
}

func (m *Manager) spawnLineage(
	ctx context.Context,
	parent *Info,
	opts SpawnOpts,
) (*store.SessionLineage, error) {
	parentLineage := store.NormalizeSessionLineage(parent.ID, parent.Lineage)
	budget := effectiveSpawnBudget(parentLineage.SpawnBudget)
	governance, err := m.spawnGovernanceForParent(ctx, parent)
	if err != nil {
		return nil, err
	}
	governedChildDepth := governance.depth + 1
	if governedChildDepth > budget.MaxDepth {
		return nil, fmt.Errorf(
			"%w: child depth %d exceeds max_depth %d",
			ErrSpawnLimitExceeded,
			governedChildDepth,
			budget.MaxDepth,
		)
	}
	targetWorkspaceID := strings.TrimSpace(opts.Workspace)
	if targetWorkspaceID == "" {
		targetWorkspaceID = strings.TrimSpace(parent.WorkspaceID)
	}
	if err := m.validateSpawnCaps(ctx, parent, governance, budget, targetWorkspaceID); err != nil {
		return nil, err
	}

	rootID := strings.TrimSpace(parentLineage.RootSessionID)
	if rootID == "" {
		rootID = parent.ID
	}
	if rootID != governance.rootID {
		if _, statusErr := m.Status(ctx, rootID); statusErr != nil {
			if !errors.Is(statusErr, ErrSessionNotFound) {
				return nil, fmt.Errorf(
					"%w: resolve child lineage root %q: %w",
					ErrSpawnValidation,
					rootID,
					statusErr,
				)
			}
			rootID = governance.rootID
		}
	}
	ttlExpiresAt := m.now().UTC().Add(opts.TTL)
	budget.TTLSeconds = durationSecondsCeil(opts.TTL)
	return store.NormalizeSessionLineage("", &store.SessionLineage{
		ParentSessionID:  parent.ID,
		RootSessionID:    rootID,
		SpawnDepth:       parentLineage.SpawnDepth + 1,
		SpawnRole:        opts.SpawnRole,
		TTLExpiresAt:     &ttlExpiresAt,
		AutoStopOnParent: opts.AutoStopOnParent,
		NotifyCreator:    opts.NotifyCreator,
		SpawnBudget:      budget,
		PermissionPolicy: opts.PermissionPolicy,
	}), nil
}

func (m *Manager) validateSpawnCaps(
	ctx context.Context,
	parent *Info,
	governance spawnGovernance,
	budget store.SessionSpawnBudget,
	targetWorkspaceID string,
) error {
	infos, err := m.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("%w: count child sessions for %q: %w", ErrSpawnValidation, parent.ID, err)
	}
	activeChildren := 0
	activeInWorkspace := 0
	infosByID := make(map[string]*Info, len(infos))
	for _, info := range infos {
		if info != nil {
			infosByID[info.ID] = info
		}
	}
	lookup := func(sessionID string) (*Info, bool, error) {
		info, ok := infosByID[sessionID]
		return info, ok, nil
	}
	for _, info := range infos {
		if info == nil || normalizeSessionType(info.Type) != SessionTypeSpawned ||
			info.Lineage == nil || !isLiveSpawnState(info.State) {
			continue
		}
		lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
		if lineage.ParentSessionID == parent.ID {
			activeChildren++
		}
		if budget.MaxActivePerWorkspace > 0 && strings.TrimSpace(info.WorkspaceID) == targetWorkspaceID {
			candidate, candidateErr := resolveSpawnGovernance(info, lookup)
			if candidateErr != nil {
				return candidateErr
			}
			if candidate.rootID == governance.rootID {
				activeInWorkspace++
			}
		}
	}
	if activeChildren >= budget.MaxChildren {
		return fmt.Errorf(
			"%w: parent %q has %d active children, max_children %d",
			ErrSpawnLimitExceeded,
			parent.ID,
			activeChildren,
			budget.MaxChildren,
		)
	}
	if budget.MaxActivePerWorkspace > 0 && activeInWorkspace >= budget.MaxActivePerWorkspace {
		return fmt.Errorf(
			"%w: workspace %q has %d active spawned sessions, max_active_per_workspace %d",
			ErrSpawnLimitExceeded,
			targetWorkspaceID,
			activeInWorkspace,
			budget.MaxActivePerWorkspace,
		)
	}
	return nil
}
