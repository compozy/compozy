package session

import (
	"context"
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

const endedEarlyPullRequestTimeout = 5 * time.Second

// recordTurnEndedEarly marks a settled child turn that stopped on its own without
// delivering. The mark is part of the durable attention projection, so the badge
// survives idle runtime release and daemon restarts until the next prompt clears it.
func (m *Manager) recordTurnEndedEarly(ctx context.Context, target *Session, turnState *promptTurnDispatchState) {
	if m == nil || m.attentionStore == nil || target == nil || turnState == nil {
		return
	}
	turnID := strings.TrimSpace(turnState.turnID)
	if turnID == "" || !m.turnEndedEarly(ctx, target, turnState) {
		return
	}
	m.setEndedEarly(ctx, target, turnID)
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

func (m *Manager) setEndedEarly(ctx context.Context, target *Session, turnID string) {
	commit, err := m.attentionStore.MarkSessionEndedEarly(context.WithoutCancel(ctx), target.ID, turnID)
	if err != nil {
		m.sessionLogger(target).WarnContext(ctx, "session: persist early-stop mark failed",
			"turn_id", turnID, "error", err)
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
	return !m.sessionHasPullRequest(ctx, info)
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

// sessionHasPullRequest reads the cached pull request first and refreshes the branch
// once, so a pull request opened during the turn counts as delivery.
func (m *Manager) sessionHasPullRequest(ctx context.Context, info *Info) bool {
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), endedEarlyPullRequestTimeout)
	defer cancel()
	views, err := m.SessionPullRequests(lookupCtx, []*Info{info})
	if err == nil && views[info.ID].PullRequest != nil {
		return true
	}
	if err == nil && views[info.ID].Branch == "" {
		return false
	}
	refreshed, refreshErr := m.RefreshSessionPullRequest(lookupCtx, info)
	if refreshErr != nil {
		m.logger.WarnContext(ctx, "session: early-stop pull request lookup failed",
			"session_id", info.ID, "error", refreshErr)
		return false
	}
	return refreshed.PullRequest != nil
}

// endedEarlyWake reports whether a needs-attention badge edge is the early-stop case.
func endedEarlyWake(info *Info, badge Badge) bool {
	return info != nil && badge == BadgeNeedsAttention && strings.TrimSpace(info.EndedEarlyTurnID) != "" &&
		!info.StopVerificationFailed && !supervisionNeedsAttention(info.Supervision)
}

// endedEarlyAnswer reads the final answer of the early-stopped turn for the parent wake.
func (m *Manager) endedEarlyAnswer(ctx context.Context, info *Info) string {
	result, err := m.TurnResult(context.WithoutCancel(ctx), info.ID, info.EndedEarlyTurnID)
	if err != nil {
		m.logger.WarnContext(ctx, "session: read early-stop answer failed",
			"session_id", info.ID, "turn_id", info.EndedEarlyTurnID, "error", err)
		return ""
	}
	return result.Text
}
