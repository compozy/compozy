package contract

import (
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type TerminalID string
type TerminalActorKind string
type TerminalMode string
type TerminalSignal string
type TerminalLeaseState string

type TerminalCapabilitiesPayload struct {
	Interactive bool `json:"interactive"`
}

type TerminalOutputShape struct {
	MaxBytes int    `json:"max_bytes,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Grep     string `json:"grep,omitempty"`
}

type TerminalSpillPayload struct {
	ArtifactID string `json:"artifact_id"`
	Bytes      int64  `json:"bytes"`
}

type TerminalControllerPayload struct {
	Kind TerminalActorKind `json:"kind"`
	ID   string            `json:"id"`
}

type TerminalRunPayload struct {
	SessionID  string `json:"session_id"`
	RunID      string `json:"run_id"`
	Generation int64  `json:"generation"`
}

type TerminalExitPayload struct {
	Cause  TerminalExitCause `json:"cause"`
	Code   *int              `json:"code,omitempty"`
	Signal *TerminalSignal   `json:"signal,omitempty"`
	At     time.Time         `json:"at"`
}

type TerminalInfoPayload struct {
	ID           TerminalID                  `json:"id"`
	WorkspaceID  string                      `json:"workspace_id"`
	ProfileID    string                      `json:"profile_id"`
	ProfileName  string                      `json:"profile_name"`
	Title        string                      `json:"title"`
	Shell        string                      `json:"shell"`
	Cwd          string                      `json:"cwd"`
	Mode         TerminalMode                `json:"mode"`
	State        TerminalState               `json:"state"`
	Controller   *TerminalControllerPayload  `json:"controller"`
	Lease        TerminalLeaseState          `json:"lease"`
	Viewers      int                         `json:"viewers"`
	BoundRun     *TerminalRunPayload         `json:"bound_run"`
	Capabilities TerminalCapabilitiesPayload `json:"capabilities"`
	CreatedAt    time.Time                   `json:"created_at"`
	Exit         *TerminalExitPayload        `json:"exit,omitempty"`
}

func TerminalInfoPayloadFromDomain(info terminalpkg.Info, profileName string) TerminalInfoPayload {
	payload := TerminalInfoPayload{
		ID:          TerminalID(info.ID),
		WorkspaceID: info.WS,
		ProfileID:   info.ProfileID,
		ProfileName: profileName,
		Title:       info.Title,
		Shell:       info.Shell,
		Cwd:         info.Cwd,
		Mode:        TerminalMode(info.Mode),
		State:       TerminalState(info.State),
		Lease:       TerminalLeaseState(info.Lease),
		Viewers:     info.Viewers,
		Capabilities: TerminalCapabilitiesPayload{
			Interactive: info.Capabilities.Interactive,
		},
		CreatedAt: info.CreatedAt,
	}
	if info.Controller != nil {
		payload.Controller = &TerminalControllerPayload{
			Kind: TerminalActorKind(info.Controller.Kind),
			ID:   info.Controller.ID,
		}
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
	Kind TerminalActorKind `json:"kind"`
	ID   string            `json:"id"`
}

type TerminalCommandRowPayload struct {
	ID          string                      `json:"command_id"`
	TerminalID  *TerminalID                 `json:"terminal_id"`
	ProfileID   string                      `json:"profile_id"`
	ProfileName string                      `json:"profile_name"`
	Actor       TerminalCommandActorPayload `json:"actor"`
	Command     string                      `json:"command"`
	ArgvDigest  *string                     `json:"argv_digest,omitempty"`
	Cwd         string                      `json:"cwd"`
	StartedAt   time.Time                   `json:"started_at"`
	DurationMs  *int64                      `json:"duration_ms"`
	ExitCode    *int                        `json:"exit_code"`
	ExitSignal  *TerminalSignal             `json:"signal"`
	ExitCause   TerminalExitCause           `json:"exit_cause"`
	DetectedBy  TerminalCommandDetection    `json:"detected_by"`
	Approval    TerminalCommandApproval     `json:"approval"`
	OutputBytes int64                       `json:"output_bytes"`
	Truncated   bool                        `json:"truncated"`
	RecordingID *string                     `json:"recording,omitempty"`
}

func TerminalCommandRowPayloadFromDomain(row terminalpkg.CommandRow, profileName string) TerminalCommandRowPayload {
	return TerminalCommandRowPayload{
		ID:          row.ID,
		TerminalID:  terminalIDFromDomain(row.TerminalID),
		ProfileID:   row.ProfileID,
		ProfileName: profileName,
		Actor:       TerminalCommandActorPayload{Kind: TerminalActorKind(row.Actor.Kind), ID: row.Actor.ID},
		Command:     row.Command,
		ArgvDigest:  row.ArgvDigest,
		Cwd:         row.Cwd,
		StartedAt:   row.StartedAt,
		DurationMs:  row.DurationMs,
		ExitCode:    row.ExitCode,
		ExitSignal:  terminalSignalFromString(row.ExitSignal),
		ExitCause:   TerminalExitCause(row.ExitCause),
		DetectedBy:  TerminalCommandDetection(row.DetectedBy),
		Approval:    TerminalCommandApproval(row.Approval),
		OutputBytes: row.OutputBytes,
		Truncated:   row.Truncated,
		RecordingID: row.RecordingID,
	}
}

func TerminalCommandRowFromPayload(row TerminalCommandRowPayload) terminalpkg.CommandRow {
	return terminalpkg.CommandRow{
		ID:          row.ID,
		TerminalID:  terminalIDToDomain(row.TerminalID),
		ProfileID:   row.ProfileID,
		ProfileName: row.ProfileName,
		Actor: terminalpkg.Actor{
			Kind:      terminalpkg.ActorKind(row.Actor.Kind),
			ID:        row.Actor.ID,
			ProfileID: row.ProfileID,
		},
		Command:     row.Command,
		ArgvDigest:  row.ArgvDigest,
		Cwd:         row.Cwd,
		StartedAt:   row.StartedAt,
		DurationMs:  row.DurationMs,
		ExitCode:    row.ExitCode,
		ExitSignal:  terminalSignalString(row.ExitSignal),
		ExitCause:   string(row.ExitCause),
		DetectedBy:  string(row.DetectedBy),
		Approval:    string(row.Approval),
		OutputBytes: row.OutputBytes,
		Truncated:   row.Truncated,
		RecordingID: row.RecordingID,
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
	Signal TerminalSignal `json:"signal"`
}

type TerminalAttachTicketRequest struct {
	Mode     TerminalAttachMode `json:"mode"`
	ClientID string             `json:"client_id,omitempty"`
}

type TerminalExecRequest struct {
	Command string              `json:"command"`
	Args    []string            `json:"args,omitempty"`
	Cwd     string              `json:"cwd,omitempty"`
	Env     map[string]string   `json:"env,omitempty"`
	YieldMs int                 `json:"yield_ms,omitzero"`
	Visible bool                `json:"visible,omitzero"`
	Output  TerminalOutputShape `json:"output,omitzero"`
}

type TerminalExecResponse struct {
	ExitCode     *int                  `json:"exit_code"`
	Signal       *TerminalSignal       `json:"signal"`
	Output       string                `json:"output"`
	Truncated    bool                  `json:"truncated"`
	Untrusted    bool                  `json:"untrusted"`
	Spill        *TerminalSpillPayload `json:"spill,omitempty"`
	DurationMs   int64                 `json:"duration_ms"`
	CommandID    string                `json:"command_id"`
	StillRunning bool                  `json:"still_running,omitempty"`
	TerminalID   *TerminalID           `json:"terminal_id"`
}

func TerminalExecResponseFromDomain(result terminalpkg.ExecResult) TerminalExecResponse {
	return TerminalExecResponse{
		ExitCode: result.ExitCode, Signal: terminalSignalFromString(result.Signal), Output: result.Output,
		Truncated: result.Truncated, Untrusted: result.Untrusted, Spill: terminalSpillFromDomain(result.Spill),
		DurationMs: result.DurationMs, CommandID: result.CommandID, StillRunning: result.StillRunning,
		TerminalID: terminalIDFromDomain(result.TerminalID),
	}
}

type TerminalWaitRequest struct {
	Until     string `json:"until"`
	Pattern   string `json:"pattern,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

type TerminalWaitResponse struct {
	Reason    string `json:"reason"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Screen    string `json:"screen"`
	Untrusted bool   `json:"untrusted"`
}

func TerminalWaitResponseFromDomain(result terminalpkg.WaitResult) TerminalWaitResponse {
	return TerminalWaitResponse{
		Reason: result.Reason, ExitCode: result.ExitCode, Screen: result.Screen, Untrusted: result.Untrusted,
	}
}

type TerminalSignalRequest struct {
	Signal TerminalSignal `json:"signal"`
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

type TerminalReadResponse struct {
	Content   string                `json:"content"`
	Seq       TerminalSequence      `json:"seq"`
	Truncated bool                  `json:"truncated"`
	Busy      bool                  `json:"busy"`
	Untrusted bool                  `json:"untrusted"`
	Spill     *TerminalSpillPayload `json:"spill,omitempty"`
}

func TerminalReadResponseFromDomain(result terminalpkg.ReadResult) TerminalReadResponse {
	return TerminalReadResponse{
		Content: result.Content, Seq: TerminalSequenceFromUint64(result.Seq), Truncated: result.Truncated,
		Busy: result.Busy, Untrusted: result.Untrusted, Spill: terminalSpillFromDomain(result.Spill),
	}
}

type TerminalInputRequestsResponse struct {
	Pending  []TerminalPendingInputRequest  `json:"pending"`
	Resolved []TerminalResolvedInputRequest `json:"resolved"`
}

type TerminalInputActorPayload struct {
	Kind TerminalActorKind `json:"kind"`
	ID   string            `json:"id"`
}

type TerminalPendingInputRequest struct {
	ID            string                    `json:"id"`
	TerminalID    TerminalID                `json:"terminal_id"`
	WorkspaceID   string                    `json:"workspace_id,omitempty"`
	ProfileID     string                    `json:"profile_id"`
	ProfileName   string                    `json:"profile_name"`
	Reason        string                    `json:"reason"`
	PromptExcerpt string                    `json:"prompt_excerpt"`
	Redacted      bool                      `json:"redacted"`
	RequestedAt   time.Time                 `json:"requested_at"`
	Requester     TerminalInputActorPayload `json:"requester"`
}

type TerminalResolvedInputRequest struct {
	ID          string                         `json:"id"`
	TerminalID  TerminalID                     `json:"terminal_id"`
	WorkspaceID string                         `json:"workspace_id,omitempty"`
	ProfileID   string                         `json:"profile_id"`
	ProfileName string                         `json:"profile_name"`
	Requester   TerminalInputActorPayload      `json:"requester"`
	Outcome     TerminalInputResolutionOutcome `json:"outcome"`
	ResolvedBy  TerminalInputActorPayload      `json:"resolved_by"`
	Reason      string                         `json:"reason,omitempty"`
	Redacted    bool                           `json:"redacted"`
	Length      int                            `json:"length"`
	RequestedAt time.Time                      `json:"requested_at"`
	ResolvedAt  time.Time                      `json:"resolved_at"`
}

func TerminalInputRequestsResponseFromDomain(
	pending []terminalpkg.PendingInputRequest,
	resolved []terminalpkg.ResolvedInputRequest,
) (TerminalInputRequestsResponse, error) {
	response := TerminalInputRequestsResponse{
		Pending:  make([]TerminalPendingInputRequest, 0, len(pending)),
		Resolved: make([]TerminalResolvedInputRequest, 0, len(resolved)),
	}
	for _, request := range pending {
		response.Pending = append(response.Pending, TerminalPendingInputRequest{
			ID: string(request.ID), TerminalID: TerminalID(request.TerminalID), WorkspaceID: request.WorkspaceID,
			ProfileID: request.ProfileID, ProfileName: request.ProfileName, Reason: request.Reason,
			PromptExcerpt: request.PromptExcerpt, Redacted: request.Redacted, RequestedAt: request.RequestedAt,
			Requester: terminalInputActorFromDomain(request.Requester),
		})
	}
	for _, request := range resolved {
		outcome, err := terminalInputResolutionOutcomeFromDomain(request.Outcome)
		if err != nil {
			return TerminalInputRequestsResponse{}, err
		}
		response.Resolved = append(response.Resolved, TerminalResolvedInputRequest{
			ID: string(request.ID), TerminalID: TerminalID(request.TerminalID), WorkspaceID: request.WorkspaceID,
			ProfileID: request.ProfileID, ProfileName: request.ProfileName,
			Requester: terminalInputActorFromDomain(request.Requester), Outcome: outcome,
			ResolvedBy: terminalInputActorFromDomain(request.ResolvedBy), Reason: request.Reason,
			Redacted: request.Redacted, Length: request.Length, RequestedAt: request.RequestedAt,
			ResolvedAt: request.ResolvedAt,
		})
	}
	return response, nil
}

func terminalInputResolutionOutcomeFromDomain(
	outcome terminalpkg.InputResolutionOutcome,
) (TerminalInputResolutionOutcome, error) {
	if _, err := terminalpkg.ParseInputResolutionOutcome(string(outcome)); err != nil {
		return "", err
	}
	return TerminalInputResolutionOutcome(outcome), nil
}

func terminalInputActorFromDomain(actor terminalpkg.InputActorProjection) TerminalInputActorPayload {
	return TerminalInputActorPayload{Kind: TerminalActorKind(actor.Kind), ID: actor.ID}
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
	TerminalID TerminalID             `json:"terminal_id"`
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
		ID: recording.ID, State: state, TerminalID: TerminalID(recording.TerminalID), ProfileID: recording.ProfileID,
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

func terminalSignalFromString(signal *string) *TerminalSignal {
	if signal == nil {
		return nil
	}
	return new(TerminalSignal(*signal))
}

func terminalSignalString(signal *TerminalSignal) *string {
	if signal == nil {
		return nil
	}
	return new(string(*signal))
}

func terminalIDFromDomain(id *terminalpkg.ID) *TerminalID {
	if id == nil {
		return nil
	}
	return new(TerminalID(*id))
}

func terminalIDToDomain(id *TerminalID) *terminalpkg.ID {
	if id == nil {
		return nil
	}
	return new(terminalpkg.ID(*id))
}

func terminalSpillFromDomain(spill *terminalpkg.SpillRef) *TerminalSpillPayload {
	if spill == nil {
		return nil
	}
	return &TerminalSpillPayload{ArtifactID: spill.ArtifactID, Bytes: spill.Bytes}
}
