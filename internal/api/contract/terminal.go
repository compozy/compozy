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

func TerminalInfoPayloadFromDomain(info terminalpkg.Info, profileName string) TerminalInfoPayload {
	payload := TerminalInfoPayload{
		ID: info.ID, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: profileName,
		Title: info.Title, Shell: info.Shell, Cwd: info.Cwd, Mode: info.Mode, State: info.State,
		Lease: info.Lease, Viewers: info.Viewers, Capabilities: info.Capabilities, CreatedAt: info.CreatedAt,
	}
	if info.Controller != nil {
		payload.Controller = &TerminalControllerPayload{Kind: info.Controller.Kind, ID: info.Controller.ID}
	}
	if info.BoundRun != nil {
		payload.BoundRun = &TerminalRunPayload{SessionID: info.BoundRun.SessionID, RunID: info.BoundRun.RunID}
	}
	if info.Exit != nil {
		payload.Exit = &TerminalExitPayload{
			Cause: info.Exit.Cause, Code: info.Exit.Code, Signal: info.Exit.Signal, At: info.Exit.At,
		}
	}
	return payload
}

type TerminalCommandActorPayload struct {
	Kind terminalpkg.ActorKind `json:"kind"`
	ID   string                `json:"id"`
}

type TerminalCommandRowPayload struct {
	ID          string                      `json:"command_id"`
	TerminalID  *terminalpkg.ID             `json:"terminal_id"`
	ProfileID   string                      `json:"profile_id"`
	ProfileName string                      `json:"profile_name"`
	Actor       TerminalCommandActorPayload `json:"actor"`
	Command     string                      `json:"command"`
	ArgvDigest  *string                     `json:"argv_digest,omitempty"`
	Cwd         string                      `json:"cwd"`
	StartedAt   time.Time                   `json:"started_at"`
	DurationMs  *int64                      `json:"duration_ms"`
	ExitCode    *int                        `json:"exit_code"`
	ExitSignal  *string                     `json:"signal"`
	ExitCause   string                      `json:"exit_cause"`
	DetectedBy  string                      `json:"detected_by"`
	Approval    string                      `json:"approval"`
	OutputBytes int64                       `json:"output_bytes"`
	Truncated   bool                        `json:"truncated"`
	RecordingID *string                     `json:"recording,omitempty"`
}

func TerminalCommandRowPayloadFromDomain(row terminalpkg.CommandRow, profileName string) TerminalCommandRowPayload {
	return TerminalCommandRowPayload{
		ID: row.ID, TerminalID: row.TerminalID, ProfileID: row.ProfileID, ProfileName: profileName,
		Actor:   TerminalCommandActorPayload{Kind: row.Actor.Kind, ID: row.Actor.ID},
		Command: row.Command, ArgvDigest: row.ArgvDigest, Cwd: row.Cwd, StartedAt: row.StartedAt,
		DurationMs: row.DurationMs, ExitCode: row.ExitCode, ExitSignal: row.ExitSignal,
		ExitCause: row.ExitCause, DetectedBy: row.DetectedBy, Approval: row.Approval,
		OutputBytes: row.OutputBytes, Truncated: row.Truncated, RecordingID: row.RecordingID,
	}
}

func TerminalCommandRowFromPayload(row TerminalCommandRowPayload) terminalpkg.CommandRow {
	return terminalpkg.CommandRow{
		ID: row.ID, TerminalID: row.TerminalID, ProfileID: row.ProfileID, ProfileName: row.ProfileName,
		Actor:   terminalpkg.Actor{Kind: row.Actor.Kind, ID: row.Actor.ID, ProfileID: row.ProfileID},
		Command: row.Command, ArgvDigest: row.ArgvDigest, Cwd: row.Cwd, StartedAt: row.StartedAt,
		DurationMs: row.DurationMs, ExitCode: row.ExitCode, ExitSignal: row.ExitSignal,
		ExitCause: row.ExitCause, DetectedBy: row.DetectedBy, Approval: row.Approval,
		OutputBytes: row.OutputBytes, Truncated: row.Truncated, RecordingID: row.RecordingID,
	}
}

type TerminalCreateRequest struct {
	Cwd      string `json:"cwd,omitempty"`
	Shell    string `json:"shell,omitempty"`
	Cols     uint16 `json:"cols,omitempty"`
	Rows     uint16 `json:"rows,omitempty"`
	Title    string `json:"title,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

type TerminalCloseRequest struct {
	Signal terminalpkg.Signal `json:"signal"`
}

type TerminalAttachTicketRequest struct {
	Mode     string `json:"mode"`
	ClientID string `json:"client_id"`
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
	Entries []TerminalCommandRowPayload `json:"entries"`
	Next    *string                     `json:"next"`
}

type TerminalCatalogSnapshot struct {
	Terminals []TerminalInfoPayload `json:"terminals"`
}

type TerminalStreamFrame struct {
	Opcode  uint8  `json:"opcode"`
	Seq     uint64 `json:"seq,omitempty"`
	Payload string `json:"payload"`
}
