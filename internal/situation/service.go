package situation

import (
	"context"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network"

	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/soul"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	// DefaultSectionLimit is the MVP bound for list sections inside agent context.
	DefaultSectionLimit = 8
	// ProvenanceSource identifies the local daemon context assembler.
	ProvenanceSource = "daemon.situation"

	defaultMaxSpawnDepth       = 1
	defaultMaxActiveTaskLeases = 1
	inboxPreviewLimit          = 180
)

// WorkspaceResolver resolves persisted workspaces into runtime snapshots.
type WorkspaceResolver interface {
	Resolve(ctx context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error)
}

// AgentResolver resolves one agent definition for a workspace.
type AgentResolver interface {
	ResolveAgent(name string, resolved *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error)
}

// SkillRegistry resolves the active skill set for a workspace.
type SkillRegistry interface {
	ForWorkspace(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skillspkg.Skill, error)
	ForAgent(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
	) ([]*skillspkg.Skill, error)
}

// TaskStore is the narrowed task read surface required by agent context.
type TaskStore interface {
	GetTask(ctx context.Context, id string) (taskpkg.Task, error)
	GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error)
	ListTaskRuns(ctx context.Context, query taskpkg.RunQuery) ([]taskpkg.Run, error)
	ListTaskEvents(ctx context.Context, query taskpkg.EventQuery) ([]taskpkg.Event, error)
	ListTaskEventRecords(ctx context.Context, query taskpkg.EventRecordQuery) ([]taskpkg.EventRecord, error)
	GetExecutionProfile(ctx context.Context, taskID string) (taskpkg.ExecutionProfile, error)
	GetRunReview(ctx context.Context, reviewID string) (taskpkg.RunReview, error)
	LookupRunReviewBySession(ctx context.Context, sessionID string) (taskpkg.RunReview, error)
	ListRunReviews(ctx context.Context, query taskpkg.RunReviewQuery) ([]taskpkg.RunReview, error)
}

// NetworkReader is the narrowed network read surface required by agent context.
type NetworkReader interface {
	ListPeers(ctx context.Context, workspaceID string, channel string) ([]network.PeerInfo, error)
	Inbox(ctx context.Context, sessionID string) ([]network.Envelope, error)
}

// CoordinatorRoleResolver reads the safe coordinator limits for a workspace.
type CoordinatorRoleResolver interface {
	ResolveCoordinatorRole(ctx context.Context, workspaceID string) (aghconfig.ResolvedCoordinatorRole, error)
}

// SoulSnapshotStore loads immutable Soul snapshots for compact context projection.
type SoulSnapshotStore interface {
	GetSoulSnapshot(ctx context.Context, id string) (soul.Snapshot, error)
}

// Deps wires situation context to daemon-owned services. Function fields are
// evaluated at render time so daemon boot can install the provider before late
// runtime services are available.
type Deps struct {
	Now func() time.Time

	SectionLimit int

	WorkspaceResolver     WorkspaceResolver
	WorkspaceResolverFunc func() WorkspaceResolver
	AgentResolver         AgentResolver
	AgentResolverFunc     func() AgentResolver
	SkillRegistry         SkillRegistry
	SkillRegistryFunc     func() SkillRegistry
	TaskStore             TaskStore
	TaskStoreFunc         func() TaskStore
	Network               NetworkReader
	NetworkFunc           func() NetworkReader
	CoordinatorRole       CoordinatorRoleResolver
	CoordinatorRoleFunc   func() CoordinatorRoleResolver
	SoulSnapshots         SoulSnapshotStore
	SoulSnapshotsFunc     func() SoulSnapshotStore
}

// Service assembles contract.AgentContextPayload and renders prompt sections.
type Service struct {
	now          func() time.Time
	sectionLimit int

	workspaceResolver     WorkspaceResolver
	workspaceResolverFunc func() WorkspaceResolver
	agentResolver         AgentResolver
	agentResolverFunc     func() AgentResolver
	skillRegistry         SkillRegistry
	skillRegistryFunc     func() SkillRegistry
	taskStore             TaskStore
	taskStoreFunc         func() TaskStore
	network               NetworkReader
	networkFunc           func() NetworkReader
	coordinatorRole       CoordinatorRoleResolver
	coordinatorRoleFunc   func() CoordinatorRoleResolver
	soulSnapshots         SoulSnapshotStore
	soulSnapshotsFunc     func() SoulSnapshotStore
	promptSections        *promptSectionCache
}

// NewService constructs a deterministic situation context assembler.
func NewService(deps Deps) *Service {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	limit := deps.SectionLimit
	if limit <= 0 {
		limit = DefaultSectionLimit
	}

	return &Service{
		now:                   now,
		sectionLimit:          limit,
		workspaceResolver:     deps.WorkspaceResolver,
		workspaceResolverFunc: deps.WorkspaceResolverFunc,
		agentResolver:         deps.AgentResolver,
		agentResolverFunc:     deps.AgentResolverFunc,
		skillRegistry:         deps.SkillRegistry,
		skillRegistryFunc:     deps.SkillRegistryFunc,
		taskStore:             deps.TaskStore,
		taskStoreFunc:         deps.TaskStoreFunc,
		network:               deps.Network,
		networkFunc:           deps.NetworkFunc,
		coordinatorRole:       deps.CoordinatorRole,
		coordinatorRoleFunc:   deps.CoordinatorRoleFunc,
		soulSnapshots:         deps.SoulSnapshots,
		soulSnapshotsFunc:     deps.SoulSnapshotsFunc,
		promptSections:        newPromptSectionCache(),
	}
}
