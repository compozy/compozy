package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	childWakeCoveredEvent    = "session.child_wake.covered"
	childWakeRegisteredEvent = "session.child_wake.registered"
)

func childWatchEventID(childID string) string { return "child-watch:" + childID }

var errChildStopReceiptMissing = errors.New("session: child stop receipt is not available")

func (m *Manager) registerChildSettleWake(ctx context.Context, info *Info) error {
	if !provenanceWakeCandidate(info) {
		return nil
	}
	parent, err := m.Status(ctx, info.Lineage.ParentSessionID)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if parent.InstanceID == "" || parent.WorkspaceID != info.WorkspaceID || parent.ProfileID != info.ProfileID {
		return nil
	}
	_, _, err = m.appendDurableSessionEvent(ctx, parent.ID, store.SessionEvent{
		ID: childWatchEventID(info.ID), SessionID: parent.ID, TurnID: info.ID,
		AgentName: "daemon", Type: childWakeRegisteredEvent, Timestamp: m.now(), Content: `{}`,
	})
	return err
}

func provenanceWakeCandidate(info *Info) bool {
	return info != nil && info.Type == SessionTypeUser && info.Lineage != nil &&
		info.Lineage.Kind == store.LineageKindProvenance && info.Lineage.ParentSessionID != ""
}

func (m *Manager) automaticChildWake(ctx context.Context, info *Info) (bool, error) {
	if !provenanceWakeCandidate(info) {
		return false, nil
	}
	parent, err := m.Status(ctx, info.Lineage.ParentSessionID)
	if errors.Is(err, ErrSessionNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Enrollment and coverage belong to the original parent, including an archived
	// predecessor. The bridge resolves and checks the instance's current main.
	eligible := parent.InstanceID != "" &&
		parent.WorkspaceID == info.WorkspaceID && parent.ProfileID == info.ProfileID &&
		info.ArchivedAt == nil && info.Later == nil &&
		info.StopReason != store.StopUserCanceled && info.StopCause != CauseClearConversation &&
		info.StopCause != CauseConversationRewind
	if !eligible {
		return false, nil
	}
	rows, err := m.Events(ctx, parent.ID, store.EventQuery{ID: childWatchEventID(info.ID), Limit: 1})
	return len(rows) > 0, err
}

func (m *Manager) automaticChildBadge(ctx context.Context, info *Info) (Badge, error) {
	badge := BadgeForInfo(info)
	if info.State == StateStopped && info.StopReason != store.StopShutdown {
		if badge == BadgeFailed {
			return BadgeFailed, nil
		}
		return BadgeStopped, nil
	}
	if info.State != StateStopped && badge != BadgeIdle && badge != BadgeDone {
		return BadgeUnknown, nil
	}
	if info.LastSettledRevision == 0 && info.State != StateStopped {
		return BadgeUnknown, nil
	}
	queue, err := m.InputQueueSummary(ctx, info.ID)
	if err != nil || queue.PendingInputs > 0 {
		return BadgeUnknown, err
	}
	rows, err := m.childCompletionReceipt(ctx, info.ID)
	if err != nil || len(rows) > 0 {
		return BadgeDone, err
	}
	if info.State == StateStopped && info.StopReason == store.StopShutdown {
		return m.interruptedChildBadge(ctx, info.ID)
	}
	return BadgeUnknown, nil
}

func (m *Manager) interruptedChildBadge(ctx context.Context, id string) (Badge, error) {
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil || meta.RestartBoundary == nil {
		return BadgeUnknown, err
	}
	interrupted, err := m.restartEventExists(ctx, id, meta.RestartBoundary.TurnID, restartInterruptedEvent)
	if interrupted {
		return BadgeStopped, err
	}
	return BadgeUnknown, err
}

func (m *Manager) childCompletionReceipt(ctx context.Context, id string) ([]store.SessionEvent, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{Type: acp.EventTypeDone, Limit: 1,
		Archive: store.EventArchiveUnarchived})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	event, err := transcript.UnmarshalAgentEvent(rows[0].Content)
	if err != nil {
		return nil, err
	}
	if event.PromptStopReason == acp.PromptStopReasonCancelled ||
		event.StopReason == string(acp.PromptStopReasonCancelled) {
		return nil, nil
	}
	inputs, err := m.Events(ctx, id, store.EventQuery{Types: []string{acp.EventTypeUserMessage,
		acp.EventTypeSyntheticReentry}, Limit: 1, Archive: store.EventArchiveUnarchived})
	if err != nil || len(inputs) == 0 || inputs[0].TurnID != rows[0].TurnID {
		return nil, err
	}
	return rows, nil
}

