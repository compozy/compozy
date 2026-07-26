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
	"github.com/compozy/agh/internal/coordinator"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	coordinatorRuntimeTaskIDKey      = "task_id"
	coordinatorRuntimeWorkspaceIDKey = "workspace_id"
	coordinatorRuntimeCleanupTimeout = 5 * time.Second
)

type coordinatorTaskStore interface {
	GetTask(ctx context.Context, id string) (taskpkg.Task, error)
	GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error)
	ListTaskRunsByStatus(ctx context.Context, statuses []taskpkg.RunStatus) ([]taskpkg.Run, error)
}

type coordinatorSessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	ListAll(ctx context.Context) ([]*session.Info, error)
	PromptSynthetic(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

type coordinatorHookDispatcher interface {
	DispatchCoordinatorPreSpawn(
		context.Context,
		hookspkg.CoordinatorPreSpawnPayload,
	) (hookspkg.CoordinatorPreSpawnPayload, error)
	DispatchCoordinatorSpawned(
		context.Context,
		hookspkg.CoordinatorSpawnedPayload,
	) (hookspkg.CoordinatorSpawnedPayload, error)
	DispatchCoordinatorDecision(
		context.Context,
		hookspkg.CoordinatorDecisionPayload,
	) (hookspkg.CoordinatorDecisionPayload, error)
	DispatchCoordinatorStopped(
		context.Context,
		hookspkg.CoordinatorStoppedPayload,
	) (hookspkg.CoordinatorStoppedPayload, error)
	DispatchCoordinatorFailed(
		context.Context,
		hookspkg.CoordinatorFailedPayload,
	) (hookspkg.CoordinatorFailedPayload, error)
}

type coordinatorRuntime struct {
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	store          coordinatorTaskStore
	sessions       coordinatorSessionManager
	roles          CoordinatorRoleResolver
	hooks          coordinatorHookDispatcher
	contextOverlay taskSessionContextOverlay
	roleEvents     roleEventSummaryWriter
	logger         *slog.Logger
	now            func() time.Time
	wakeInFlight   map[string]struct{}
	wg             sync.WaitGroup
}

var _ taskRunEnqueuedObserver = (*coordinatorRuntime)(nil)
var _ sessionLifecycleObserver = (*coordinatorRuntime)(nil)

type coordinatorRuntimeOption func(*coordinatorRuntime)

func withCoordinatorTaskContextOverlay(overlay taskSessionContextOverlay) coordinatorRuntimeOption {
	return func(runtime *coordinatorRuntime) {
		if runtime != nil {
			runtime.contextOverlay = overlay
		}
	}
}

func withCoordinatorRoleEvents(events roleEventSummaryWriter) coordinatorRuntimeOption {
	return func(runtime *coordinatorRuntime) {
		if runtime != nil {
			runtime.roleEvents = events
		}
	}
}

func newCoordinatorRuntime(
	ctx context.Context,
	store coordinatorTaskStore,
	sessions coordinatorSessionManager,
	roles CoordinatorRoleResolver,
	hooks coordinatorHookDispatcher,
	logger *slog.Logger,
	now func() time.Time,
	options ...coordinatorRuntimeOption,
) (*coordinatorRuntime, error) {
	if ctx == nil {
		return nil, errors.New("daemon: coordinator runtime context is required")
	}
	if store == nil {
		return nil, errors.New("daemon: coordinator runtime requires task store")
	}
	if sessions == nil {
		return nil, errors.New("daemon: coordinator runtime requires session manager")
	}
	if roles == nil {
		return nil, errors.New("daemon: coordinator runtime requires role resolver")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	lifecycleCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	runtime := &coordinatorRuntime{
		ctx:          lifecycleCtx,
		cancel:       cancel,
		store:        store,
		sessions:     sessions,
		roles:        roles,
		hooks:        hooks,
		logger:       logger,
		now:          now,
		wakeInFlight: make(map[string]struct{}),
	}
	for _, option := range options {
		if option != nil {
			option(runtime)
		}
	}
	return runtime, nil
}

func (d *Daemon) bootCoordinator(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil || state.tasks == nil || state.tasks.store == nil {
		return nil
	}
	if state.sessions == nil {
		return errors.New("daemon: coordinator runtime requires session manager")
	}
	if state.deps.CoordinatorRole == nil {
		return errors.New("daemon: coordinator runtime requires coordinator role resolver")
	}

	runtime, err := newCoordinatorRuntime(
		ctx,
		state.tasks.store,
		state.sessions,
		state.deps.CoordinatorRole,
		state.notifier,
		state.logger,
		d.now,
		withCoordinatorTaskContextOverlay(state.situationContext),
		withCoordinatorRoleEvents(state.registry),
	)
	if err != nil {
		return err
	}
	router, err := newReviewRouter(
		state.tasks.manager,
		state.tasks.store,
		state.sessions,
		state.workspaceResolver,
		agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
		state.logger,
		d.now,
		withReviewRouterTaskContextOverlay(state.situationContext),
	)
	if err != nil {
		return err
	}
	if state.notifier != nil {
		state.notifier.AddTaskRunEnqueuedObserver(runtime)
	}
	if state.lifecycleObservers != nil {
		state.lifecycleObservers.Add(runtime)
		state.lifecycleObservers.Add(router)
	}
	if cleanup != nil {
		cleanup.add(func(cleanupCtx context.Context) error {
			return runtime.shutdown(cleanupCtx)
		})
	}
	if state.reviewRequests != nil {
		state.reviewRequests.Set(router)
	}
	runtime.Recover(ctx)
	state.coordinator = runtime
	return nil
}

func (r *coordinatorRuntime) OnTaskRunEnqueued(ctx context.Context, payload hookspkg.TaskRunEnqueuedPayload) {
	if ctx == nil {
		ctx = context.Background()
	}
	runID := strings.TrimSpace(payload.RunID)
	if runID == "" {
		r.logCoordinatorError("daemon: coordinator enqueue payload missing run id", nil, payload)
		return
	}
	run, err := r.store.GetTaskRun(ctx, runID)
	if err != nil {
		r.logCoordinatorError("daemon: load task run for coordinator enqueue", err, payload)
		return
	}
	if !run.IsTaskAnchored() {
		return
	}
	taskRecord, err := r.store.GetTask(ctx, run.TaskID)
	if err != nil {
		r.logCoordinatorError("daemon: load task for coordinator enqueue", err, payload)
		return
	}
	if _, _, err := r.bootstrapRun(ctx, taskRecord, run, coordinator.ReasonRunEnqueued); err != nil {
		r.logCoordinatorError("daemon: bootstrap coordinator from enqueue", err, payload)
	}
}

func (r *coordinatorRuntime) OnSessionCreated(context.Context, *session.Session) {
}

func (r *coordinatorRuntime) OnSessionStopped(ctx context.Context, sess *session.Session) {
	if r == nil || sess == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	info := sess.Info()
	if info == nil || info.Type != session.SessionTypeCoordinator {
		return
	}
	r.dispatchStopped(ctx, info)
	r.recoverWorkspace(ctx, strings.TrimSpace(info.WorkspaceID), coordinator.ReasonCoordinatorStopped)
}

func (r *coordinatorRuntime) Recover(ctx context.Context) {
	r.recoverWorkspace(ctx, "", coordinator.ReasonRecovery)
}

func (r *coordinatorRuntime) recoverWorkspace(ctx context.Context, workspaceID string, reason string) {
	if r == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runs, err := r.store.ListTaskRunsByStatus(ctx, coordinator.ExecutableRunStatuses())
	if err != nil {
		r.logCoordinatorError(
			"daemon: list executable runs for coordinator recovery",
			err,
			hookspkg.TaskRunEnqueuedPayload{},
		)
		return
	}

	workspaceID = strings.TrimSpace(workspaceID)
	for _, run := range runs {
		if !run.IsTaskAnchored() {
			continue
		}
		taskRecord, err := r.store.GetTask(ctx, run.TaskID)
		if err != nil {
			r.logCoordinatorError("daemon: load task for coordinator recovery", err, hookspkg.TaskRunEnqueuedPayload{
				TaskRunContext: hookspkg.TaskRunContext{
					RunID:                        run.ID,
					TaskID:                       run.TaskID,
					ResolvedNetworkParticipation: new(run.NetworkSpecSnapshot()),
				},
			})
			continue
		}
		if workspaceID != "" && strings.TrimSpace(taskRecord.WorkspaceID) != workspaceID {
			continue
		}
		if _, _, err := r.bootstrapRun(ctx, taskRecord, run, reason); err != nil {
			r.logCoordinatorError(
				"daemon: recover coordinator for executable run",
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

func (r *coordinatorRuntime) bootstrapRun(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	reason string,
) (*session.Info, bool, error) {
	if ctx == nil {
		return nil, false, errors.New("daemon: coordinator bootstrap context is required")
	}

	preflightConfig := defaultEnabledCoordinatorRole()
	preflight := coordinator.DecideBootstrap(taskRecord, run, preflightConfig)
	if !preflight.ShouldBootstrap {
		r.dispatchDecision(ctx, preflight, nil, reason, "")
		return nil, false, nil
	}

	correlation := roleInvocationCorrelation{
		WorkspaceID: strings.TrimSpace(preflight.WorkspaceID),
		Event: store.EventCorrelation{
			TaskID:          strings.TrimSpace(preflight.TaskID),
			RunID:           strings.TrimSpace(preflight.RunID),
			WorkflowID:      strings.TrimSpace(preflight.WorkflowID),
			SchedulerReason: strings.TrimSpace(reason),
			ActorKind:       string(taskpkg.ActorKindDaemon),
			ActorID:         "coordinator-runtime",
		},
	}
	ctx = withRoleInvocationCorrelation(ctx, correlation)
	cfg, err := r.roles.ResolveCoordinatorRole(ctx, preflight.WorkspaceID)
	if err != nil {
		r.dispatchFailed(ctx, preflight, nil, reason, err)
		return nil, false, fmt.Errorf("daemon: resolve coordinator role: %w", err)
	}
	decision := coordinator.DecideBootstrap(taskRecord, run, cfg)
	if !decision.ShouldBootstrap {
		r.dispatchDecision(ctx, decision, nil, reason, "")
		return nil, false, nil
	}

	r.mu.Lock()
	existing, err := r.activeCoordinator(ctx, decision.WorkspaceID)
	if err != nil {
		r.mu.Unlock()
		r.dispatchFailed(ctx, decision, nil, reason, err)
		return nil, false, err
	}
	if existing != nil {
		shouldPrompt := r.beginCoordinatorWakeLocked(existing, decision)
		r.mu.Unlock()
		existingParticipation := participation.CloneSpec(existing.NetworkParticipation)
		r.dispatchDecision(ctx, decision, existingParticipation, reason, coordinator.DecisionExisting)
		if err := r.wakeCoordinatorIfNeeded(ctx, existing, decision, reason, shouldPrompt); err != nil {
			r.dispatchFailed(ctx, decision, existingParticipation, reason, err)
			return existing, false, err
		}
		return existing, false, nil
	}
	r.mu.Unlock()

	info, createdCfg, created, err := r.createCoordinatorSession(ctx, decision, cfg, reason)
	if err != nil {
		return nil, false, err
	}
	if !created {
		return nil, false, nil
	}
	return r.reconcileCreatedCoordinator(ctx, info, decision, createdCfg, reason)
}
