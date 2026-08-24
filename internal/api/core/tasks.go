package core

import (
	"fmt"

	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

const (
	defaultTaskActorRef              = "local-user"
	taskDesignationRollupDetailLimit = 20
	taskDesignationRollupCompleted   = "completed"
	taskDesignationRollupCanceled    = "canceled"
	// #nosec G101 -- This is an HTTP header name, not a credential value.
	taskClaimTokenHeader       = "X-Compozy-Claim-Token"
	taskActionCreate           = "create"
	taskActionList             = "list"
	taskActionGet              = "get"
	taskActionInspect          = "inspect"
	taskActionDelete           = "delete"
	taskActionPublish          = "publish"
	taskActionStart            = "start"
	taskActionUpdate           = "update"
	taskActionCancel           = "cancel"
	taskActionBlock            = "block"
	taskActionListBlocks       = "list_blocks"
	taskActionClearBlock       = "clear_block"
	taskActionRecover          = "recover"
	taskActionCreateChild      = "create_child"
	taskActionAddDependency    = "add_dependency"
	taskActionRemoveDependency = "remove_dependency"
	taskActionListRuns         = "list_runs"
	taskActionGetRun           = "get_run"
	taskActionEnqueueRun       = "enqueue_run"
	taskActionFanOutRuns       = "fan_out_runs"
	taskActionStartRun         = "start_run"
	taskActionAttachRun        = "attach_run_session"
	taskActionCompleteRun      = "complete_run"
	taskActionFailRun          = "fail_run"
	taskActionForceReleaseRun  = "force_release_run"
	taskActionForceFailRun     = "force_fail_run"
	taskActionRetryRun         = "retry_run"
	taskActionRecoverRun       = "recover_run"
	taskActionBulkReleaseRuns  = "bulk_release_runs"
	taskActionBulkFailRuns     = "bulk_fail_runs"
	taskActionCancelRun        = "cancel_run"
	taskActionTimeline         = "timeline"
	taskActionStream           = "stream"
	taskActionTree             = "tree"
	taskActionGetProfile       = "get_profile"
	taskActionSetProfile       = "set_profile"
	taskActionSetWorktree      = "set_worktree_policy"
	taskActionDeleteProfile    = "delete_profile"
	taskActionRequestReview    = "request_review"
	taskActionListReviews      = "list_reviews"
	taskActionGetReview        = "get_review"
	taskActionSubmitReview     = "submit_review"
	taskActionCreateBridgeSub  = "create_bridge_notification_subscription"
	taskActionListBridgeSubs   = "list_bridge_notification_subscriptions"
	taskActionGetBridgeSub     = "get_bridge_notification_subscription"
	taskActionDeleteBridgeSub  = "delete_bridge_notification_subscription"
	taskActionPromoteNetwork   = "promote_network_thread"
	taskActionDashboard        = "dashboard"
	taskActionInbox            = "inbox"
	taskActionOverview         = "overview"
	taskActionApprove          = "approve"
	taskActionReject           = "reject"
	taskActionTriageRead       = "triage_read"
	taskActionTriageArchive    = "triage_archive"
	taskActionTriageDismiss    = "triage_dismiss"
	taskActionPauseTask        = "pause_task"
	taskActionResumeTask       = "resume_task"
	taskActionSchedulerStatus  = "scheduler_status"
	taskActionSchedulerPause   = "scheduler_pause"
	taskActionSchedulerResume  = "scheduler_resume"
	taskActionSchedulerDrain   = "scheduler_drain"
	taskActionSchedulerBacklog = "scheduler_backlog"
)

func (h *BaseHandlers) requireTaskManager(c *gin.Context) (TaskService, bool) {
	if h.Tasks == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: task service is not configured", h.transportName()),
		)
		return nil, false
	}
	return h.Tasks, true
}

func (h *BaseHandlers) requireTaskObserver(c *gin.Context) (Observer, bool) {
	if h.Observer == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			fmt.Errorf("%s: observe service is not configured", h.transportName()),
		)
		return nil, false
	}
	return h.Observer, true
}

func (h *BaseHandlers) taskActorContext(c *gin.Context, action string) (taskpkg.ActorContext, error) {
	return h.taskActorContextForWorkspace(c, action, "")
}

func (h *BaseHandlers) taskActorContextForWorkspace(
	c *gin.Context,
	action string,
	expectedWorkspaceRef string,
) (taskpkg.ActorContext, error) {
	readScope, err := h.resolveTaskActionProfileScope(c, action)
	if err != nil {
		return taskpkg.ActorContext{}, err
	}
	var actor taskpkg.ActorContext
	if h.TaskActorContextResolver != nil {
		actor, err = h.TaskActorContextResolver(c, action)
		if err != nil {
			return taskpkg.ActorContext{}, err
		}
		actor.ReadScope = readScope
		return actor, actor.Validate()
	}
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		caller, resolveErr := h.resolveAgentCallerForWorkspace(
			c.Request.Context(),
			credentials,
			"tasks."+strings.TrimSpace(action),
			expectedWorkspaceRef,
		)
		if resolveErr != nil {
			return taskpkg.ActorContext{}, resolveErr
		}
		actor = caller.Actor
		actor.ReadScope = readScope
		return actor, actor.Validate()
	}
	actor, err = taskpkg.DeriveHumanActorContext(
		defaultTaskActorRef,
		taskOriginKindForTransport(h.transportName()),
		"tasks."+strings.TrimSpace(action),
	)
	if err != nil {
		return taskpkg.ActorContext{}, err
	}
	actor.ReadScope = readScope
	return actor, actor.Validate()
}

func (h *BaseHandlers) resolveTaskActionProfileScope(
	c *gin.Context,
	action string,
) (store.ReadScope, error) {
	if isTaskReadAction(action) {
		return h.resolveProfileReadScope(c)
	}
	return h.resolveProfileMutationScope(c)
}

func isTaskReadAction(action string) bool {
	switch strings.TrimSpace(action) {
	case taskActionList,
		taskActionGet,
		taskActionInspect,
		taskActionListBlocks,
		taskActionListRuns,
		taskActionGetRun,
		taskActionTimeline,
		taskActionStream,
		taskActionTree,
		taskActionGetProfile,
		taskActionListReviews,
		taskActionGetReview,
		taskActionListBridgeSubs,
		taskActionGetBridgeSub,
		taskActionDashboard,
		taskActionInbox,
		taskActionOverview,
		taskActionTriageRead,
		taskActionSchedulerStatus,
		taskActionSchedulerBacklog:
		return true
	default:
		return false
	}
}

func taskOriginKindForTransport(name string) taskpkg.OriginKind {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(normalized, "uds"):
		return taskpkg.OriginKindUDS
	case strings.Contains(normalized, "web"):
		return taskpkg.OriginKindWeb
	case strings.Contains(normalized, "cli"):
		return taskpkg.OriginKindCLI
	default:
		return taskpkg.OriginKindHTTP
	}
}
