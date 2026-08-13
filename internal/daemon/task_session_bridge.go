package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/worktree"
)

type taskBridgeSessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	Status(ctx context.Context, id string) (*session.Info, error)
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

type taskBridgeSessionRequestStopper interface {
	RequestStopWithCause(
		ctx context.Context,
		id string,
		cause session.StopCause,
		detail string,
	) error
}

type taskSessionBridge struct {
	sessions            taskBridgeSessionManager
	worktrees           taskBridgeWorktrees
	globalWorkspacePath string
	contextOverlay      taskSessionContextOverlay
	logger              *slog.Logger
}

type taskBridgeWorktrees interface {
	Get(context.Context, string, string) (*worktree.Worktree, error)
	MaterializeForRun(context.Context, string, worktree.RunWorktreeRequest) (*worktree.Worktree, error)
	RollbackRunMaterialization(context.Context, string, string, string) error
}

type taskSessionContextOverlay interface {
	TaskRunPromptOverlay(
		ctx context.Context,
		taskRecord taskpkg.Task,
		run taskpkg.Run,
		profile *taskpkg.ExecutionProfile,
	) (string, error)
}

type taskSessionBridgeOption func(*taskSessionBridge)

func withTaskSessionContextOverlay(overlay taskSessionContextOverlay) taskSessionBridgeOption {
	return func(bridge *taskSessionBridge) {
		if bridge != nil {
			bridge.contextOverlay = overlay
		}
	}
}

func withTaskSessionWorktrees(worktrees taskBridgeWorktrees) taskSessionBridgeOption {
	return func(bridge *taskSessionBridge) {
		if bridge != nil {
			bridge.worktrees = worktrees
		}
	}
}

type taskRecoveryStats struct {
	terminalSettled int
	requeued        int
	markedRunning   int
	failed          int
}

type taskSessionRecoveryEvidence struct {
	live           bool
	stopRequired   bool
	state          string
	classification string
	detail         string
}

var _ taskpkg.SessionExecutor = (*taskSessionBridge)(nil)

func newTaskSessionBridge(
	sessions taskBridgeSessionManager,
	globalWorkspacePath string,
	logger *slog.Logger,
	options ...taskSessionBridgeOption,
) (*taskSessionBridge, error) {
	if sessions == nil {
		return nil, errors.New("daemon: task session bridge requires a session manager")
	}
	if logger == nil {
		logger = slog.Default()
	}
	bridge := &taskSessionBridge{
		sessions:            sessions,
		globalWorkspacePath: strings.TrimSpace(globalWorkspacePath),
		logger:              logger,
	}
	for _, option := range options {
		if option != nil {
			option(bridge)
		}
	}
	return bridge, nil
}

