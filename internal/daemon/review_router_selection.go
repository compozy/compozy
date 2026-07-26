package daemon

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (r *reviewRouter) routeRunReview(
	ctx context.Context,
	notification *taskpkg.RunReviewRequestedNotification,
) (bool, string, error) {
	actor, err := taskpkg.DeriveDaemonActorContext(reviewRouterActorRef, reviewRouterOriginRef)
	if err != nil {
		return false, "", err
	}
	review := notification.Review
	taskRecord, err := r.store.GetTask(ctx, review.TaskID)
	if err != nil {
		return false, "", fmt.Errorf("daemon: review router load task %q: %w", review.TaskID, err)
	}
	run, err := r.store.GetTaskRun(ctx, review.RunID)
	if err != nil {
		return false, "", fmt.Errorf("daemon: review router load run %q: %w", review.RunID, err)
	}
	profile, err := r.tasks.GetExecutionProfile(ctx, taskRecord.ID, actor)
	if err != nil {
		return false, "", fmt.Errorf("daemon: review router load profile for task %q: %w", taskRecord.ID, err)
	}

	route, diagnostic, err := r.selectRoute(ctx, taskRecord, run, &profile, notification.Actor)
	if err != nil || diagnostic != "" {
		return false, diagnostic, err
	}
	if route.info == nil && route.create == nil {
		return false, "no eligible reviewer route", nil
	}
	info := route.info
	if info == nil {
		created, err := r.sessions.Create(ctx, *route.create)
		if err != nil {
			return false, "reviewer session create failed: " + err.Error(), err
		}
		if created == nil || created.Info() == nil {
			return false, "reviewer session create returned no session info", nil
		}
		info = created.Info()
	}

	peerID := reviewRouterPeerID(info)
	if _, err := r.tasks.BindRunReviewSession(ctx, taskpkg.BindRunReviewSessionRequest{
		ReviewID:          review.ReviewID,
		SessionID:         info.ID,
		ReviewerAgentName: info.AgentName,
		ReviewerPeerID:    peerID,
		ReviewerChannelID: info.NetworkParticipation.ChannelID,
	}, actor); err != nil {
		if route.create != nil {
			err = errors.Join(err, r.cleanupCreatedReviewerSession(ctx, info))
		}
		return false, "reviewer session binding failed: " + err.Error(), err
	}
	return true, "", nil
}

type reviewRoute struct {
	info   *session.Info
	create *session.CreateOpts
}

func (r *reviewRouter) selectRoute(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	profile *taskpkg.ExecutionProfile,
	requesterActor taskpkg.ActorContext,
) (reviewRoute, string, error) {
	resolved, err := r.resolveWorkspace(ctx, taskRecord.WorkspaceID)
	if err != nil {
		return reviewRoute{}, "review workspace resolution failed: " + err.Error(), err
	}
	review := profile.Review
	original := r.originalWorkerIdentity(ctx, taskRecord.WorkspaceID, run)
	requester := r.requesterIdentity(ctx, taskRecord.WorkspaceID, requesterActor)
	existing, err := r.selectExistingRoute(ctx, taskRecord, &review, original, requester, resolved)
	if err != nil {
		return reviewRoute{}, "", err
	}
	if existing != nil {
		return reviewRoute{info: existing}, "", nil
	}

	create, diagnostic, err := r.createRoute(ctx, taskRecord, run, &review, original, requester, resolved)
	if err != nil || diagnostic != "" {
		return reviewRoute{}, diagnostic, err
	}
	return reviewRoute{create: create}, "", nil
}

type originalWorkerIdentity struct {
	sessionID string
	agentName string
	peerID    string
}

func (r *reviewRouter) originalWorkerIdentity(
	ctx context.Context,
	workspaceID string,
	run taskpkg.Run,
) originalWorkerIdentity {
	identity := originalWorkerIdentity{sessionID: strings.TrimSpace(run.SessionID)}
	if identity.sessionID == "" && run.ClaimedBy != nil &&
		run.ClaimedBy.Kind.Normalize() == taskpkg.ActorKindAgentSession {
		identity.sessionID = strings.TrimSpace(run.ClaimedBy.Ref)
	}
	if identity.sessionID == "" {
		return identity
	}
	infos, err := r.sessions.ListAll(ctx)
	if err != nil {
		r.logger.Warn(
			"daemon: review router could not list sessions for original-worker exclusion",
			"session_id", identity.sessionID,
			"error", err,
		)
		return identity
	}
	for _, info := range infos {
		if info == nil || strings.TrimSpace(info.ID) != identity.sessionID {
			continue
		}
		if workspaceID != "" && strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) {
			continue
		}
		identity.agentName = strings.TrimSpace(info.AgentName)
		identity.peerID = reviewRouterPeerID(info)
		return identity
	}
	return identity
}

func (r *reviewRouter) requesterIdentity(
	ctx context.Context,
	workspaceID string,
	actor taskpkg.ActorContext,
) originalWorkerIdentity {
	if actor.Actor.Kind.Normalize() != taskpkg.ActorKindAgentSession {
		return originalWorkerIdentity{}
	}
	identity := originalWorkerIdentity{sessionID: strings.TrimSpace(actor.Actor.Ref)}
	if identity.sessionID == "" {
		return identity
	}
	infos, err := r.sessions.ListAll(ctx)
	if err != nil {
		r.logger.Warn(
			"daemon: review router could not list sessions for requester exclusion",
			"session_id", identity.sessionID,
			"error", err,
		)
		return identity
	}
	for _, info := range infos {
		if info == nil || strings.TrimSpace(info.ID) != identity.sessionID {
			continue
		}
		if workspaceID != "" && strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) {
			continue
		}
		identity.agentName = strings.TrimSpace(info.AgentName)
		identity.peerID = reviewRouterPeerID(info)
		return identity
	}
	return identity
}