func (m *Manager) automaticChildWakeID(
	ctx context.Context, info *Info, reason SpawnWakeReason,
) (id, completedTurnID string, err error) {
	if reason == SpawnWakeReasonCompleted {
		rows, err := m.childCompletionReceipt(ctx, info.ID)
		if err != nil || len(rows) == 0 {
			return "", "", err
		}
		return fmt.Sprintf("session-wake:%s:completed:%s", info.ID, rows[0].ID), rows[0].TurnID, nil
	}
	// A stop receipt is stable across presence changes and distinguishes a later stop
	// after resume. Query only after finalization releases the child's recorder.
	rows, err := m.Events(ctx, info.ID, store.EventQuery{Type: EventTypeSessionStopped, Limit: 1,
		Archive: store.EventArchiveUnarchived})
	if err != nil {
		return "", "", err
	}
	if len(rows) == 0 {
		return "", "", errChildStopReceiptMissing
	}
	return automaticStoppedWakeID(info.ID, rows[0].ID, reason), "", nil
}

func automaticStoppedWakeID(id, receipt string, reason SpawnWakeReason) string {
	return fmt.Sprintf("session-wake:%s:stopped:%s:%s", id, receipt, reason)
}

func (m *Manager) suppressConversationStopWake(ctx context.Context, id string, cause StopCause) error {
	if cause != CauseClearConversation && cause != CauseConversationRewind {
		return nil
	}
	active, ok := m.Get(id)
	if !ok {
		return nil
	}
	info := active.Info()
	if info.StopCause != cause {
		return nil
	}
	info.StopCause = CauseNone
	eligible, err := m.automaticChildWake(ctx, info)
	if err != nil || !eligible {
		return err
	}
	turnID, err := m.sessionStopTurnID(active)
	if err != nil {
		return err
	}
	return m.recordChildWakeCoverage(ctx, info.Lineage.ParentSessionID, SpawnWakeEvent{
		Automatic: true, WakeEventID: automaticStoppedWakeID(id, sessionStopEventID(turnID), SpawnWakeReasonStopped),
	})
}

func (m *Manager) prepareCreatorWake(ctx context.Context, info *Info, badge Badge) (SpawnWakeEvent, bool) {
	if _, deferred := m.spawnShutdownWakeState(info, badge); deferred {
		return SpawnWakeEvent{}, false
	}
	automatic, err := m.prepareAutomaticChildBadge(ctx, info, &badge)
	if err != nil {
		m.logger.WarnContext(ctx, "session: resolve child settle wake", "session_id", info.ID, "error", err)
		return SpawnWakeEvent{}, false
	}
	reason, ok := spawnWakeReasonForBadge(badge)
	shutdown, deferred := m.spawnShutdownWakeState(info, badge)
	if !ok || deferred {
		return SpawnWakeEvent{}, false
	}
	event, err := m.prepareSpawnWakeEvent(ctx, info, badge, reason, shutdown && !automatic)
	if err != nil {
		m.logSpawnWakeSuppressed(ctx, info.Lineage.ParentSessionID, event, "delivery_failed", err)
		return event, false
	}
	event.Automatic = automatic
	if automatic {
		event.WakeEventID, event.completedTurnID, err = m.automaticChildWakeID(ctx, info, reason)
		if err != nil {
			if errors.Is(err, errChildStopReceiptMissing) {
				m.childWakePending.Delete(info.ID)
				return event, false
			}
			m.logSpawnWakeSuppressed(ctx, info.Lineage.ParentSessionID, event, "delivery_failed", err)
			return event, false
		}
		covered, err := m.automaticChildWakeCovered(ctx, info.Lineage.ParentSessionID, info, event)
		if err != nil || covered {
			if err != nil {
				m.logSpawnWakeSuppressed(ctx, info.Lineage.ParentSessionID, event, "delivery_failed", err)
			} else {
				m.childWakePending.Delete(info.ID)
			}
			return event, false
		}
	}
	if !m.reserveSpawnWakeEvent(event.WakeEventID) {
		return event, false
	}
	return event, true
}

func (m *Manager) prepareAutomaticChildBadge(ctx context.Context, info *Info, badge *Badge) (bool, error) {
	switch *badge {
	case BadgeDone, BadgeIdle, BadgeStopped, BadgeFailed:
	default:
		return false, nil
	}
	if provenanceWakeCandidate(info) {
		m.childWakePending.Store(info.ID, struct{}{})
	}
	automatic, err := m.automaticChildWake(ctx, info)
	if err != nil || !automatic {
		if err == nil {
			m.childWakePending.Delete(info.ID)
		}
		return false, err
	}
	m.mu.RLock()
	_, finalizing := m.finalizing[info.ID]
	m.mu.RUnlock()
	if finalizing {
		*badge = BadgeUnknown
		return true, nil
	}
	*badge, err = m.automaticChildBadge(ctx, info)
	if err == nil && *badge == BadgeUnknown {
		m.childWakePending.Delete(info.ID)
	}
	return true, err
}

func (m *Manager) automaticChildWakeCovered(
	ctx context.Context, creatorID string, info *Info, event SpawnWakeEvent,
) (bool, error) {
	id := event.WakeEventID + ":covered"
	rows, err := m.Events(ctx, creatorID, store.EventQuery{ID: id, Limit: 1})
	if err != nil || len(rows) > 0 {
		return len(rows) > 0, err
	}
	if !m.spawnWakeCovered(creatorID, info, event.Badge) {
		return false, nil
	}
	return true, m.recordChildWakeCoverage(ctx, creatorID, event)
}

