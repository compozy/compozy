package daemon

import (
	"context"
	"errors"

	"log/slog"

	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	reviewRouterActorRef        = "review-router"
	reviewRouterOriginRef       = "daemon.review_router"
	reviewRouterRouteTimeout    = 30 * time.Second
	reviewRouterCleanupTimeout  = 5 * time.Second
	reviewRouterNoRouteGuidance = "Update the task execution profile review selectors " +
		"or start an eligible reviewer session."
	reviewRouterNoRouteDeliveryPrefix = "review-router:no-route:"
)

type runReviewRequestedForwarder struct {
	mu     sync.RWMutex
	target taskpkg.RunReviewRequestedObserver
}

var _ taskpkg.RunReviewRequestedObserver = (*runReviewRequestedForwarder)(nil)

func newRunReviewRequestedForwarder() *runReviewRequestedForwarder {
	return &runReviewRequestedForwarder{}
}

func (f *runReviewRequestedForwarder) Set(target taskpkg.RunReviewRequestedObserver) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.target = target
}

func (f *runReviewRequestedForwarder) OnRunReviewRequested(
	ctx context.Context,
	notification *taskpkg.RunReviewRequestedNotification,
) {
	if f == nil {
		return
	}
	f.mu.RLock()
	target := f.target
	f.mu.RUnlock()
	if target == nil {
		return
	}
	target.OnRunReviewRequested(ctx, notification)
}

type reviewRouterTasks interface {
	GetExecutionProfile(
		ctx context.Context,
		taskID string,
		actor taskpkg.ActorContext,
	) (taskpkg.ExecutionProfile, error)
	ListRunReviews(
		ctx context.Context,
		query taskpkg.RunReviewQuery,
		actor taskpkg.ActorContext,
	) ([]taskpkg.RunReview, error)
	BindRunReviewSession(
		ctx context.Context,
		req taskpkg.BindRunReviewSessionRequest,
		actor taskpkg.ActorContext,
	) (taskpkg.RunReviewBinding, error)
	RecordRunReview(
		ctx context.Context,
		req taskpkg.RecordRunReviewRequest,
		actor taskpkg.ActorContext,
	) (taskpkg.RunReviewResult, error)
}

type reviewRouterTaskStore interface {
	GetTask(ctx context.Context, id string) (taskpkg.Task, error)
	GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error)
}

type reviewRouterSessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	ListAll(ctx context.Context) ([]*session.Info, error)
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

type reviewRouterAgentResolver interface {
	ResolveAgent(name string, resolved *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error)
}

type reviewRouterWorkspaceResolver interface {
	Resolve(ctx context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error)
}

type reviewRouter struct {
	tasks          reviewRouterTasks
	store          reviewRouterTaskStore
	sessions       reviewRouterSessionManager
	workspaces     reviewRouterWorkspaceResolver
	agents         reviewRouterAgentResolver
	contextOverlay taskSessionContextOverlay
	logger         *slog.Logger
	now            func() time.Time
}

var _ taskpkg.RunReviewRequestedObserver = (*reviewRouter)(nil)
var _ sessionLifecycleObserver = (*reviewRouter)(nil)

type reviewRouterOption func(*reviewRouter)

func withReviewRouterTaskContextOverlay(overlay taskSessionContextOverlay) reviewRouterOption {
	return func(router *reviewRouter) {
		if router != nil {
			router.contextOverlay = overlay
		}
	}
}

