package daemon

import (
	"context"

	"fmt"

	"slices"

	"strings"

	"time"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (r *reviewRouter) isOriginalWorker(info *session.Info, original originalWorkerIdentity) bool {
	if info == nil {
		return false
	}
	if strings.TrimSpace(original.agentName) != "" &&
		strings.TrimSpace(info.AgentName) == strings.TrimSpace(original.agentName) {
		return true
	}
	if strings.TrimSpace(original.sessionID) != "" && strings.TrimSpace(info.ID) == original.sessionID {
		return true
	}
	if strings.TrimSpace(original.peerID) != "" && reviewRouterPeerID(info) == original.peerID {
		return true
	}
	return false
}

func (r *reviewRouter) cleanupCreatedReviewerSession(ctx context.Context, info *session.Info) error {
	if info == nil || strings.TrimSpace(info.ID) == "" {
		return nil
	}
	stopCtx, cancel := detachedDaemonOperationContext(ctx, reviewRouterCleanupTimeout)
	defer cancel()
	return r.sessions.StopWithCause(
		stopCtx,
		strings.TrimSpace(info.ID),
		session.CauseFailed,
		"review router bind failed",
	)
}

func detachDaemonOwnedContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func detachedDaemonOperationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(detachDaemonOwnedContext(ctx), timeout)
}

func (r *reviewRouter) recordNoRouteDiagnostic(
	ctx context.Context,
	review taskpkg.RunReview,
	diagnostic string,
) error {
	actor, err := taskpkg.DeriveDaemonActorContext(reviewRouterActorRef, reviewRouterOriginRef)
	if err != nil {
		return err
	}
	confidence := 1.0
	_, err = r.tasks.RecordRunReview(ctx, taskpkg.RecordRunReviewRequest{
		ReviewID: review.ReviewID,
		RunID:    review.RunID,
		Verdict: taskpkg.RunReviewVerdict{
			Outcome:           taskpkg.RunReviewOutcomeBlocked,
			Confidence:        &confidence,
			Reason:            normalizeReviewRouterDiagnostic(diagnostic),
			DeliveryID:        reviewRouterNoRouteDeliveryPrefix + strings.TrimSpace(review.ReviewID),
			NextRoundGuidance: reviewRouterNoRouteGuidance,
		},
	}, actor)
	return err
}

func selectorAllows(exact string, allowed []string, value string) bool {
	value = strings.TrimSpace(value)
	if strings.TrimSpace(exact) != "" && value != strings.TrimSpace(exact) {
		return false
	}
	return len(allowed) == 0 || slices.Contains(allowed, value)
}

func reviewRouterPeerID(info *session.Info) string {
	if info == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(info.AgentName)) + "." + strings.TrimSpace(info.ID)
}

func reviewSessionName(taskID string) string {
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		return "AGH Task Reviewer"
	}
	return "AGH Task Reviewer " + trimmed
}

func reviewRouterPromptOverlay(taskID string, runID string) string {
	return fmt.Sprintf(
		"Load the agh skill and read %s. Review task %s run %s and submit the verdict with submit_run_review.",
		bundledTaskReference,
		strings.TrimSpace(taskID),
		strings.TrimSpace(runID),
	)
}

func normalizeReviewRouterDiagnostic(diagnostic string) string {
	trimmed := strings.TrimSpace(diagnostic)
	if trimmed == "" {
		return "review router found no eligible reviewer"
	}
	return "review router found no eligible reviewer: " + trimmed
}

func stoppedReviewerDiagnostic(stoppedSessionID string, diagnostic string) string {
	trimmedID := strings.TrimSpace(stoppedSessionID)
	trimmedDiagnostic := strings.TrimSpace(diagnostic)
	if trimmedDiagnostic == "" {
		trimmedDiagnostic = "no eligible reviewer route is available"
	}
	if trimmedID == "" {
		return "reviewer session stopped before submitting review; " + trimmedDiagnostic
	}
	return fmt.Sprintf(
		"reviewer session %q stopped before submitting review; %s",
		trimmedID,
		trimmedDiagnostic,
	)
}

func uniqueTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
