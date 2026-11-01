package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

var (
	// errStopDeclined marks a guarded stop whose condition no longer holds. Nothing stopped.
	errStopDeclined = errors.New("session: stop declined")
	// errStopDeclinedEnded is the declined stop of a session that is no longer live.
	errStopDeclinedEnded = fmt.Errorf("%w: session ended", errStopDeclined)
)

// stopGuard decides whether a stop may commit. It runs with m.lifecycleMu held and
// returns errStopDeclined, errStopDeclinedEnded, or the error that kept it from deciding.
type stopGuard func(ctx context.Context, session *Session) error

// ParentStopOutcome reports what StopIfParentStopped did.
type ParentStopOutcome int

const (
	// ParentStopCommitted means the condition held and the session was stopped.
	ParentStopCommitted ParentStopOutcome = iota
	// ParentStopSuperseded means the session keeps running: it was resumed or taken
	// over, its parent came back, or the condition could not be read.
	ParentStopSuperseded
	// ParentStopEnded means the session already stopped or disappeared.
	ParentStopEnded
)

// StopIfParentStopped stops a spawned session only while its parent-stop binding
// still applies. The condition is decided in the same lifecycleMu section that
// moves the session to stopping. Resume and take-over change lineage under
// lifecycleMu, and a parent becomes live under it, so each of them orders entirely
// before the decision, which then declines, or after it, against a child already
// stopping. onCommitted runs once the stop is committed, before termination begins
// and outside every lock. A condition that no longer holds is an outcome, not an error.
func (m *Manager) StopIfParentStopped(
	ctx context.Context,
	id string,
	detail string,
	onCommitted func(),
) (ParentStopOutcome, error) {
	if m == nil {
		return ParentStopEnded, errors.New("session: manager is required")
	}
	committed := false
	_, err := m.guardedStopWithCauseReceipt(ctx, id, CauseUserRequested, detail, m.parentStopGuard, func() {
		committed = true
		if onCommitted != nil {
			onCommitted()
		}
	})
	switch {
	case committed:
		return ParentStopCommitted, err
	case errors.Is(err, errStopDeclinedEnded), errors.Is(err, ErrSessionNotFound):
		return ParentStopEnded, nil
	case errors.Is(err, errStopDeclined):
		return ParentStopSuperseded, nil
	case err != nil:
		return ParentStopSuperseded, err
	default:
		// Another stop owned the run: the session ended without this request.
		return ParentStopEnded, nil
	}
}

// commitStopIntent moves session to stopping, deciding guard atomically with that
// transition. Lock order is goalStopMu or the launch commit, then lifecycleMu, then
// session.mu; lifecycleMu holders never stop a session. Only the in-memory transition
// needs the lock: from there on the session is stopping, which neither resume nor
// take-over admits. The caller persists after lifecycleMu is released.
func (m *Manager) commitStopIntent(
	ctx context.Context,
	session *Session,
	cause StopCause,
	detail string,
	guard stopGuard,
) (bool, <-chan struct{}, error) {
	if guard == nil {
		return session.prepareStop(m.now(), cause, detail)
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if err := guard(ctx, session); err != nil {
		return false, nil, err
	}
	return session.prepareStop(m.now(), cause, detail)
}

// parentStopGuard admits a live spawned session whose auto_stop_on_parent binding
// still names a parent that is not live.
func (m *Manager) parentStopGuard(ctx context.Context, session *Session) error {
	current, ok := m.Get(session.ID)
	if !ok {
		return errStopDeclinedEnded
	}
	if current != session {
		return errStopDeclined
	}
	info := session.Info()
	if !liveState(info.State) {
		return errStopDeclinedEnded
	}
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if info.Type != SessionTypeSpawned || !lineage.AutoStopOnParent || lineage.ParentSessionID == "" {
		return errStopDeclined
	}
	state, reason, err := m.sessionStopState(ctx, lineage.ParentSessionID)
	if err != nil {
		return fmt.Errorf("session: read parent %q of %q: %w", lineage.ParentSessionID, session.ID, err)
	}
	if liveState(state) || SubagentSurvivesParentStop(lineage, reason) {
		return errStopDeclined
	}
	return nil
}

// SubagentSurvivesParentStop exempts a subagent whose parent stopped for daemon
// shutdown or a crash: the parent's next resume picks its subagents back up.
func SubagentSurvivesParentStop(lineage *store.SessionLineage, parentStop store.StopReason) bool {
	return lineage.SpawnRole == store.SubagentSpawnRole &&
		(parentStop == store.StopShutdown || parentStop == store.StopAgentCrashed)
}

// sessionStopState reads id by the same rule the spawn reaper applies: a pending
// start, the in-memory run when there is one, else the persisted state. A deleted
// session reports no state.
func (m *Manager) sessionStopState(ctx context.Context, id string) (State, store.StopReason, error) {
	id = strings.TrimSpace(id)
	if m.isPending(id) {
		return StateStarting, "", nil
	}
	if session, ok := m.Get(id); ok && session != nil {
		info := session.Info()
		return info.State, info.StopReason, nil
	}
	meta, err := m.readSessionMetaReadOnly(ctx, id)
	if errors.Is(err, ErrSessionNotFound) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return State(meta.State), sessionMetaStopReason(&meta), nil
}

func liveState(state State) bool {
	return state == StateStarting || state == StateActive || state == StateStopping
}
