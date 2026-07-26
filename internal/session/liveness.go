package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/procutil"
	"github.com/compozy/agh/internal/store"
)

const (
	// DefaultLivenessStallAfter defines how long a session may go without ACP
	// activity before recovery marks it stalled.
	DefaultLivenessStallAfter = 2 * time.Minute

	resumeStopDetailAgentOrphaned = "daemon exited while session subprocess remained alive"
	resumeStopDetailAgentStalled  = "daemon exited while stalled session subprocess remained alive"
)

// ClassifyInactiveMetaForRecovery rewrites non-terminal persisted session
// metadata into a stopped view that captures the best available supervision
// evidence after daemon interruption.
func ClassifyInactiveMetaForRecovery(now time.Time, meta store.SessionMeta) (store.SessionMeta, bool) {
	next := meta

	switch strings.TrimSpace(meta.State) {
	case string(StateActive):
		next.State = string(StateStopped)
		next.StopReason = resumeStopReasonPointer(store.StopAgentCrashed)
		next.StopDetail = classifyInterruptedStopDetail(meta, now, resumeStopDetailAgentCrashed)
		next.Failure = interruptedSessionFailure(meta.Failure, store.FailureProcess, next.StopDetail)
		markInterruptedStall(&next, now)
		return next, sessionMetaChanged(meta, next)
	case string(StateStopping):
		next.State = string(StateStopped)
		next.StopReason = resumeStopReasonPointer(store.StopAgentCrashed)
		next.StopDetail = classifyInterruptedStopDetail(meta, now, "stop did not complete")
		next.Failure = interruptedSessionFailure(meta.Failure, store.FailureProcess, next.StopDetail)
		markInterruptedStall(&next, now)
		return next, sessionMetaChanged(meta, next)
	case string(StateStarting):
		next.State = string(StateStopped)
		next.StopReason = resumeStopReasonPointer(store.StopError)
		next.StopDetail = classifyInterruptedStopDetail(meta, now, resumeStopDetailStartIncomplete)
		next.Failure = interruptedSessionFailure(meta.Failure, store.FailureStartup, next.StopDetail)
		next.ACPSessionID = nil
		markInterruptedStall(&next, now)
		return next, sessionMetaChanged(meta, next)
	case string(StateStopped):
		if strings.TrimSpace(meta.StopDetail) == resumeStopDetailStartIncomplete && meta.ACPSessionID != nil {
			next.ACPSessionID = nil
			return next, sessionMetaChanged(meta, next)
		}
		return next, false
	default:
		return next, false
	}
}

func classifyPreviousStop(meta store.SessionMeta) (store.SessionMeta, bool) {
	return ClassifyInactiveMetaForRecovery(time.Now().UTC(), meta)
}

func interruptedSessionFailure(
	existing *store.SessionFailure,
	fallbackKind store.FailureKind,
	fallbackSummary string,
) *store.SessionFailure {
	next := store.CloneSessionFailure(existing)
	if next == nil {
		next = &store.SessionFailure{}
	}
	if next.Kind == "" {
		next.Kind = fallbackKind
	}
	if strings.TrimSpace(next.Summary) == "" {
		next.Summary = fallbackSummary
	}
	return normalizeSessionFailure(next, fallbackSummary)
}

func classifyInterruptedStopDetail(meta store.SessionMeta, now time.Time, fallback string) string {
	switch {
	case sessionMetaIsStalled(meta, now):
		return resumeStopDetailAgentStalled
	case sessionMetaOwnsLiveSubprocess(meta):
		return resumeStopDetailAgentOrphaned
	default:
		return fallback
	}
}

func markInterruptedStall(meta *store.SessionMeta, now time.Time) {
	if meta == nil {
		return
	}
	if !sessionMetaIsStalled(*meta, now) {
		if meta.Liveness != nil {
			meta.Liveness.StallState = ""
			meta.Liveness.StallReason = ""
		}
		return
	}
	if meta.Liveness == nil {
		meta.Liveness = &store.SessionLivenessMeta{}
	}
	meta.Liveness.StallState = store.SessionStallStateDetected
	if strings.TrimSpace(meta.Liveness.StallReason) == "" {
		meta.Liveness.StallReason = store.SessionStallReasonActivityTimeout
	}
}

