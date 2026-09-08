package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
)

type recoveredStopReceipt struct {
	Version           int                 `json:"version"`
	SessionID         string              `json:"session_id"`
	WorkspaceID       string              `json:"workspace_id"`
	RuntimeGeneration int64               `json:"runtime_generation"`
	CreatedAt         time.Time           `json:"created_at"`
	TurnID            string              `json:"turn_id"`
	StartedAt         time.Time           `json:"started_at"`
	ActorID           string              `json:"actor_id,omitempty"`
	Outcome           StopOutcome         `json:"outcome"`
	Detail            string              `json:"detail,omitempty"`
	TerminalEvent     *store.SessionEvent `json:"terminal_event,omitempty"`
}

func (m *Manager) recoveredStopReceiptPath(id string) string {
	return filepath.Join(m.homePaths.SessionsDir, id, ".stop-settlement.json")
}

func (m *Manager) prepareClassifiedExitSettlement(before, after *store.SessionMeta) error {
	if (before.State != string(StateActive) && before.State != string(StateStopping) &&
		before.State != string(StateStarting)) ||
		after.State != string(StateStopped) ||
		!inactiveProcessExitVerified(before) {
		return nil
	}
	turnID, err := m.newPromptTurnID()
	if err != nil {
		return err
	}
	cause := CauseProcessExited
	if interruptedStartupMeta(before) {
		cause = CauseFailed
	}
	settlement := &recoveredStopSettlement{
		turnID:    turnID,
		startedAt: m.now(),
		detail:    after.StopDetail,
		outcome: StopOutcome{
			FinalState: StateStopped,
			Verified:   true,
			Phase:      StopPhaseCooperative,
			Cause:      cause,
		},
		receipt: recoveredStopReceipt{
			Version: 1, SessionID: before.ID, WorkspaceID: before.WorkspaceID,
			RuntimeGeneration: before.RuntimeGeneration, CreatedAt: before.CreatedAt,
		},
	}
	if err := m.writeRecoveredStopReceipt(before.ID, settlement); err != nil {
		return err
	}
	_, err = m.restoreRecoveredStopReceipt(after)
	return err
}

func (m *Manager) writeRecoveredStopReceipt(id string, settlement *recoveredStopSettlement) error {
	receipt := settlement.receipt
	receipt.TurnID, receipt.StartedAt = settlement.turnID, settlement.startedAt
	receipt.ActorID, receipt.Outcome, receipt.Detail = settlement.actorID, settlement.outcome, settlement.detail
	content, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	if err := fileutil.AtomicWriteFile(m.recoveredStopReceiptPath(id), content, 0o600); err != nil {
		return fmt.Errorf("%w: persist recovered stop receipt for %s: %w", ErrRecoveryPersistence, id, err)
	}
	return nil
}

func (m *Manager) restoreRecoveredStopReceipt(meta *store.SessionMeta) (bool, error) {
	content, err := os.ReadFile(m.recoveredStopReceiptPath(meta.ID))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w: read recovered stop receipt: %w", ErrRecoveryPersistence, err)
	}
	var receipt recoveredStopReceipt
	if err := json.Unmarshal(content, &receipt); err != nil {
		return false, fmt.Errorf("%w: decode recovered stop receipt: %w", ErrRecoveryPersistence, err)
	}
	if err := receipt.validate(meta); err != nil {
		return false, err
	}
	settlement := &recoveredStopSettlement{
		turnID: receipt.TurnID, startedAt: receipt.StartedAt, actorID: receipt.ActorID,
		outcome: receipt.Outcome, detail: receipt.Detail, receipt: receipt,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if run := m.stopRuns[meta.ID]; run != nil {
		if signalClosed(run.done) && run.outcome.Verified && run.recoveredSettlement == nil {
			return true, nil
		}
		if !signalClosed(run.ready) && run.recoveredID == meta.ID && run.recoveredSettlement == nil {
			run.recoveredSettlement = settlement
		}
		return true, nil
	}
	run := &sessionStopRun{
		recoveredID: meta.ID, recoveredSettlement: settlement, outcome: receipt.Outcome,
		ready: make(chan struct{}), done: make(chan struct{}),
		err: fmt.Errorf("%w: recovered stop for %s has pending persistence", ErrRecoveryPersistence, meta.ID),
	}
	close(run.ready)
	close(run.done)
	if m.stopRuns == nil {
		m.stopRuns = make(map[string]*sessionStopRun)
	}
	m.stopRuns[meta.ID] = run
	return true, nil
}

func (m *Manager) removeRecoveredStopReceipt(id string) error {
	err := fileutil.AtomicRemoveFile(m.recoveredStopReceiptPath(id))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: remove recovered stop receipt for %s: %w", ErrRecoveryPersistence, id, err)
	}
	return nil
}

func (receipt *recoveredStopReceipt) validate(meta *store.SessionMeta) error {
	if err := receipt.validateTerminalEvent(meta); err != nil {
		return err
	}

	if receipt.Version != 1 || receipt.SessionID != meta.ID || receipt.WorkspaceID != meta.WorkspaceID ||
		receipt.RuntimeGeneration != meta.RuntimeGeneration || !receipt.CreatedAt.Equal(meta.CreatedAt) ||
		receipt.TurnID == "" || receipt.StartedAt.IsZero() || !receipt.Outcome.Verified ||
		receipt.Outcome.FinalState != StateStopped ||
		(meta.State != string(StateActive) && meta.State != string(StateStarting) &&
			meta.State != string(StateStopping) && meta.State != string(StateStopped)) {
		return fmt.Errorf("%w: invalid recovered stop receipt for %s", ErrRecoveryPersistence, meta.ID)
	}
	return nil
}

func (receipt *recoveredStopReceipt) validateTerminalEvent(meta *store.SessionMeta) error {
	event := receipt.TerminalEvent
	if event == nil {
		return nil
	}
	if event.ID == "" || event.SessionID != meta.ID || event.Type != EventTypeSessionStopped ||
		event.TurnID != receipt.TurnID || event.Timestamp.IsZero() || !json.Valid([]byte(event.Content)) {
		return fmt.Errorf("%w: invalid terminal event in stop receipt for %s", ErrRecoveryPersistence, meta.ID)
	}
	return nil
}
