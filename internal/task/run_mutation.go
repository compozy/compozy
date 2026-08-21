package task

import (
	"encoding/json"
	"strings"
	"time"
)

// RunMutationFence captures the complete mutable ownership identity of one run
// snapshot. Its fields are intentionally private so mutation commands can only
// obtain a fence by capturing an observed Run.
type RunMutationFence struct {
	runID          string
	taskID         string
	workspaceID    string
	status         RunStatus
	sessionID      string
	claimTokenHash string
	leaseUntil     time.Time
}

// NewRunMutationFence captures the fields that authorize one lifecycle write.
func NewRunMutationFence(run Run) RunMutationFence {
	return RunMutationFence{
		runID:          strings.TrimSpace(run.ID),
		taskID:         strings.TrimSpace(run.TaskID),
		workspaceID:    strings.TrimSpace(run.WorkspaceID),
		status:         run.Status.Normalize(),
		sessionID:      strings.TrimSpace(run.SessionID),
		claimTokenHash: strings.TrimSpace(run.ClaimTokenHash),
		leaseUntil:     run.LeaseUntil,
	}
}

// RunID reports the fenced run identity.
func (f RunMutationFence) RunID() string { return f.runID }

// TaskID reports the fenced task ownership identity.
func (f RunMutationFence) TaskID() string { return f.taskID }

// WorkspaceID reports the fenced workspace ownership identity.
func (f RunMutationFence) WorkspaceID() string { return f.workspaceID }

// Status reports the fenced lifecycle state.
func (f RunMutationFence) Status() RunStatus { return f.status }

// SessionID reports the fenced session binding.
func (f RunMutationFence) SessionID() string { return f.sessionID }

// ClaimTokenHash reports the fenced lease credential identity.
func (f RunMutationFence) ClaimTokenHash() string { return f.claimTokenHash }

// LeaseUntil reports the fenced lease generation.
func (f RunMutationFence) LeaseUntil() time.Time { return f.leaseUntil }

// Matches reports whether run still represents the captured ownership state.
func (f RunMutationFence) Matches(run Run) bool {
	return strings.TrimSpace(run.ID) == f.runID &&
		strings.TrimSpace(run.TaskID) == f.taskID &&
		strings.TrimSpace(run.WorkspaceID) == f.workspaceID &&
		run.Status.Normalize() == f.status &&
		strings.TrimSpace(run.SessionID) == f.sessionID &&
		strings.TrimSpace(run.ClaimTokenHash) == f.claimTokenHash &&
		run.LeaseUntil.Equal(f.leaseUntil)
}

func (f RunMutationFence) matchesLeaseOwner(run Run) bool {
	return strings.TrimSpace(run.ID) == f.runID &&
		strings.TrimSpace(run.TaskID) == f.taskID &&
		strings.TrimSpace(run.WorkspaceID) == f.workspaceID &&
		run.Status.Normalize() == f.status &&
		strings.TrimSpace(run.SessionID) == f.sessionID &&
		strings.TrimSpace(run.ClaimTokenHash) == f.claimTokenHash
}

// TerminalRunMutation declares the exact run snapshot that may win one
// terminal transition. Construction always captures lifecycle, session, claim,
// and lease identity together.
type TerminalRunMutation struct {
	next  *Run
	fence RunMutationFence
}

// NewTerminalRunMutation creates one fully fenced terminal command.
func NewTerminalRunMutation(previous Run, next Run) TerminalRunMutation {
	return TerminalRunMutation{next: &next, fence: NewRunMutationFence(previous)}
}

// NextRun reports the requested terminal record.
func (m TerminalRunMutation) NextRun() Run {
	if m.next == nil {
		return Run{}
	}
	return *m.next
}

// Fence reports the exact source snapshot required by the command.
func (m TerminalRunMutation) Fence() RunMutationFence { return m.fence }

// RunNeedsAttentionCommand declares one fully fenced needs-attention request.
type RunNeedsAttentionCommand struct {
	diagnostic string
	fence      RunMutationFence
	now        time.Time
}

// NewRunNeedsAttentionCommand captures the exact run snapshot to escalate.
func NewRunNeedsAttentionCommand(
	previous Run,
	diagnostic string,
	now ...time.Time,
) RunNeedsAttentionCommand {
	command := RunNeedsAttentionCommand{
		diagnostic: strings.TrimSpace(diagnostic),
		fence:      NewRunMutationFence(previous),
	}
	if len(now) > 0 {
		command.now = now[0].UTC()
	}
	return command
}

// Diagnostic reports the sanitized operator-facing diagnostic.
func (c RunNeedsAttentionCommand) Diagnostic() string { return c.diagnostic }

// Fence reports the exact source snapshot required by the command.
func (c RunNeedsAttentionCommand) Fence() RunMutationFence { return c.fence }

// Now reports when the needs-attention command was admitted.
func (c RunNeedsAttentionCommand) Now() time.Time { return c.now }

