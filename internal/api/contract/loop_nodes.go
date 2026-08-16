package contract

import (
	"encoding/json"
	"time"
)

// LoopNodeInventoryState is the closed workspace inventory vocabulary.
type LoopNodeInventoryState string

const (
	LoopNodeInventoryWaiting     LoopNodeInventoryState = "waiting"
	LoopNodeInventoryQuarantined LoopNodeInventoryState = "quarantined"
	LoopNodeInventoryAttention   LoopNodeInventoryState = "attention"
	LoopNodeInventoryRetrying    LoopNodeInventoryState = "retrying"
)

// LoopControlProvenancePayload attributes one node-control mutation.
type LoopControlProvenancePayload struct {
	ActorKind   string    `json:"actor_kind"`
	ActorID     string    `json:"actor_id"`
	Reason      string    `json:"reason,omitempty"`
	RuleID      string    `json:"rule_id,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
}

// LoopNodeControlPayload is the public durable control state for one authored node.
type LoopNodeControlPayload struct {
	LoopRunID       string                        `json:"loop_run_id"`
	NodeID          string                        `json:"node_id"`
	Paused          bool                          `json:"paused"`
	PauseProvenance *LoopControlProvenancePayload `json:"pause_provenance,omitempty"`
	Quarantined     bool                          `json:"quarantined"`
	QuarantineEntry json.RawMessage               `json:"quarantine_entry,omitempty"`
	QuarantinedAt   *time.Time                    `json:"quarantined_at,omitempty"`
	AttentionFlag   string                        `json:"attention_flag,omitempty"`
	AttentionReason string                        `json:"attention_reason,omitempty"`
	// AttentionProducerNodeID routes a dependency-parked consumer to the
	// quarantined producer that owns the repair record.
	AttentionProducerNodeID string                        `json:"attention_producer_node_id,omitempty"`
	CancelState             string                        `json:"cancel_state,omitempty"`
	CancelProvenance        *LoopControlProvenancePayload `json:"cancel_provenance,omitempty"`
	LastEvidenceAt          *time.Time                    `json:"last_evidence_at,omitempty"`
	DeathResumeStreak       int                           `json:"death_resume_streak"`
	Revision                int64                         `json:"revision"`
	UpdatedAt               time.Time                     `json:"updated_at"`
}

// LoopNodeWaitPayload is one public durable wait cell.
type LoopNodeWaitPayload struct {
	LoopRunID         string          `json:"loop_run_id"`
	Generation        int             `json:"generation"`
	NodeID            string          `json:"node_id"`
	ItemIndex         int             `json:"item_index"`
	Kind              string          `json:"kind"`
	ResumeAt          *time.Time      `json:"resume_at,omitempty"`
	NextEscalationAt  *time.Time      `json:"next_escalation_at,omitempty"`
	EscalationCursor  int             `json:"escalation_cursor"`
	ClaimState        string          `json:"claim_state"`
	ClaimedByKind     string          `json:"claimed_by_kind,omitempty"`
	ClaimedByID       string          `json:"claimed_by_id,omitempty"`
	ClaimedAt         *time.Time      `json:"claimed_at,omitempty"`
	AdmissionFailures int             `json:"admission_failures"`
	Expect            json.RawMessage `json:"expect,omitempty"`
	IssuedEpoch       int64           `json:"issued_epoch"`
	CreatedAt         time.Time       `json:"created_at"`
	AgeSeconds        int64           `json:"age_seconds"`
}

// LoopNodeInventoryItem is one stable-ordered workspace inventory row.
type LoopNodeInventoryItem struct {
	State      LoopNodeInventoryState  `json:"state"`
	LoopRunID  string                  `json:"loop_run_id"`
	LoopName   string                  `json:"loop_name"`
	Generation int                     `json:"generation"`
	NodeID     string                  `json:"node_id"`
	ItemIndex  int                     `json:"item_index"`
	StateAt    time.Time               `json:"state_at"`
	Wait       *LoopNodeWaitPayload    `json:"wait,omitempty"`
	Control    *LoopNodeControlPayload `json:"control,omitempty"`
	Output     *LoopGenerationOutput   `json:"output,omitempty"`
}

// LoopNodeInventoryResponse is one page of workspace-scoped node state.
type LoopNodeInventoryResponse struct {
	Items      []LoopNodeInventoryItem `json:"items"`
	NextCursor string                  `json:"next_cursor,omitempty"`
}

// LoopNodePauseRequest parks one authored node.
type LoopNodePauseRequest struct {
	Mode      string `json:"mode"`
	Reason    string `json:"reason,omitempty"`
	ItemIndex *int   `json:"item_index,omitempty"`
}

// LoopNodeResumeRequest releases one authored node pause or manual wait.
type LoopNodeResumeRequest struct {
	Mode      string          `json:"mode"`
	ItemIndex *int            `json:"item_index,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// LoopNodeMutationRequest carries optional operator context for a node mutation.
type LoopNodeMutationRequest struct {
	Reason    string `json:"reason,omitempty"`
	ItemIndex *int   `json:"item_index,omitempty"`
}

type LoopNodeAmendRequest struct {
	Generation int             `json:"generation,omitempty"`
	ItemIndex  int             `json:"item_index,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	Reason     string          `json:"reason,omitempty"`
}

type LoopNodeAmendmentPayload struct {
	LoopRunID       string                     `json:"loop_run_id"`
	Generation      int                        `json:"generation"`
	NodeID          string                     `json:"node_id"`
	ItemIndex       int                        `json:"item_index"`
	Sequence        int                        `json:"amendment_seq"`
	Original        json.RawMessage            `json:"original,omitempty"`
	Amended         json.RawMessage            `json:"amended,omitempty"`
	OriginalSummary *LoopAmendmentValueSummary `json:"original_summary,omitempty"`
	AmendedSummary  *LoopAmendmentValueSummary `json:"amended_summary,omitempty"`
	ActorKind       string                     `json:"actor_kind"`
	ActorID         string                     `json:"actor_id"`
	Reason          string                     `json:"reason,omitempty"`
	CreatedAt       time.Time                  `json:"created_at"`
}

type LoopAmendmentValueSummary struct {
	ByteSize    int    `json:"byte_size"`
	ContentHash string `json:"content_hash"`
}

type LoopNodeAmendResponse struct {
	OK        bool                     `json:"ok"`
	Amendment LoopNodeAmendmentPayload `json:"amendment"`
}

// LoopMutationResponse is the shared structured answer for lifecycle verbs.
type LoopMutationResponse struct {
	OK         bool                          `json:"ok"`
	RunID      string                        `json:"run_id"`
	NodeID     string                        `json:"node_id,omitempty"`
	ItemIndex  *int                          `json:"item_index,omitempty"`
	Status     string                        `json:"status,omitempty"`
	Provenance *LoopControlProvenancePayload `json:"provenance,omitempty"`
	Control    *LoopNodeControlPayload       `json:"control,omitempty"`
}

// LoopNodeInventoryStateValues returns the closed inventory-state vocabulary.
func LoopNodeInventoryStateValues() []string {
	return []string{
		string(LoopNodeInventoryWaiting),
		string(LoopNodeInventoryQuarantined),
		string(LoopNodeInventoryAttention),
		string(LoopNodeInventoryRetrying),
	}
}
