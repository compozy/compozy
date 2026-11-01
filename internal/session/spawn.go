package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	// DefaultSpawnRole is used when an agent omits the advisory child role.
	DefaultSpawnRole = "worker"
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
	ParentTurnID    string
	Permissions     compozyconfig.PermissionMode
	Subagent        *hookspkg.SubagentSpawnPayload
	ParentSessionID string
	// InheritedWorktreeID is daemon-owned structural context copied from the parent.
	// Public callers cannot select or override it.
	InheritedWorktreeID string
	IsolatedWorktreeID  string
	AgentName           string
	Provider            string
	Model               string
	ReasoningEffort     string
	Speed               speedpkg.Speed
	ACPOptions          []acp.SessionConfigOptionSelection
	Name                string
	Workspace           string
	WorkspacePath       string
	PromptOverlay       string
	SpawnRole           string
	TTL                 time.Duration
	AutoStopOnParent    bool
	NotifyCreator       bool
	NotifyCreatorSet    bool
	PermissionPolicy    store.SessionPermissionPolicy
	// InheritPermissions starts from the parent's ceiling; PermissionNarrowing lists
	// replace only the categories they set. Without it, PermissionPolicy is literal.
	InheritPermissions  bool
	PermissionNarrowing *SubagentPermissionNarrowing
	// Toolsets selects parent-granted tools by toolset ID; it never widens the parent.
	Toolsets       []string
	IdempotencyKey string
	// DiscardStartFailure is reserved for ephemeral internal role attempts.
	DiscardStartFailure bool
	// Command is an explicit launch command for this attempt (a fallback route's account).
	Command    string
	SessionEnv compozyconfig.SessionEnv
	// ChainOwner declares who owns the fallback chain; see CreateOpts.ChainOwner.
	ChainOwner ChainOwner
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
	desiredID := ""
	if normalized.SpawnRole == store.SubagentSpawnRole && normalized.IdempotencyKey != "" {
		desiredID = subagentChildSessionID(parent.ID, normalized.IdempotencyKey)
		if existing, ok := m.Get(desiredID); ok {
			return existing, nil
		}
	}

	child, err := m.Create(ctx, CreateOpts{
		Origin: workspaceaccess.Origin{
			Kind:    workspaceaccess.ActorAgentSession,
			ID:      parent.ID,
			Ceiling: parent.EffectiveWorkspaceAccess,
		},
		DesiredSessionID:    desiredID,
		ProfileID:           strings.TrimSpace(parent.ProfileID),
		AgentName:           normalized.AgentName,
		Permissions:         normalized.Permissions,
		Provider:            normalized.Provider,
		Model:               normalized.Model,
		ReasoningEffort:     normalized.ReasoningEffort,
		Speed:               normalized.Speed,
		ACPOptions:          acp.CloneSessionConfigOptionSelections(normalized.ACPOptions),
		Name:                normalized.Name,
		Workspace:           workspaceRef,
		WorkspacePath:       workspacePath,
		Worktree:            normalized.InheritedWorktreeID,
		PromptOverlay:       normalized.PromptOverlay,
		RuntimeMode:         spawnRuntimeMode(normalized.SpawnRole),
		Type:                SessionTypeSpawned,
		Lineage:             lineage,
		ParentSoulDigest:    strings.TrimSpace(parent.SoulDigest),
		DiscardStartFailure: normalized.DiscardStartFailure,
		Command:             normalized.Command,
		SessionEnv:          compozyconfig.MergeSessionEnv(nil, normalized.SessionEnv),
		ChainOwner:          normalized.ChainOwner,
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
	parent, err := m.spawnParent(ctx, normalized.ParentSessionID)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}
	if normalized.SpawnRole == store.SubagentSpawnRole {
		if err := m.validateSubagentSpawn(parent, &normalized); err != nil {
			return SpawnOpts{}, nil, nil, err
		}
	}
	if normalized.Speed == "" {
		normalized.Speed = parent.Speed
	}
	normalized.InheritedWorktreeID = strings.TrimSpace(parent.WorktreeID)
	if normalized.IsolatedWorktreeID != "" {
		normalized.InheritedWorktreeID = normalized.IsolatedWorktreeID
	}
	normalized, err = m.validateSpawnWorkspace(ctx, parent, normalized)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}

	normalized.PermissionPolicy, err = m.resolveSpawnPermissionPolicy(parent, normalized)
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

	expectedWorktreeID := normalized.InheritedWorktreeID
	normalized, lineage, err = m.dispatchSpawnPreCreate(ctx, parent, normalized, lineage)
	if err != nil {
		return SpawnOpts{}, nil, nil, err
	}
	if strings.TrimSpace(normalized.InheritedWorktreeID) != expectedWorktreeID {
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
	normalized.Command = strings.TrimSpace(normalized.Command)
	normalized.SessionEnv = compozyconfig.MergeSessionEnv(nil, normalized.SessionEnv)
	normalized.ReasoningEffort = strings.TrimSpace(normalized.ReasoningEffort)
	var err error
	normalized.ACPOptions, err = acp.NormalizeSessionConfigOptionSelections(normalized.ACPOptions)
	if err != nil {
		return SpawnOpts{}, spawnValidation(err.Error())
	}
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
	case normalized.SpawnRole == store.SubagentSpawnRole && normalized.TTL != 0:
		return SpawnOpts{}, ErrSubagentCapabilityDenied
	case normalized.TTL <= 0 && normalized.SpawnRole != store.SubagentSpawnRole:
		return SpawnOpts{}, spawnValidation("ttl is required and must be positive")
	case isCoordinatorSpawnRole(normalized.SpawnRole):
		return SpawnOpts{}, spawnValidation("coordinator spawn role is not supported in MVP")
	default:
		return normalized, nil
	}
}

