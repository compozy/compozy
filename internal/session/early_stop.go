package session

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// EndedEarlyAttentionCode is the stable payload attention code for a child session
// whose last turn ended without a delivery and without open work to wake it.
const EndedEarlyAttentionCode = "ended_early"

// endedEarlyWakeDetail explains the early-stop wake to the parent session.
const endedEarlyWakeDetail = "Turn ended without a linked pull request and with no monitor, " +
	"subagent, reply watch, or pending interaction left to resume it."

const (
	endedEarlyPullRequestTimeout = 5 * time.Second
	endedEarlyAnswerTimeout      = 5 * time.Second
	endedEarlyWriteAttempts      = 3
	endedEarlyWriteRetryDelay    = 50 * time.Millisecond
)

// Lookup causes that prove the session cannot have a hosted pull request, as opposed
// to a transient failure that leaves delivery unknown.
const (
	endedEarlyLookupUnsupportedRemote = "unsupported_remote"
	endedEarlyLookupProviderDisabled  = "provider_disabled"
)

// recordTurnEndedEarly marks a settled child turn that stopped on its own without
// delivering. The mark is part of the durable attention projection, so the badge
// survives idle runtime release and daemon restarts until the next prompt clears it.
// A settled turn that did not end early drops a mark a failed clear left behind.
func (m *Manager) recordTurnEndedEarly(ctx context.Context, target *Session, turnState *promptTurnDispatchState) {
	if m == nil || m.attentionStore == nil || target == nil || turnState == nil {
		return
	}
	turnID := strings.TrimSpace(turnState.turnID)
	if turnID != "" && m.turnEndedEarly(ctx, target, turnState) {
		m.setEndedEarly(ctx, target, turnID)
		return
	}
	m.clearEndedEarly(ctx, target)
}

// clearEndedEarly drops the early-stop mark when the session receives its next prompt.
func (m *Manager) clearEndedEarly(ctx context.Context, target *Session) {
	if m == nil || m.attentionStore == nil || target == nil {
		return
	}
	target.mu.RLock()
	marked := target.EndedEarlyTurnID != ""
	target.mu.RUnlock()
	if marked {
		m.setEndedEarly(ctx, target, "")
	}
}

// setEndedEarly retries the durable write so a transient store failure neither hides
// an early stop nor keeps a stale mark that would alarm the parent on a later turn.
func (m *Manager) setEndedEarly(ctx context.Context, target *Session, turnID string) {
	writeCtx := context.WithoutCancel(ctx)
	var (
		commit store.SessionAttentionCommit
		err    error
	)
	for attempt := range endedEarlyWriteAttempts {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * endedEarlyWriteRetryDelay)
		}
		commit, err = m.attentionStore.MarkSessionEndedEarly(writeCtx, target.ID, turnID)
		if err == nil {
			break
		}
	}
	if err != nil {
		m.sessionLogger(target).ErrorContext(ctx, "session: persist early-stop mark failed",
			"turn_id", turnID, "attempts", endedEarlyWriteAttempts, "error", err)
		return
	}
	m.applyAttentionProjection(target.ID, commit.After)
	if turnID != "" {
		m.sessionLogger(target).InfoContext(ctx, "session.turn.ended_early", "turn_id", turnID)
	}
}

// turnEndedEarly applies the early-stop trigger: a child or orchestrated session ended
// a turn with end_turn, holds no open work signal or pending interaction, its creator
// is not waiting on this exact reply, and no pull request is linked to its branch.
func (m *Manager) turnEndedEarly(ctx context.Context, target *Session, turnState *promptTurnDispatchState) bool {
	if turnState.stopReason != acp.PromptStopReasonEndTurn || promptTurnWasCanceled(target, turnState) {
		return false
	}
	info := target.Info()
	if !endedEarlyCandidate(info) || !m.workSignals.hasExternalSources() {
		return false
	}
	if !supervisionProvesNoOpenWork(info.Supervision) ||
		len(info.PendingInteractions) > 0 || info.PendingPermissionCount > 0 || info.PendingClarifyCount > 0 {
		return false
	}
	parentID := store.NormalizeSessionLineage(info.ID, info.Lineage).ParentSessionID
	if m.creatorAwaitsChildReply(ctx, parentID, info.ID, turnState.turnID) {
		return false
	}
	delivered, known := m.sessionHasPullRequest(ctx, info)
	return known && !delivered
}

