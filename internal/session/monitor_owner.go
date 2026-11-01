package session

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// ErrMonitorOwnerStopped reports an owner that was stopped on purpose, which a monitor must not undo.
var ErrMonitorOwnerStopped = errors.New("session: monitor owner was stopped on purpose")

// MonitorOwnerIneligibleError reports a restart-stopped owner that automatic resume must not revive.
// Reason uses the restart-continuation vocabulary (disabled, archived, permissions_changed, ...).
type MonitorOwnerIneligibleError struct{ Reason string }

func (e *MonitorOwnerIneligibleError) Error() string {
	return "session: monitor owner cannot be resumed: " + e.Reason
}

// MonitorOwnerBinding snapshots a live owner's restart-eligibility binding (route, permissions,
// policy, worktree, runtime revision). It is the monitor-side counterpart of the checkpoint a
// running turn records, taken while the owner is live because an idle owner has no turn boundary.
func MonitorOwnerBinding(active *Session) (string, error) {
	if active == nil {
		return "", ErrSessionNotFound
	}
	meta := active.Meta()
	digest, err := restartPolicyDigest(active.AgentDefinition())
	if err != nil {
		return "", err
	}
	checkpoint := restartCheckpointFromMeta(&meta, "")
	checkpoint.Delivery, checkpoint.PolicyDigest = string(PromptDeliveryNormal), digest
	data, err := json.Marshal(checkpoint, json.Deterministic(true))
	if err != nil {
		return "", fmt.Errorf("session: encode monitor owner binding: %w", err)
	}
	return string(data), nil
}

// MonitorOwnerBinding reads the binding of the live session id.
func (m *Manager) MonitorOwnerBinding(id string) (string, error) {
	active, ok := m.Get(id)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}
	return MonitorOwnerBinding(active)
}

// ResumeMonitorOwner readmits an ordinary session the daemon interrupted with a restart, so a
// recovered monitor can wake it. It applies the same eligibility rules as restart continuation
// (auto-continue, archive, worktree, route, permission, and policy changes) against the binding
// captured while the owner was live, and never revives a session stopped on purpose.
func (m *Manager) ResumeMonitorOwner(ctx context.Context, id, binding string) error {
	var checkpoint store.SessionRestartCheckpoint
	if binding == "" || json.Unmarshal([]byte(binding), &checkpoint) != nil {
		return &MonitorOwnerIneligibleError{Reason: "binding_unknown"}
	}
	if active, ok := m.Get(id); ok && active.Info().State == StateActive {
		// A restart continuation or an operator may already have resumed the owner under a new route
		// or policy; an old monitor must not be delivered under it.
		return monitorOwnerBindingMatches(active, checkpoint)
	}
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return err
	}
	if normalizeSessionType(Type(meta.SessionType)) != SessionTypeUser {
		return &MonitorOwnerIneligibleError{Reason: "session_type"}
	}
	if sessionMetaStopReason(&meta) != store.StopShutdown {
		return ErrMonitorOwnerStopped
	}
	reason, err := m.restartSkipReason(ctx, &meta, checkpoint)
	if err != nil {
		return err
	}
	if reason != "" {
		return &MonitorOwnerIneligibleError{Reason: reason}
	}
	if _, held := m.restartInputsHeld.LoadOrStore(id, true); held {
		return fmt.Errorf("%w: restart recovery already running", ErrPromptInProgress)
	}
	defer m.restartInputsHeld.Delete(id)
	active, err := m.resume(ctx, id, false)
	if err != nil {
		return err
	}
	if err := monitorOwnerBindingMatches(active, checkpoint); err != nil {
		return errors.Join(err, m.StopWithCause(ctx, id, CauseShutdown, "Monitor owner needs review."))
	}
	m.restartInputsHeld.Delete(id)
	m.startNextQueuedInputPrompt(id)
	return nil
}

func monitorOwnerBindingMatches(active *Session, checkpoint store.SessionRestartCheckpoint) error {
	current := active.Meta()
	digest, err := restartPolicyDigest(active.AgentDefinition())
	if err != nil {
		return err
	}
	if !sameRestartBinding(checkpoint, &current) ||
		(checkpoint.PolicyDigest != "" && digest != checkpoint.PolicyDigest) {
		return &MonitorOwnerIneligibleError{Reason: "binding_changed"}
	}
	return nil
}

// SyntheticWakeAdmitted reports whether a synthetic wake with messageID was already admitted for the
// session. A recovered monitor uses it so a wake whose queue entry was already sent is not replayed.
func (m *Manager) SyntheticWakeAdmitted(ctx context.Context, id, messageID string) (bool, error) {
	if m.inputQueue == nil {
		return false, nil
	}
	return m.inputQueue.SyntheticAdmitted(ctx, id, messageID)
}
