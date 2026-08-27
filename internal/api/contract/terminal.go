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
	SessionID  string `json:"session_id"`
	RunID      string `json:"run_id"`
	Generation int64  `json:"generation"`
}

type TerminalExitPayload struct {
	Cause  TerminalExitCause   `json:"cause"`
	Code   *int                `json:"code,omitempty"`
	Signal *terminalpkg.Signal `json:"signal,omitempty"`
	At     time.Time           `json:"at"`
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
	State        TerminalState              `json:"state"`
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
		Title: info.Title, Shell: info.Shell, Cwd: info.Cwd, Mode: info.Mode, State: TerminalState(info.State),
		Lease: info.Lease, Viewers: info.Viewers, Capabilities: info.Capabilities, CreatedAt: info.CreatedAt,
	}
	if info.Controller != nil {
		payload.Controller = &TerminalControllerPayload{Kind: info.Controller.Kind, ID: info.Controller.ID}
	}
	if info.BoundRun != nil {
		payload.BoundRun = &TerminalRunPayload{
			SessionID: info.BoundRun.SessionID, RunID: info.BoundRun.RunID,
			Generation: info.BoundRun.Generation,
		}
	}
	if info.Exit != nil {
		payload.Exit = &TerminalExitPayload{
			Cause: TerminalExitCause(info.Exit.Cause), Code: info.Exit.Code,
			Signal: terminalSignalFromString(info.Exit.Signal), At: info.Exit.At,
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
	ExitSignal  *terminalpkg.Signal         `json:"signal"`
	ExitCause   TerminalExitCause           `json:"exit_cause"`
	DetectedBy  TerminalCommandDetection    `json:"detected_by"`
	Approval    TerminalCommandApproval     `json:"approval"`
	OutputBytes int64                       `json:"output_bytes"`
	Truncated   bool                        `json:"truncated"`
	RecordingID *string                     `json:"recording,omitempty"`
}

func TerminalCommandRowPayloadFromDomain(row terminalpkg.CommandRow, profileName string) TerminalCommandRowPayload {
	return TerminalCommandRowPayload{
		ID: row.ID, TerminalID: row.TerminalID, ProfileID: row.ProfileID, ProfileName: profileName,
		Actor:   TerminalCommandActorPayload{Kind: row.Actor.Kind, ID: row.Actor.ID},
		Command: row.Command, ArgvDigest: row.ArgvDigest, Cwd: row.Cwd, StartedAt: row.StartedAt,
		DurationMs: row.DurationMs, ExitCode: row.ExitCode, ExitSignal: terminalSignalFromString(row.ExitSignal),
		ExitCause: TerminalExitCause(row.ExitCause), DetectedBy: TerminalCommandDetection(row.DetectedBy),
		Approval:    TerminalCommandApproval(row.Approval),
		OutputBytes: row.OutputBytes, Truncated: row.Truncated, RecordingID: row.RecordingID,
	}
}

func TerminalCommandRowFromPayload(row TerminalCommandRowPayload) terminalpkg.CommandRow {
	return terminalpkg.CommandRow{
		ID: row.ID, TerminalID: row.TerminalID, ProfileID: row.ProfileID, ProfileName: row.ProfileName,
		Actor:   terminalpkg.Actor{Kind: row.Actor.Kind, ID: row.Actor.ID, ProfileID: row.ProfileID},
		Command: row.Command, ArgvDigest: row.ArgvDigest, Cwd: row.Cwd, StartedAt: row.StartedAt,
		DurationMs: row.DurationMs, ExitCode: row.ExitCode, ExitSignal: terminalSignalString(row.ExitSignal),
		ExitCause: string(row.ExitCause), DetectedBy: string(row.DetectedBy), Approval: string(row.Approval),
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
	Mode     TerminalAttachMode `json:"mode"`
	ClientID string             `json:"client_id,omitempty"`
}

type TerminalExecRequest struct {
	Command string                  `json:"command"`
	Args    []string                `json:"args,omitempty"`
	Cwd     string                  `json:"cwd,omitempty"`
	Env     map[string]string       `json:"env,omitempty"`
	YieldMs int                     `json:"yield_ms,omitzero"`
	Visible bool                    `json:"visible,omitzero"`
	Output  terminalpkg.OutputShape `json:"output,omitzero"`
}

type TerminalWaitRequest struct {
	Until     string `json:"until"`
	Pattern   string `json:"pattern,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
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
	Action TerminalRecordingAction `json:"action"`
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
	Outcome TerminalInputRejectOutcome `json:"outcome"`
}

type TerminalRecordingPayload struct {
	ID         string                 `json:"id"`
	State      TerminalRecordingState `json:"state"`
	TerminalID terminalpkg.ID         `json:"terminal_id"`
	ProfileID  string                 `json:"profile_id"`
	Digest     string                 `json:"digest"`
	StartedAt  time.Time              `json:"started_at"`
	StoppedAt  *time.Time             `json:"stopped_at,omitempty"`
	Bytes      int64                  `json:"bytes"`
	ExpiresAt  time.Time              `json:"expires_at"`
}

func TerminalRecordingPayloadFromDomain(
	recording terminalpkg.RecordingRef,
	state TerminalRecordingState,
) TerminalRecordingPayload {
	return TerminalRecordingPayload{
		ID: recording.ID, State: state, TerminalID: recording.TerminalID, ProfileID: recording.ProfileID,
		Digest: recording.Digest, StartedAt: recording.StartedAt, StoppedAt: recording.StoppedAt,
		Bytes: recording.Bytes, ExpiresAt: recording.ExpiresAt,
	}
}

type TerminalRecordingResponse struct {
	Recording TerminalRecordingPayload `json:"recording"`
}

type TerminalJournalResponse struct {
	Entries []TerminalCommandRowPayload `json:"entries"`
	Next    *string                     `json:"next"`
}

type TerminalCatalogSnapshot struct {
	Terminals []TerminalInfoPayload `json:"terminals"`
}

func terminalSignalFromString(signal *string) *terminalpkg.Signal {
	if signal == nil {
		return nil
	}
	return new(terminalpkg.Signal(*signal))
}

func terminalSignalString(signal *terminalpkg.Signal) *string {
	if signal == nil {
		return nil
	}
	return new(string(*signal))
}