// RunMetadataMutation is a metadata-only CAS. It cannot alter lifecycle,
// session, lease, identity, or payload fields.
type RunMetadataMutation struct {
	RunID            string
	ExpectedMetadata json.RawMessage
	Metadata         json.RawMessage
}

// RunNeedsAttentionMutation records the run state observed and applied by one
// needs-attention transaction. Applied is false for the idempotent path.
type RunNeedsAttentionMutation struct {
	Previous Run
	Run      Run
	Applied  bool
}

// ForceReleaseRunMutation declares one force release authorized against an
// exact observed ownership snapshot. Callers cannot construct its fence field
// independently from the run they authorized.
type ForceReleaseRunMutation struct {
	fence RunMutationFence
	now   time.Time
}

// NewForceReleaseRunMutation captures the exact claimed run to release.
func NewForceReleaseRunMutation(previous Run, now time.Time) ForceReleaseRunMutation {
	return ForceReleaseRunMutation{fence: NewRunMutationFence(previous), now: now}
}

// Fence reports the exact source snapshot required by the command.
func (m ForceReleaseRunMutation) Fence() RunMutationFence { return m.fence }

// Now reports the command timestamp.
func (m ForceReleaseRunMutation) Now() time.Time { return m.now }

// ForceFailRunMutation declares one forced failure authorized against an exact
// observed ownership snapshot.
type ForceFailRunMutation struct {
	fence  RunMutationFence
	reason string
	now    time.Time
}

// NewForceFailRunMutation captures the exact queued or claimed run to fail.
func NewForceFailRunMutation(previous Run, reason string, now time.Time) ForceFailRunMutation {
	return ForceFailRunMutation{
		fence:  NewRunMutationFence(previous),
		reason: reason,
		now:    now,
	}
}

// Fence reports the exact source snapshot required by the command.
func (m ForceFailRunMutation) Fence() RunMutationFence { return m.fence }

// Reason reports the requested failure diagnostic.
func (m ForceFailRunMutation) Reason() string { return m.reason }

// Now reports the command timestamp.
func (m ForceFailRunMutation) Now() time.Time { return m.now }

// RetryRunMutation declares one retry authorized against an exact observed
// failed-run snapshot.
type RetryRunMutation struct {
	fence    RunMutationFence
	newRun   string
	origin   Origin
	metadata json.RawMessage
	queuedAt time.Time
}

// NewRetryRunMutation captures the exact failed source and immutable inputs
// for its replacement run.
func NewRetryRunMutation(
	previous Run,
	newRunID string,
	origin Origin,
	metadata json.RawMessage,
	queuedAt time.Time,
) RetryRunMutation {
	return RetryRunMutation{
		fence:    NewRunMutationFence(previous),
		newRun:   strings.TrimSpace(newRunID),
		origin:   origin,
		metadata: cloneRawJSON(metadata),
		queuedAt: queuedAt,
	}
}

// Fence reports the exact failed source snapshot required by the command.
func (m RetryRunMutation) Fence() RunMutationFence { return m.fence }

// NewRunID reports the replacement run identity.
func (m RetryRunMutation) NewRunID() string { return m.newRun }

// Origin reports the replacement run origin.
func (m RetryRunMutation) Origin() Origin { return m.origin }

// Metadata reports an isolated copy of the replacement run metadata.
func (m RetryRunMutation) Metadata() json.RawMessage { return cloneRawJSON(m.metadata) }

// QueuedAt reports the replacement run enqueue time.
func (m RetryRunMutation) QueuedAt() time.Time { return m.queuedAt }

// RecoverRunMutation declares one needs-attention recovery authorized against
// an exact observed ownership snapshot.
type RecoverRunMutation struct {
	fence    RunMutationFence
	newRun   string
	origin   Origin
	reason   string
	metadata json.RawMessage
	queuedAt time.Time
}

// NewRecoverRunMutation captures the exact needs-attention source and the
// immutable inputs for its replacement run.
func NewRecoverRunMutation(
	previous Run,
	newRunID string,
	origin Origin,
	reason string,
	metadata json.RawMessage,
	queuedAt time.Time,
) RecoverRunMutation {
	return RecoverRunMutation{
		fence:    NewRunMutationFence(previous),
		newRun:   strings.TrimSpace(newRunID),
		origin:   origin,
		reason:   reason,
		metadata: cloneRawJSON(metadata),
		queuedAt: queuedAt,
	}
}

// Fence reports the exact source snapshot required by the command.
func (m RecoverRunMutation) Fence() RunMutationFence { return m.fence }

// NewRunID reports the replacement run identity.
func (m RecoverRunMutation) NewRunID() string { return m.newRun }

// Origin reports the replacement run origin.
func (m RecoverRunMutation) Origin() Origin { return m.origin }

// Reason reports the recovery diagnostic.
func (m RecoverRunMutation) Reason() string { return m.reason }

// Metadata reports an isolated copy of the replacement run metadata.
func (m RecoverRunMutation) Metadata() json.RawMessage { return cloneRawJSON(m.metadata) }

// QueuedAt reports the replacement run enqueue time.
func (m RecoverRunMutation) QueuedAt() time.Time { return m.queuedAt }