func (b *taskSessionBridge) StartTaskSession(
	ctx context.Context,
	spec *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	if ctx == nil {
		return nil, errors.New("daemon: start task session context is required")
	}
	if spec == nil {
		return nil, errors.New("daemon: start task session spec is required")
	}

	opts := session.CreateOpts{
		AgentName:                    taskSessionAgentName(spec.Task),
		Provider:                     "",
		Name:                         taskSessionName(spec),
		ResolvedNetworkParticipation: participationSnapshotPointer(spec.Run.NetworkSpecSnapshot()),
		Type:                         session.SessionTypeSystem,
	}
	owner := participation.OwnerRef{Kind: participation.OwnerKindTaskRun, ID: spec.Run.ID}
	if strings.TrimSpace(spec.Run.LoopRunID) != "" {
		owner = participation.OwnerRef{Kind: participation.OwnerKindLoopRun, ID: spec.Run.LoopRunID}
	}
	opts.NetworkOwnerKey = participation.OwnerKey(owner)
	applyTaskSessionWorkerProfile(&opts, spec.ExecutionProfile)
	policy := sessionPolicyFromTaskExecutionProfile(spec.ExecutionProfile)
	applySessionSandboxPolicy(&opts, policy)
	applySessionPermissionPolicy(&opts, policy)
	switch spec.Task.Scope.Normalize() {
	case taskpkg.ScopeWorkspace:
		opts.Workspace = strings.TrimSpace(spec.Task.WorkspaceID)
	case taskpkg.ScopeGlobal:
		if b.globalWorkspacePath == "" {
			return nil, errors.New("daemon: task session bridge global workspace path is required")
		}
		opts.WorkspacePath = b.globalWorkspacePath
	default:
		return nil, fmt.Errorf(
			"%w: unsupported task scope %q for task session start",
			taskpkg.ErrValidation,
			spec.Task.Scope,
		)
	}
	materialized, err := b.applySessionWorktreePolicy(ctx, &opts, spec)
	if err != nil {
		return nil, err
	}
	if b.contextOverlay != nil {
		overlayRun := spec.Run
		if materialized != nil {
			overlayRun.SetWorktreeID(materialized.ID)
		}
		overlay, err := b.contextOverlay.TaskRunPromptOverlay(
			ctx,
			spec.Task,
			overlayRun,
			spec.ExecutionProfile,
		)
		if err != nil {
			return nil, b.rollbackTaskSessionMaterialization(
				ctx,
				spec.Run,
				materialized,
				fmt.Errorf("daemon: render task session context overlay: %w", err),
			)
		}
		opts.PromptOverlay = joinPromptOverlays(opts.PromptOverlay, overlay)
	}

	return b.createTaskSession(ctx, spec.Run, opts, materialized)
}

func (b *taskSessionBridge) createTaskSession(
	ctx context.Context,
	run taskpkg.Run,
	opts session.CreateOpts,
	materialized *worktree.Worktree,
) (*taskpkg.SessionRef, error) {
	created, err := b.sessions.Create(ctx, opts)
	if err != nil {
		return nil, b.rollbackTaskSessionMaterialization(ctx, run, materialized, err)
	}
	if created == nil {
		return nil, b.rollbackTaskSessionMaterialization(ctx, run, materialized, fmt.Errorf(
			"%w: task session bridge create returned nil session",
			taskpkg.ErrValidation,
		))
	}
	info := created.Info()
	if info == nil {
		return nil, b.rollbackTaskSessionMaterialization(ctx, run, materialized, fmt.Errorf(
			"%w: task session bridge create returned nil session info",
			taskpkg.ErrValidation,
		))
	}
	worktreeID := strings.TrimSpace(info.WorktreeID)
	if worktreeID == "" && materialized != nil {
		worktreeID = materialized.ID
	}
	return &taskpkg.SessionRef{
		SessionID:   strings.TrimSpace(info.ID),
		WorkspaceID: strings.TrimSpace(info.WorkspaceID),
		WorktreeID:  worktreeID,
		StartedAt:   info.CreatedAt,
	}, nil
}

