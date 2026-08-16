package contract

import (
	"encoding/json"
	"time"
)

// LoopRequestPayload is the public request projection; private execution refs never appear here.
type LoopRequestPayload struct {
	LoopRunID        string          `json:"loop_run_id"`
	LoopName         string          `json:"loop_name,omitempty"`
	Generation       int             `json:"generation"`
	NodeID           string          `json:"node_id"`
	ItemIndex        int             `json:"item_index"`
	Kind             string          `json:"kind"`
	State            string          `json:"state"`
	Prompt           string          `json:"prompt"`
	Context          json.RawMessage `json:"context"`
	Expect           json.RawMessage `json:"expect,omitempty"`
	EditSchema       json.RawMessage `json:"edit_schema,omitempty"`
	RespondSchema    json.RawMessage `json:"respond_schema,omitempty"`
	ProposedPreview  json.RawMessage `json:"proposed_preview,omitempty"`
	Decisions        []string        `json:"decisions"`
	Agents           string          `json:"agents"`
	AnsweredDecision string          `json:"answered_decision,omitempty"`
	ActorKind        string          `json:"actor_kind,omitempty"`
	ActorID          string          `json:"actor_id,omitempty"`
	OpenedAt         time.Time       `json:"opened_at"`
	AnsweredAt       *time.Time      `json:"answered_at,omitempty"`
	ResolvedAt       *time.Time      `json:"resolved_at,omitempty"`
	ExpiresAt        *time.Time      `json:"expires_at,omitempty"`
}

type LoopRequestAggregates struct {
	Pending int `json:"pending"`
}

type LoopRequestsResponse struct {
	Items      []LoopRequestPayload  `json:"items"`
	Aggregates LoopRequestAggregates `json:"aggregates"`
	NextCursor string                `json:"next_cursor"`
}

type RespondLoopRequest struct {
	ItemIndex int             `json:"item_index,omitempty"`
	Decision  string          `json:"decision,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	Note      string          `json:"note,omitempty"`
}

type LoopRequestProvenance struct {
	ActorKind  string    `json:"actor_kind"`
	ActorID    string    `json:"actor_id"`
	AnsweredAt time.Time `json:"answered_at"`
}

type RespondLoopRequestResponse struct {
	OK         bool                  `json:"ok"`
	RunID      string                `json:"run_id"`
	NodeID     string                `json:"node_id"`
	Decision   string                `json:"decision"`
	State      string                `json:"state"`
	Provenance LoopRequestProvenance `json:"provenance"`
}