func sessionMetaIsStalled(meta store.SessionMeta, now time.Time) bool {
	if !sessionMetaOwnsLiveSubprocess(meta) {
		return false
	}
	if meta.Liveness == nil {
		return false
	}
	if strings.TrimSpace(meta.Liveness.StallState) == store.SessionStallStateDetected {
		return true
	}
	lastActivityAt := sessionMetaLastActivityAt(meta.Liveness)
	if lastActivityAt == nil || lastActivityAt.IsZero() || now.IsZero() {
		return false
	}
	return now.UTC().Sub(lastActivityAt.UTC()) >= DefaultLivenessStallAfter
}

func sessionMetaOwnsLiveSubprocess(meta store.SessionMeta) bool {
	if meta.Liveness == nil || meta.Liveness.SubprocessPID <= 0 {
		return false
	}
	return procutil.Alive(meta.Liveness.SubprocessPID)
}

func sessionMetaChanged(before store.SessionMeta, after store.SessionMeta) bool {
	return before.State != after.State ||
		before.StopDetail != after.StopDetail ||
		sessionMetaStopReason(before) != sessionMetaStopReason(after) ||
		stringValue(before.ACPSessionID) != stringValue(after.ACPSessionID) ||
		!sessionFailureEqual(before.Failure, after.Failure) ||
		!sessionLivenessEqual(before.Liveness, after.Liveness)
}

func sessionFailureEqual(left *store.SessionFailure, right *store.SessionFailure) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftValue := left.Normalize()
	rightValue := right.Normalize()
	return leftValue == rightValue
}

func sessionLivenessEqual(left *store.SessionLivenessMeta, right *store.SessionLivenessMeta) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	}
	return left.SubprocessPID == right.SubprocessPID &&
		timesEqual(left.SubprocessStartedAt, right.SubprocessStartedAt) &&
		timesEqual(left.LastUpdateAt, right.LastUpdateAt) &&
		strings.TrimSpace(left.StallState) == strings.TrimSpace(right.StallState) &&
		strings.TrimSpace(left.StallReason) == strings.TrimSpace(right.StallReason) &&
		sessionActivityEqual(left.Activity, right.Activity)
}

func sessionMetaLastActivityAt(liveness *store.SessionLivenessMeta) *time.Time {
	if liveness == nil {
		return nil
	}
	if liveness.Activity != nil &&
		liveness.Activity.LastActivityAt != nil &&
		!liveness.Activity.LastActivityAt.IsZero() {
		return liveness.Activity.LastActivityAt
	}
	return liveness.LastUpdateAt
}

func sessionActivityEqual(left *store.SessionActivityMeta, right *store.SessionActivityMeta) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	}
	return strings.TrimSpace(left.TurnID) == strings.TrimSpace(right.TurnID) &&
		strings.TrimSpace(left.TurnSource) == strings.TrimSpace(right.TurnSource) &&
		timesEqual(left.TurnStartedAt, right.TurnStartedAt) &&
		timesEqual(left.LastActivityAt, right.LastActivityAt) &&
		strings.TrimSpace(left.LastActivityKind) == strings.TrimSpace(right.LastActivityKind) &&
		strings.TrimSpace(left.LastActivityDetail) == strings.TrimSpace(right.LastActivityDetail) &&
		strings.TrimSpace(left.CurrentTool) == strings.TrimSpace(right.CurrentTool) &&
		strings.TrimSpace(left.ToolCallID) == strings.TrimSpace(right.ToolCallID) &&
		timesEqual(left.LastProgressAt, right.LastProgressAt) &&
		left.IterationCurrent == right.IterationCurrent &&
		left.IterationMax == right.IterationMax &&
		left.IdleSeconds == right.IdleSeconds
}

func timesEqual(left *time.Time, right *time.Time) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.UTC().Equal(right.UTC())
	}
}

// AnnotateUnpersistedRecovery appends persistence failure detail to the
// recovered stopped-state metadata returned to callers when the synthetic crash
// classification could not be durably written.
func AnnotateUnpersistedRecovery(meta store.SessionMeta, err error) store.SessionMeta {
	annotated := meta
	detail := strings.TrimSpace(meta.StopDetail)
	suffix := fmt.Sprintf("classification not persisted: %v", err)
	if detail == "" {
		annotated.StopDetail = suffix
		return annotated
	}
	if strings.Contains(detail, suffix) {
		return annotated
	}
	annotated.StopDetail = detail + " (" + suffix + ")"
	return annotated
}