func (b *taskSessionBridge) applySessionWorktreePolicy(
	ctx context.Context,
	opts *session.CreateOpts,
	spec *taskpkg.StartTaskSession,
) (*worktree.Worktree, error) {
	policy := taskpkg.WorktreePolicy{
		Mode:        spec.Run.ResolvedWorktreeModeValue().Normalize(),
		WorktreeRef: strings.TrimSpace(spec.Run.ResolvedWorktreeRefValue()),
	}
	if policy.Mode == "" {
		policy.Mode = taskpkg.WorktreeModeNone
	}
	if spec.Task.Scope.Normalize() == taskpkg.ScopeGlobal && policy.Mode != taskpkg.WorktreeModeNone {
		return nil, fmt.Errorf("%w: global tasks cannot use worktree execution", taskpkg.ErrValidation)
	}
	switch policy.Mode {
	case taskpkg.WorktreeModeNone:
		return nil, nil
	case taskpkg.WorktreeModeRef:
		item, err := b.resolveTaskWorktreeRef(ctx, spec.Task.WorkspaceID, policy.WorktreeRef)
		if err != nil {
			return nil, err
		}
		opts.Worktree = item.ID
		return nil, nil
	case taskpkg.WorktreeModePerRun:
		if b.worktrees == nil {
			return nil, fmt.Errorf("%w: worktree service is unavailable", worktree.ErrPerRunMaterialization)
		}
		workspaceID := strings.TrimSpace(spec.Task.WorkspaceID)
		if workspaceID == "" {
			return nil, fmt.Errorf("%w: task workspace is required", worktree.ErrPerRunMaterialization)
		}
		item, err := b.worktrees.MaterializeForRun(ctx, workspaceID, worktree.RunWorktreeRequest{
			TaskSlug: taskWorktreeSlug(spec.Task),
			RunID:    spec.Run.ID,
		})
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, fmt.Errorf("%w: materializer returned nil worktree", worktree.ErrPerRunMaterialization)
		}
		opts.Worktree = item.ID
		return item, nil
	default:
		return nil, fmt.Errorf("%w: unsupported resolved worktree mode %q", taskpkg.ErrValidation, policy.Mode)
	}
}

func (b *taskSessionBridge) resolveTaskWorktreeRef(
	ctx context.Context,
	workspaceID string,
	ref string,
) (*worktree.Worktree, error) {
	if b.worktrees == nil {
		return nil, fmt.Errorf("%w: worktree service is unavailable", worktree.ErrRefInvalid)
	}
	item, err := b.worktrees.Get(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(ref))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", worktree.ErrRefInvalid, err)
	}
	if item == nil || item.State != worktree.StateReady {
		return nil, fmt.Errorf("%w: referenced worktree is not ready", worktree.ErrRefInvalid)
	}
	info, err := os.Stat(item.Path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", worktree.ErrRefInvalid, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: referenced worktree path is not a directory", worktree.ErrRefInvalid)
	}
	return item, nil
}

func taskWorktreeSlug(taskRecord taskpkg.Task) string {
	return firstNonEmpty(taskRecord.Identifier, taskRecord.Title, taskRecord.ID)
}

func (b *taskSessionBridge) rollbackTaskSessionMaterialization(
	ctx context.Context,
	run taskpkg.Run,
	item *worktree.Worktree,
	cause error,
) error {
	if item == nil || b.worktrees == nil {
		return cause
	}
	rollbackErr := b.worktrees.RollbackRunMaterialization(
		context.WithoutCancel(ctx),
		item.WorkspaceID,
		item.ID,
		run.ID,
	)
	if rollbackErr != nil {
		rollbackErr = fmt.Errorf("daemon: roll back per-run worktree: %w", rollbackErr)
	}
	return errors.Join(cause, rollbackErr)
}

func (b *taskSessionBridge) CleanupUnboundTaskSession(
	ctx context.Context,
	run taskpkg.Run,
	ref taskpkg.SessionRef,
) error {
	if run.ResolvedWorktreeModeValue().Normalize() != taskpkg.WorktreeModePerRun ||
		strings.TrimSpace(ref.WorktreeID) == "" || b.worktrees == nil {
		return nil
	}
	return b.worktrees.RollbackRunMaterialization(
		context.WithoutCancel(ctx),
		strings.TrimSpace(run.WorkspaceID),
		strings.TrimSpace(ref.WorktreeID),
		strings.TrimSpace(run.ID),
	)
}

func joinPromptOverlays(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "\n\n")
}

func applyTaskSessionWorkerProfile(opts *session.CreateOpts, profile *taskpkg.ExecutionProfile) {
	if opts == nil || profile == nil {
		return
	}
	worker := profile.Worker
	opts.AgentName = strings.TrimSpace(worker.AgentName)
	opts.Provider = strings.TrimSpace(worker.Provider)
	opts.Model = strings.TrimSpace(worker.Model)
}
