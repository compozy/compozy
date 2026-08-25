package contract

import (
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type TerminalControllerPayload struct {
	Kind terminalpkg.ActorKind `json:"kind"`
	ID   string                `json:"id"`
}

type TerminalRunPayload struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
}

type TerminalExitPayload struct {
	Cause  string    `json:"cause"`
	Code   *int      `json:"code,omitempty"`
	Signal *string   `json:"signal,omitempty"`
	At     time.Time `json:"at"`
}

type TerminalInfoPayload struct {
	ID           terminalpkg.ID             `json:"id"`
	WorkspaceID  string                     `json:"workspace_id"`
	ProfileID    string                     `json:"profile_id"`
	ProfileName  string                     `json:"profile_name"`
	Title        string                     `json:"title"`
	Shell        string                     `json:"shell"`
	Cwd          string                     `json:"cwd"`
	Mode         terminalpkg.Mode           `json:"mode"`
	State        string                     `json:"state"`
	Controller   *TerminalControllerPayload `json:"controller"`
	Lease        terminalpkg.LeaseState     `json:"lease"`
	Viewers      int                        `json:"viewers"`
	BoundRun     *TerminalRunPayload        `json:"bound_run"`
	Capabilities terminalpkg.Capabilities   `json:"capabilities"`
	CreatedAt    time.Time                  `json:"created_at"`
	Exit         *TerminalExitPayload       `json:"exit,omitempty"`
}

type TerminalCreateRequest struct {
	Cwd   string `json:"cwd"`
	Shell string `json:"shell"`
	Cols  uint16 `json:"cols"`
	Rows  uint16 `json:"rows"`
	Title string `json:"title"`
}

type TerminalCloseRequest struct {
	Signal terminalpkg.Signal `json:"signal"`
}

type TerminalAttachTicketRequest struct {
	Mode string `json:"mode"`
}

type TerminalExecRequest struct {
	Command string                  `json:"command"`
	Args    []string                `json:"args"`
	Cwd     string                  `json:"cwd"`
	Env     map[string]string       `json:"env"`
	YieldMs int                     `json:"yield_ms"`
	Visible bool                    `json:"visible"`
	Output  terminalpkg.OutputShape `json:"output"`
}

type TerminalWaitRequest struct {
	Until     string `json:"until"`
	Pattern   string `json:"pattern"`
	TimeoutMs int    `json:"timeout_ms"`
}

type TerminalSignalRequest struct {
	Signal terminalpkg.Signal `json:"signal"`
}

type TerminalAnswerInputRequest struct {
	Input string `json:"input"`
}

type TerminalRejectInputRequest struct {
	Reason string `json:"reason"`
}

type TerminalRecordingRequest struct {
	Action string `json:"action"`
}

type TerminalResponse struct {
	Terminal TerminalInfoPayload `json:"terminal"`
}

type TerminalListResponse struct {
	Terminals []TerminalInfoPayload `json:"terminals"`
}

type TerminalExitResponse struct {
	Exit *TerminalExitPayload `json:"exit"`
}

type TerminalAttachTicketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

type TerminalDeliveredResponse struct {
	Delivered bool `json:"delivered"`
}

type TerminalInputRequestsResponse struct {
	Requests []terminalpkg.PendingInputRequest `json:"requests"`
}

type TerminalInputAnswerResponse struct {
	DeliveredBytes int  `json:"delivered_bytes"`
	Redacted       bool `json:"redacted"`
}

type TerminalInputRejectResponse struct {
	Outcome string `json:"outcome"`
}

type TerminalRecordingResponse struct {
	Recording terminalpkg.RecordingRef `json:"recording"`
}

type TerminalJournalResponse struct {
	Entries []terminalpkg.CommandRow `json:"entries"`
	Next    *string                  `json:"next"`
}

type TerminalCatalogSnapshot struct {
	Terminals []TerminalInfoPayload `json:"terminals"`
}

type TerminalStreamFrame struct {
	Opcode  uint8  `json:"opcode"`
	Seq     uint64 `json:"seq,omitempty"`
	Payload string `json:"payload"`
}