func (m *Manager) recordChildWakeCoverage(ctx context.Context, creatorID string, event SpawnWakeEvent) error {
	if !event.Automatic {
		return nil
	}
	_, _, err := m.appendDurableSessionEvent(ctx, creatorID, store.SessionEvent{
		ID: event.WakeEventID + ":covered", SessionID: creatorID,
		TurnID: event.WakeEventID, AgentName: "daemon",
		Type: childWakeCoveredEvent, Timestamp: m.now(), Content: `{}`,
	})
	return err
}

func (m *Manager) validateAutomaticChildWake(
	ctx context.Context,
	creatorID string,
	meta *acp.PromptSyntheticMeta,
) error {
	info, err := m.Status(ctx, meta.ChildSessionID)
	if errors.Is(err, ErrSessionNotFound) {
		return fmt.Errorf("%w: child no longer exists", errSpawnWakeStale)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", errSpawnWakeLookup, err)
	}
	eligible, err := m.automaticChildWake(ctx, info)
	if err != nil {
		return fmt.Errorf("%w: %w", errSpawnWakeLookup, err)
	}
	badge, err := m.automaticChildBadge(ctx, info)
	if err != nil {
		return fmt.Errorf("%w: %w", errSpawnWakeLookup, err)
	}
	if !eligible || string(badge) != meta.Badge {
		return fmt.Errorf("%w: child advanced", errSpawnWakeStale)
	}
	if err := m.validateChildWakeCreator(ctx, info.Lineage.ParentSessionID, creatorID); err != nil {
		return err
	}
	id, _, err := m.automaticChildWakeID(ctx, info, SpawnWakeReason(meta.Reason))
	if errors.Is(err, errChildStopReceiptMissing) {
		return fmt.Errorf("%w: stop receipt no longer exists", errSpawnWakeStale)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", errSpawnWakeLookup, err)
	}
	if id != meta.WakeEventID || m.spawnWakeCovered(creatorID, info, badge) {
		return fmt.Errorf("%w: child advanced or a session monitor covers it", errSpawnWakeStale)
	}
	return nil
}

func (m *Manager) validateChildWakeCreator(ctx context.Context, parentID, creatorID string) error {
	if parentID == creatorID {
		return nil
	}
	parent, err := m.Status(ctx, parentID)
	if err != nil {
		return fmt.Errorf("%w: %w", errSpawnWakeLookup, err)
	}
	creator, err := m.Status(ctx, creatorID)
	if err != nil {
		return fmt.Errorf("%w: %w", errSpawnWakeLookup, err)
	}
	if parent.InstanceID == "" || creator.InstanceID != parent.InstanceID ||
		creator.ProfileID != parent.ProfileID || creator.WorkspaceID != parent.WorkspaceID {
		return fmt.Errorf("%w: creator is outside the owning instance", errSpawnWakeStale)
	}
	return nil
}

func (m *Manager) retryChildSettleWakes(ctx context.Context) {
	m.childWakePending.Range(func(key, _ any) bool {
		id, ok := key.(string)
		if !ok {
			return true
		}
		info, err := m.Status(ctx, id)
		if errors.Is(err, ErrSessionNotFound) {
			m.childWakePending.Delete(id)
		} else if err == nil {
			m.retryChildSettleWake(ctx, info)
		}
		return ctx.Err() == nil
	})
}

func (m *Manager) retryChildSettleWake(ctx context.Context, info *Info) {
	m.retryChildSettleWakeForTurn(ctx, info, "")
}

func (m *Manager) retryChildSettleWakeForTurn(ctx context.Context, info *Info, completedTurnID string) {
	if provenanceWakeCandidate(info) {
		m.childWakePending.Store(info.ID, struct{}{})
	}
	eligible, err := m.automaticChildWake(ctx, info)
	if err != nil {
		m.logger.WarnContext(ctx, "session: resolve pending child wake", "session_id", info.ID, "error", err)
		return
	}
	if !eligible {
		m.childWakePending.Delete(info.ID)
		return
	}
	m.dispatchSpawnWakeForTurn(ctx, info, BadgeForInfo(info), completedTurnID)
}

func (m *Manager) recoverChildSettleWakes(ctx context.Context, infos []*Info) {
	mains := make(map[string]bool)
	for _, info := range infos {
		if info.InstanceID != "" {
			mains[info.ID] = true
		}
	}
	for _, info := range infos {
		if !provenanceWakeCandidate(info) || !mains[info.Lineage.ParentSessionID] {
			continue
		}
		eligible, err := m.automaticChildWake(ctx, info)
		if err != nil {
			m.childWakePending.Store(info.ID, struct{}{})
			m.logger.WarnContext(ctx, "session: recover child wake", "session_id", info.ID, "error", err)
		} else if eligible {
			m.childWakePending.Store(info.ID, struct{}{})
		}
	}
}
