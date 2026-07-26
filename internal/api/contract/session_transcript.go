package contract

import "github.com/compozy/agh/internal/transcript"

const (
	SessionStreamFrameRaw        = "raw"
	SessionStreamFrameTranscript = "transcript"

	SessionStreamEventTranscriptSnapshot  = "transcript_snapshot"
	SessionStreamEventTranscriptDelta     = "transcript_delta"
	SessionStreamEventGoalSnapshotChanged = "goal_snapshot_changed"

	TranscriptSnapshotReasonFenceMissing       = "fence_missing"
	TranscriptSnapshotReasonEpochMismatch      = "epoch_mismatch"
	TranscriptSnapshotReasonGenerationMismatch = "generation_mismatch"
	TranscriptSnapshotReasonSequenceReset      = "sequence_reset"
)

// SessionTranscriptResponse wraps one bounded chronological transcript page.
type SessionTranscriptResponse struct {
	Entries            []transcript.Entry `json:"entries"`
	Epoch              int64              `json:"epoch"`
	Generation         int64              `json:"generation"`
	MaxSequence        int64              `json:"max_sequence"`
	HasOlder           bool               `json:"has_older"`
	NextBeforeSequence int64              `json:"next_before_sequence,omitempty"`
	Limit              int                `json:"limit"`
}

// TranscriptSnapshotPayload seeds or explicitly resets one bounded transcript tail.
type TranscriptSnapshotPayload struct {
	SessionID          string             `json:"session_id"`
	WorkspaceID        string             `json:"workspace_id,omitempty"`
	WorkspacePath      string             `json:"workspace_path,omitempty"`
	Epoch              int64              `json:"epoch"`
	Generation         int64              `json:"generation"`
	Entries            []transcript.Entry `json:"entries"`
	MaxSequence        int64              `json:"max_sequence"`
	HasOlder           bool               `json:"has_older"`
	NextBeforeSequence int64              `json:"next_before_sequence,omitempty"`
	Reset              bool               `json:"reset"`
	Reason             string             `json:"reason,omitempty"`
}

// TranscriptDeltaPayload carries one bounded batch of materialized entry upserts.
type TranscriptDeltaPayload struct {
	SessionID     string             `json:"session_id"`
	WorkspaceID   string             `json:"workspace_id,omitempty"`
	WorkspacePath string             `json:"workspace_path,omitempty"`
	Epoch         int64              `json:"epoch"`
	Generation    int64              `json:"generation"`
	Entries       []transcript.Entry `json:"entries"`
	Cursor        int64              `json:"cursor"`
	MaxSequence   int64              `json:"max_sequence"`
	HasMore       bool               `json:"has_more"`
}

// SessionStreamPayload documents the possible SSE frame payloads.
type SessionStreamPayload struct {
	Raw                 *SessionEventPayload        `json:"raw,omitempty"`
	TranscriptSnapshot  *TranscriptSnapshotPayload  `json:"transcript_snapshot,omitempty"`
	TranscriptDelta     *TranscriptDeltaPayload     `json:"transcript_delta,omitempty"`
	GoalSnapshotChanged *GoalSnapshotChangedPayload `json:"goal_snapshot_changed,omitempty"`
	SessionStopped      *SessionEventPayload        `json:"session_stopped,omitempty"`
}
