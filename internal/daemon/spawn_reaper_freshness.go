package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// spawnCandidateFreshness classifies a reap candidate against a fresh read.
type spawnCandidateFreshness int

const (
	// spawnCandidateCurrent still matches the snapshot: reap it.
	spawnCandidateCurrent spawnCandidateFreshness = iota
	// spawnCandidateEnded stopped or disappeared since the snapshot.
	spawnCandidateEnded
	// spawnCandidateChanged was taken over, resumed, or freed by its parent since the snapshot.
	spawnCandidateChanged
)

// candidateFreshness rereads the child, and for a parent stop its parent, before
// any reap effect. A sweep snapshot can outlive what it saw: the child may have
// been taken over, or resumed after its parent stopped (which releases
// auto_stop_on_parent), and the parent may have been resumed. Reaping on the old
// snapshot would end a run nobody asked to stop. The reread narrows the window to
// the reap itself; it does not make the stop conditional.
func (r *spawnReaper) candidateFreshness(
	ctx context.Context,
	child *session.Info,
	reason string,
) (spawnCandidateFreshness, error) {
	current, err := r.sessions.Status(ctx, child.ID)
	if errors.Is(err, session.ErrSessionNotFound) {
		return spawnCandidateEnded, nil
	}
	if err != nil {
		return spawnCandidateCurrent, fmt.Errorf("daemon: reread spawned child %q: %w", child.ID, err)
	}
	if current == nil || !spawnReaperLiveState(current.State) {
		return spawnCandidateEnded, nil
	}
	if current.Type != child.Type {
		return spawnCandidateChanged, nil
	}
	lineage := store.NormalizeSessionLineage(current.ID, current.Lineage)
	switch reason {
	case spawnReapReasonTTLExpired:
		if lineage.TTLExpiresAt == nil || lineage.TTLExpiresAt.After(r.now().UTC()) {
			return spawnCandidateChanged, nil
		}
	case spawnReapReasonParentStopped:
		if !lineage.AutoStopOnParent {
			return spawnCandidateChanged, nil
		}
		reapable, err := r.parentStopStillReaps(ctx, lineage)
		if err != nil || !reapable {
			return spawnCandidateChanged, err
		}
	}
	return spawnCandidateCurrent, nil
}

// parentStopStillReaps rereads the parent with the same rule the sweep applied:
// a live parent, or a subagent's parent stopped by shutdown or a crash, keeps the child.
func (r *spawnReaper) parentStopStillReaps(ctx context.Context, lineage *store.SessionLineage) (bool, error) {
	parent, err := r.parentInfo(ctx, lineage.ParentSessionID)
	if err != nil || parent == nil {
		return err == nil, err
	}
	if spawnReaperLiveState(parent.State) {
		return false, nil
	}
	return !subagentSurvivesParentStop(lineage, parent), nil
}

func (r *spawnReaper) parentInfo(ctx context.Context, parentID string) (*session.Info, error) {
	parent, err := r.sessions.Status(ctx, parentID)
	if errors.Is(err, session.ErrSessionNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("daemon: reread spawn parent %q: %w", parentID, err)
	}
	return parent, nil
}

// subagentSurvivesParentStop exempts a subagent whose parent stopped for daemon
// shutdown or a crash: the parent's next resume picks its subagents back up.
func subagentSurvivesParentStop(lineage *store.SessionLineage, parent *session.Info) bool {
	return lineage.SpawnRole == store.SubagentSpawnRole &&
		(parent.StopReason == store.StopShutdown || parent.StopReason == store.StopAgentCrashed)
}