// endedEarlyCandidate limits the trigger to live sessions with a parent session.
// Subagents and daemon-internal children report through their own result paths.
func endedEarlyCandidate(info *Info) bool {
	if info == nil || info.State != StateActive || info.VerdictOnly ||
		normalizeSessionType(info.Type) == SessionTypeSystem {
		return false
	}
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if lineage.ParentSessionID == "" || lineage.ParentSessionID == strings.TrimSpace(info.ID) {
		return false
	}
	return lineage.SpawnRole != store.SubagentSpawnRole && !IsInternalSpawnRole(lineage.SpawnRole)
}

// supervisionProvesNoOpenWork requires a complete inspection with no work that can
// resume the session; agent progress only records the turn that just ended.
func supervisionProvesNoOpenWork(state *SupervisionState) bool {
	if state == nil || supervisionSourceUnknown(state) || supervisionNeedsAttention(state) {
		return false
	}
	for _, signal := range state.WorkSignals {
		if signal.Kind != WorkSignalAgentProgress {
			return false
		}
	}
	return true
}

// sessionHasPullRequest records the branch the turn left checked out, reads the cached
// pull request, and refreshes the branch once, so a pull request opened during the turn
// counts as delivery. known is false when a transient failure leaves delivery unproven.
func (m *Manager) sessionHasPullRequest(ctx context.Context, info *Info) (delivered bool, known bool) {
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), endedEarlyPullRequestTimeout)
	defer cancel()
	if err := m.saveSessionBranch(lookupCtx, info); err != nil {
		m.logger.WarnContext(ctx, "session: early-stop branch snapshot failed",
			"session_id", info.ID, "error", err)
		return false, false
	}
	views, err := m.SessionPullRequests(lookupCtx, []*Info{info})
	if err == nil && views[info.ID].PullRequest != nil {
		return true, true
	}
	if err == nil && views[info.ID].Branch == "" {
		return false, true
	}
	refreshed, refreshErr := m.RefreshSessionPullRequest(lookupCtx, info)
	if refreshErr != nil {
		known = pullRequestLookupConclusive(refreshErr, refreshed.Lookup.Cause)
		m.logger.WarnContext(ctx, "session: early-stop pull request lookup failed",
			"session_id", info.ID, "cause", refreshed.Lookup.Cause, "conclusive", known, "error", refreshErr)
		return false, known
	}
	return refreshed.PullRequest != nil, true
}

// pullRequestLookupConclusive accepts a failure that proves no hosted pull request can
// exist (no supported remote, no forge provider); timeouts and forge errors stay unknown.
func pullRequestLookupConclusive(err error, cause string) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	switch strings.TrimSpace(cause) {
	case endedEarlyLookupUnsupportedRemote, endedEarlyLookupProviderDisabled:
		return true
	default:
		return false
	}
}

// EndedEarlyAttention reports whether a needs-attention badge is the early-stop case:
// an unverified stop or a supervision source that needs attention names the cause first.
func EndedEarlyAttention(info *Info, badge Badge) bool {
	return info != nil && badge == BadgeNeedsAttention && strings.TrimSpace(info.EndedEarlyTurnID) != "" &&
		!info.StopVerificationFailed && !supervisionNeedsAttention(info.Supervision)
}

// endedEarlyAnswer reads the final answer of the early-stopped turn for the parent wake.
func (m *Manager) endedEarlyAnswer(ctx context.Context, info *Info) string {
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), endedEarlyAnswerTimeout)
	defer cancel()
	result, err := m.TurnResult(readCtx, info.ID, info.EndedEarlyTurnID)
	if err != nil {
		m.logger.WarnContext(ctx, "session: read early-stop answer failed",
			"session_id", info.ID, "turn_id", info.EndedEarlyTurnID, "error", err)
		return ""
	}
	return result.Text
}
