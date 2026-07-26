package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	// defaultStarvationWorkerTTL bounds a capability-matched starvation worker's lifetime. The
	// spawn reaper releases its leases and stops it past this deadline so a worker that claims
	// nothing cannot pile up; released work re-queues and re-escalates from the durable budget.
	defaultStarvationWorkerTTL = 15 * time.Minute
	// defaultStarvationMaxActivePerWorkspace is advisory metadata on the spawn budget; the real
	// per-(agent, channel, scope) cap is the role-session dedup in activeRoleSession.
	defaultStarvationMaxActivePerWorkspace = 3
)

const (
	taskRoleRuntimeTaskIDKey      = daemonTaskIDKey
	taskRoleRuntimeWorkspaceIDKey = daemonWorkspaceIDKey
)

const taskRoleActivationReasonRunEnqueued = "task_run_enqueued"
const taskRoleActivationReasonRecovery = "recovery"
const taskRoleActivationReasonStarvation = "starvation"

type taskRoleStore interface {
	GetTask(ctx context.Context, id string) (taskpkg.Task, error)
	GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error)
	GetExecutionProfile(ctx context.Context, taskID string) (taskpkg.ExecutionProfile, error)
	ListTaskRunsByStatus(ctx context.Context, statuses []taskpkg.RunStatus) ([]taskpkg.Run, error)
}

type taskRoleSessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	Events(ctx context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error)
	ListAll(ctx context.Context) ([]*session.Info, error)
	PromptSynthetic(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

type taskRoleRuntime struct {
	drainCtx            context.Context
	cancel              context.CancelFunc
	lifecycleMu         sync.Mutex
	stopping            bool
	mu                  sync.Mutex
	store               taskRoleStore
	sessions            taskRoleSessionManager
	globalWorkspacePath string
	logger              *slog.Logger
	now                 func() time.Time
	promptInFlight      map[string]struct{}
	wg                  sync.WaitGroup
}

type taskRoleActivation struct {
	TaskID               string
	RunID                string
	Scope                taskpkg.Scope
	WorkspaceID          string
	WorkspacePath        string
	AgentName            string
	Provider             string
	Model                string
	NetworkParticipation *participation.Spec
	Title                string
	Profile              *taskpkg.ExecutionProfile
	Capabilities         []string
	Designation          taskpkg.RunDesignation
	HasDesignation       bool
}

var _ taskRunEnqueuedObserver = (*taskRoleRuntime)(nil)

func newTaskRoleRuntime(
	store taskRoleStore,
	sessions taskRoleSessionManager,
	globalWorkspacePath string,
	logger *slog.Logger,
	now func() time.Time,
) (*taskRoleRuntime, error) {
	if store == nil {
		return nil, errors.New("daemon: task role runtime requires task store")
	}
	if sessions == nil {
		return nil, errors.New("daemon: task role runtime requires session manager")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	// #nosec G118 -- cancel is owned by taskRoleRuntime and invoked from shutdown.
	drainCtx, cancel := context.WithCancel(context.Background())
	return &taskRoleRuntime{
		drainCtx:            drainCtx,
		cancel:              cancel,
		store:               store,
		sessions:            sessions,
		globalWorkspacePath: strings.TrimSpace(globalWorkspacePath),
		logger:              logger,
		now:                 now,
		promptInFlight:      make(map[string]struct{}),
	}, nil
}

func (r *taskRoleRuntime) OnTaskRunEnqueued(ctx context.Context, payload hookspkg.TaskRunEnqueuedPayload) {
	if r == nil {
		return
	}
	runID := strings.TrimSpace(payload.RunID)
	if runID == "" {
		r.logTaskRoleError("daemon: task role enqueue payload missing run id", nil, payload)
		return
	}
	if !r.beginEnqueuedActivation() {
		return
	}
	go r.activateEnqueuedRun(ctx, runID, payload)
}

func (r *taskRoleRuntime) beginEnqueuedActivation() bool {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	if r.stopping {
		return false
	}
	// Keep Add ordered before shutdown starts waiting on the owned work.
	r.wg.Add(1)
	return true
}

func (r *taskRoleRuntime) activateEnqueuedRun(
	ctx context.Context,
	runID string,
	payload hookspkg.TaskRunEnqueuedPayload,
) {
	defer r.wg.Done()
	ctx, cancel := taskRunActivationContext(ctx)
	defer cancel()
	stopDrainCancellation := context.AfterFunc(r.drainCtx, cancel)
	defer stopDrainCancellation()

	run, err := r.store.GetTaskRun(ctx, runID)
	if err != nil {
		r.logTaskRoleError("daemon: load task run for role activation", err, payload)
		return
	}
	taskRecord, err := r.store.GetTask(ctx, run.TaskID)
	if err != nil {
		r.logTaskRoleError("daemon: load task for role activation", err, payload)
		return
	}
	if err := r.activateRun(ctx, taskRecord, run, taskRoleActivationReasonRunEnqueued); err != nil {
		r.logTaskRoleError("daemon: activate task role session from enqueue", err, payload)
	}
}

func (r *taskRoleRuntime) Recover(ctx context.Context) {
	if r == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runs, err := r.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
	if err != nil {
		r.logTaskRoleError("daemon: list queued task runs for role recovery", err, hookspkg.TaskRunEnqueuedPayload{})
		return
	}
	for _, run := range runs {
		if run.RunKind.Normalize() != taskpkg.RunKindWorker || run.IsLoopWorker() {
			continue
		}
		taskRecord, err := r.store.GetTask(ctx, run.TaskID)
		if err != nil {
			r.logTaskRoleError("daemon: load task for role recovery", err, hookspkg.TaskRunEnqueuedPayload{
				TaskRunContext: hookspkg.TaskRunContext{
					RunID:                        run.ID,
					TaskID:                       run.TaskID,
					ResolvedNetworkParticipation: new(run.NetworkSpecSnapshot()),
				},
			})
			continue
		}
		if err := r.activateRun(ctx, taskRecord, run, taskRoleActivationReasonRecovery); err != nil {
			r.logTaskRoleError(
				"daemon: recover task role session for queued run",
				err,
				hookspkg.TaskRunEnqueuedPayload{
					TaskRunContext: hookspkg.TaskRunContext{
						RunID:                        run.ID,
						TaskID:                       run.TaskID,
						WorkspaceID:                  taskRecord.WorkspaceID,
						ResolvedNetworkParticipation: new(run.NetworkSpecSnapshot()),
					},
				},
			)
		}
	}
}

func (r *taskRoleRuntime) activateRun(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	reason string,
) error {
	if ctx == nil {
		return errors.New("daemon: task role activation context is required")
	}
	activation, ok, err := r.activationForRun(ctx, taskRecord, run)
	if err != nil || !ok {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.activateRoleSession(ctx, activation, reason, r.startRoleSession)
	if err != nil {
		return err
	}
	if !result.created {
		r.logger.Info(
			"daemon: task role session already active",
			"session_id", result.info.ID,
			taskRoleRuntimeTaskIDKey, activation.TaskID,
			"run_id", activation.RunID,
			"agent_name", activation.AgentName,
			"channel", activation.NetworkParticipation.ChannelID,
			"reason", reason,
		)
		return nil
	}
	r.logger.Info(
		"daemon: task role session started",
		"session_id", result.info.ID,
		taskRoleRuntimeTaskIDKey, activation.TaskID,
		"run_id", activation.RunID,
		"agent_name", activation.AgentName,
		"channel", activation.NetworkParticipation.ChannelID,
		"reason", reason,
	)
	return nil
}

func (r *taskRoleRuntime) activationForRun(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
) (taskRoleActivation, bool, error) {
	if run.Status.Normalize() != taskpkg.TaskRunStatusQueued {
		return taskRoleActivation{}, false, nil
	}
	if run.RunKind.Normalize() != taskpkg.RunKindWorker {
		return taskRoleActivation{}, false, nil
	}
	switch taskRecord.Status.Normalize() {
	case taskpkg.TaskStatusDraft, taskpkg.TaskStatusBlocked, taskpkg.TaskStatusCanceled:
		return taskRoleActivation{}, false, nil
	default:
	}
	agentName, provider, model, profile, ok, err := r.workerTargetForRun(ctx, taskRecord)
	if err != nil || !ok {
		return taskRoleActivation{}, false, err
	}
	if agentName == "" {
		return taskRoleActivation{}, false, nil
	}

	activation := taskRoleActivation{
		TaskID:               strings.TrimSpace(taskRecord.ID),
		RunID:                strings.TrimSpace(run.ID),
		Scope:                taskRecord.Scope.Normalize(),
		WorkspaceID:          strings.TrimSpace(taskRecord.WorkspaceID),
		AgentName:            agentName,
		Provider:             provider,
		Model:                model,
		NetworkParticipation: new(run.NetworkSpecSnapshot()),
		Title:                strings.TrimSpace(taskRecord.Title),
		Profile:              profile,
		Capabilities: append(
			append([]string(nil), run.RequiredCapabilities...),
			profileRequiredWorkerCapabilities(profile)...,
		),
	}
	applyTaskRoleDesignation(&activation, run)
	if err := r.applyActivationScope(&activation, taskRecord.ID); err != nil {
		return taskRoleActivation{}, false, err
	}
	return activation, true, nil
}

func (r *taskRoleRuntime) workerTargetForRun(
	ctx context.Context,
	taskRecord taskpkg.Task,
) (agentName string, provider string, model string, profile *taskpkg.ExecutionProfile, ok bool, err error) {
	if taskRecord.Owner != nil && !taskRecord.Owner.IsZero() {
		owner := *taskRecord.Owner
		if owner.Kind.Normalize() != taskpkg.OwnerKindPool {
			return "", "", "", nil, false, nil
		}
		return strings.TrimSpace(owner.Ref), "", "", nil, true, nil
	}

	loaded, err := r.store.GetExecutionProfile(ctx, taskRecord.ID)
	if err != nil {
		if errors.Is(err, taskpkg.ErrExecutionProfileNotFound) {
			return "", "", "", nil, false, nil
		}
		return "", "", "", nil, false, err
	}
	worker := loaded.Worker
	if worker.Mode.Normalize() != taskpkg.WorkerModeSelect {
		return "", "", "", nil, false, nil
	}
	// The exact worker name wins; selector lists only provide fallback candidates.
	agentName = strings.TrimSpace(worker.AgentName)
	if agentName == "" && len(worker.PreferredAgentNames) > 0 {
		agentName = strings.TrimSpace(worker.PreferredAgentNames[0])
	}
	if agentName == "" && len(worker.AllowedAgentNames) == 1 {
		agentName = strings.TrimSpace(worker.AllowedAgentNames[0])
	}
	if agentName == "" {
		return "", "", "", nil, false, nil
	}
	return agentName, strings.TrimSpace(worker.Provider), strings.TrimSpace(worker.Model), &loaded, true, nil
}

func (r *taskRoleRuntime) applyActivationScope(activation *taskRoleActivation, taskID string) error {
	switch activation.Scope {
	case taskpkg.ScopeWorkspace:
		if activation.WorkspaceID == "" {
			return fmt.Errorf("%w: workspace-scoped task %q has no workspace id", taskpkg.ErrValidation, taskID)
		}
	case taskpkg.ScopeGlobal:
		if r.globalWorkspacePath == "" {
			return errors.New("daemon: task role global workspace path is required")
		}
		activation.WorkspacePath = r.globalWorkspacePath
	default:
		return fmt.Errorf("%w: unsupported task scope %q for role activation", taskpkg.ErrValidation, activation.Scope)
	}
	return nil
}