func (m *Manager) spawnParent(ctx context.Context, parentID string) (*Info, error) {
	parent, err := m.Status(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("%w: parent session %q: %w", ErrSpawnValidation, parentID, err)
	}
	if parent == nil {
		return nil, fmt.Errorf("%w: parent session %q returned nil status", ErrSpawnValidation, parentID)
	}
	if parent.State != StateActive {
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
	for index, ref := range refs {
		resolved, resolveErr := resolveWorkspaceForProfile(ctx, resolver, m.profileNames, ref, parent.ProfileID)
		if resolveErr != nil {
			return SpawnOpts{}, fmt.Errorf(
				"%w: resolve child workspace %q: %w",
				ErrSpawnValidation,
				ref,
				resolveErr,
			)
		}
		resolvedID := strings.TrimSpace(resolved.ID)
		if resolvedID == "" && resolved.Scope != workspacepkg.ScopeUser {
			return SpawnOpts{}, fmt.Errorf("%w: child workspace %q has no id", ErrSpawnValidation, ref)
		}
		if index > 0 && targetWorkspaceID != resolvedID {
			return SpawnOpts{}, fmt.Errorf("%w: child workspace references disagree", ErrSpawnValidation)
		}
		targetWorkspaceID = resolvedID
	}
	parentWorkspaceID := strings.TrimSpace(parent.WorkspaceID)

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
			Target: workspaceaccess.ScopeTarget(targetWorkspaceID, parent.ProfileID),
			Seam:   workspaceaccess.SeamSpawn,
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
	if targetWorkspaceID == "" {
		opts.Workspace = workspaceaccess.UserSelector
		if opts.IsolatedWorktreeID != "" {
			return SpawnOpts{}, spawnValidation("User scope does not support worktrees")
		}
		opts.InheritedWorktreeID = ""
	}
	opts.WorkspacePath = ""
	return opts, nil
}

func (m *Manager) spawnLineage(
	ctx context.Context,
	parent *Info,
	opts SpawnOpts,
) (*store.SessionLineage, error) {
	parentLineage := store.NormalizeSessionLineage(parent.ID, parent.Lineage)
	if opts.SpawnRole == store.SubagentSpawnRole {
		return store.NormalizeSessionLineage("", &store.SessionLineage{
			ParentSessionID: parent.ID, RootSessionID: parentLineage.RootSessionID,
			SpawnDepth: parentLineage.SpawnDepth + 1, SpawnRole: opts.SpawnRole,
			AutoStopOnParent: true, NotifyCreator: false, PermissionPolicy: opts.PermissionPolicy,
		}), nil
	}
	budget := store.EffectiveSessionSpawnBudget(parentLineage.SpawnBudget)
	governance, err := m.spawnGovernanceForParent(ctx, parent)
	if err != nil {
		return nil, err
	}
	governedChildDepth := governance.depth + 1
	if governedChildDepth > budget.MaxDepth {
		return nil, &SpawnDepthError{Parent: parent.ID, ChildDepth: governedChildDepth, Max: budget.MaxDepth}
	}
	targetWorkspaceID := strings.TrimSpace(opts.Workspace)
	switch targetWorkspaceID {
	case workspaceaccess.UserSelector:
		targetWorkspaceID = ""
	case "":
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
		// Subagents are bounded by the caller's turn, not the spawn budget.
		if lineage.SpawnRole == store.SubagentSpawnRole {
			continue
		}
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
	coordinatorBudget := spawnBudgetFromCoordinator(parent)
	if activeChildren >= budget.MaxChildren {
		limitErr := &SpawnLimitError{
			Limit: SpawnLimitMaxChildren, Max: budget.MaxChildren, Live: activeChildren, Scope: parent.ID,
			DefaultCap: LineageSpawnBudget(parent.Lineage).MaxChildren <= 0,
		}
		if coordinatorBudget {
			limitErr.ConfigKey = coordinatorMaxChildrenConfigKey
		}
		return limitErr
	}
	if budget.MaxActivePerWorkspace > 0 && activeInWorkspace >= budget.MaxActivePerWorkspace {
		limitErr := &SpawnLimitError{
			Limit: SpawnLimitMaxActivePerWorkspace, Max: budget.MaxActivePerWorkspace,
			Live: activeInWorkspace, Scope: targetWorkspaceID,
		}
		if coordinatorBudget {
			limitErr.ConfigKey = coordinatorMaxActiveConfigKey
		}
		return limitErr
	}
	return nil
}
