package globaldb

import (
	"encoding/json"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

type taskBlockWatchEventPayload struct {
	BlockID        string            `json:"block_id"`
	BlockKind      taskpkg.BlockKind `json:"block_kind"`
	Reason         string            `json:"reason"`
	ExpiresAt      time.Time         `json:"expires_at,omitzero"`
	ClearNote      string            `json:"clear_note,omitempty"`
	ClearedAt      time.Time         `json:"cleared_at,omitzero"`
	ClaimTokenHash string            `json:"claim_token_hash,omitempty"`
}

type taskAttentionWatchEventPayload struct {
	Status          taskpkg.Status    `json:"status"`
	Reason          string            `json:"reason,omitempty"`
	At              time.Time         `json:"at,omitzero"`
	BlockID         string            `json:"block_id,omitempty"`
	BlockKind       taskpkg.BlockKind `json:"block_kind,omitempty"`
	RecurrenceCount int               `json:"recurrence_count,omitempty"`
}

type taskRecoveredWatchEventPayload struct {
	Status taskpkg.Status `json:"status"`
	Note   string         `json:"note,omitempty"`
	At     time.Time      `json:"at,omitzero"`
}

type taskRunCompletedWatchEventPayload struct {
	Status         taskpkg.RunStatus `json:"status"`
	Result         json.RawMessage   `json:"result,omitempty"`
	ClaimTokenHash string            `json:"claim_token_hash,omitempty"`
}

type taskRunFailedWatchEventPayload struct {
	Status         taskpkg.RunStatus `json:"status"`
	Error          string            `json:"error"`
	Metadata       json.RawMessage   `json:"metadata,omitempty"`
	ClaimTokenHash string            `json:"claim_token_hash,omitempty"`
}
