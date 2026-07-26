package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
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
	globalWorkspacePath string
	contextOverlay      taskSessionContextOverlay
	logger              *slog.Logger
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

type taskRecoveryStats struct {
	requeued      int
	markedRunning int
	failed        int
}

type taskSessionRecoveryEvidence struct {
	live           bool
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
	if b.contextOverlay != nil {
		overlay, err := b.contextOverlay.TaskRunPromptOverlay(
			ctx,
			spec.Task,
			spec.Run,
			spec.ExecutionProfile,
		)
		if err != nil {
			return nil, fmt.Errorf("daemon: render task session context overlay: %w", err)
		}
		opts.PromptOverlay = joinPromptOverlays(opts.PromptOverlay, overlay)
	}

	created, err := b.sessions.Create(ctx, opts)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, fmt.Errorf(
			"%w: task session bridge create returned nil session",
			taskpkg.ErrValidation,
		)
	}
	info := created.Info()
	if info == nil {
		return nil, fmt.Errorf(
			"%w: task session bridge create returned nil session info",
			taskpkg.ErrValidation,
		)
	}
	return &taskpkg.SessionRef{
		SessionID:   strings.TrimSpace(info.ID),
		WorkspaceID: strings.TrimSpace(info.WorkspaceID),
		StartedAt:   info.CreatedAt,
	}, nil
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

func (b *taskSessionBridge) AttachTaskSession(
	ctx context.Context,
	_ string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	if ctx == nil {
		return nil, errors.New("daemon: attach task session context is required")
	}

	info, err := b.sessions.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf(
			"%w: session %q is unavailable",
			taskpkg.ErrSessionAttachNotAllowed,
			strings.TrimSpace(sessionID),
		)
	}
	if !isTaskSessionStateLive(info.State) {
		return nil, fmt.Errorf(
			"%w: session %q is %q",
			taskpkg.ErrSessionAttachNotAllowed,
			strings.TrimSpace(sessionID),
			info.State,
		)
	}

	return &taskpkg.SessionRef{
		SessionID:   strings.TrimSpace(info.ID),
		WorkspaceID: strings.TrimSpace(info.WorkspaceID),
		StartedAt:   info.CreatedAt,
	}, nil
}

func (b *taskSessionBridge) RequestTaskStop(
	ctx context.Context,
	sessionID string,
	reason taskpkg.StopReason,
) error {
	if ctx == nil {
		return errors.New("daemon: request task stop context is required")
	}

	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return fmt.Errorf("%w: task session stop id is required", taskpkg.ErrValidation)
	}

	if requester, ok := b.sessions.(taskBridgeSessionRequestStopper); ok {
		if err := requester.RequestStopWithCause(
			ctx,
			trimmedID,
			taskStopCause(reason),
			taskStopDetail(reason),
		); err != nil {
			if errors.Is(err, session.ErrSessionNotFound) {
				return nil
			}
			return err
		}
		return nil
	}

	return b.ForceTaskStop(ctx, trimmedID, reason)
}

func (b *taskSessionBridge) ForceTaskStop(
	ctx context.Context,
	sessionID string,
	reason taskpkg.StopReason,
) error {
	if ctx == nil {
		return errors.New("daemon: force task stop context is required")
	}

	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return fmt.Errorf("%w: task session stop id is required", taskpkg.ErrValidation)
	}

	if err := b.sessions.StopWithCause(ctx, trimmedID, taskStopCause(reason), taskStopDetail(reason)); err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return nil
		}
		return err
	}
	return nil
}