func newReviewRouter(
	tasks reviewRouterTasks,
	store reviewRouterTaskStore,
	sessions reviewRouterSessionManager,
	workspaces reviewRouterWorkspaceResolver,
	agents reviewRouterAgentResolver,
	logger *slog.Logger,
	now func() time.Time,
	options ...reviewRouterOption,
) (*reviewRouter, error) {
	if tasks == nil {
		return nil, errors.New("daemon: review router requires task manager")
	}
	if store == nil {
		return nil, errors.New("daemon: review router requires task store")
	}
	if sessions == nil {
		return nil, errors.New("daemon: review router requires session manager")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	router := &reviewRouter{
		tasks:      tasks,
		store:      store,
		sessions:   sessions,
		workspaces: workspaces,
		agents:     agents,
		logger:     logger,
		now:        now,
	}
	for _, option := range options {
		if option != nil {
			option(router)
		}
	}
	return router, nil
}

func (r *reviewRouter) OnRunReviewRequested(
	ctx context.Context,
	notification *taskpkg.RunReviewRequestedNotification,
) {
	if r == nil || notification == nil {
		return
	}
	if strings.TrimSpace(notification.Review.ReviewID) == "" {
		return
	}
	routeCtx, cancel := detachedDaemonOperationContext(ctx, reviewRouterRouteTimeout)
	defer cancel()

	routed, diagnostic, err := r.routeRunReview(routeCtx, notification)
	if err != nil {
		r.logger.Warn(
			"daemon: review router failed",
			"review_id", notification.Review.ReviewID,
			"task_id", notification.Review.TaskID,
			"run_id", notification.Review.RunID,
			"error", err,
		)
		return
	}
	if routed || strings.TrimSpace(diagnostic) == "" {
		return
	}
	recordCtx, recordCancel := detachedDaemonOperationContext(ctx, reviewRouterRouteTimeout)
	defer recordCancel()

	if err := r.recordNoRouteDiagnostic(recordCtx, notification.Review, diagnostic); err != nil {
		r.logger.Warn(
			"daemon: review router failed to record no-route diagnostic",
			"review_id", notification.Review.ReviewID,
			"task_id", notification.Review.TaskID,
			"run_id", notification.Review.RunID,
			"error", err,
		)
	}
}

func (r *reviewRouter) OnSessionCreated(context.Context, *session.Session) {
}

func (r *reviewRouter) OnSessionStopped(ctx context.Context, sess *session.Session) {
	if r == nil || sess == nil {
		return
	}
	info := sess.Info()
	if info == nil {
		return
	}
	sessionID := strings.TrimSpace(info.ID)
	if sessionID == "" {
		return
	}
	actor, err := taskpkg.DeriveDaemonActorContext(reviewRouterActorRef, reviewRouterOriginRef)
	if err != nil {
		r.logger.Warn("daemon: review router failed to derive actor for stopped reviewer recovery", "error", err)
		return
	}
	routeCtx, cancel := detachedDaemonOperationContext(ctx, reviewRouterRouteTimeout)
	defer cancel()
	reviews, err := r.tasks.ListRunReviews(
		routeCtx,
		taskpkg.RunReviewQuery{
			Status:            taskpkg.RunReviewStatusInReview,
			ReviewerSessionID: sessionID,
		},
		actor,
	)
	if err != nil {
		r.logger.Warn(
			"daemon: review router failed to list stopped reviewer bindings",
			"session_id", sessionID,
			"error", err,
		)
		return
	}
	for _, review := range reviews {
		r.recoverStoppedReviewerReview(routeCtx, review, sessionID)
	}
}

func (r *reviewRouter) recoverStoppedReviewerReview(
	ctx context.Context,
	review taskpkg.RunReview,
	stoppedSessionID string,
) {
	reviewID := strings.TrimSpace(review.ReviewID)
	if reviewID == "" {
		return
	}
	routed, diagnostic, err := r.routeRunReview(ctx, &taskpkg.RunReviewRequestedNotification{Review: review})
	if err != nil {
		r.logger.Warn(
			"daemon: review router failed to recover stopped reviewer binding",
			"review_id", reviewID,
			"task_id", review.TaskID,
			"run_id", review.RunID,
			"stopped_session_id", stoppedSessionID,
			"error", err,
		)
		return
	}
	if routed {
		return
	}
	diagnostic = stoppedReviewerDiagnostic(stoppedSessionID, diagnostic)
	if err := r.recordNoRouteDiagnostic(ctx, review, diagnostic); err != nil {
		r.logger.Warn(
			"daemon: review router failed to record stopped reviewer diagnostic",
			"review_id", reviewID,
			"task_id", review.TaskID,
			"run_id", review.RunID,
			"stopped_session_id", stoppedSessionID,
			"error", err,
		)
	}
}
