package cli

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func terminalExecRequestContract(request TerminalExecRequest) contract.TerminalExecRequest {
	return contract.TerminalExecRequest{
		Command: request.Command,
		Args:    slices.Clone(request.Args),
		Cwd:     request.Cwd,
		Env:     maps.Clone(request.Env),
		YieldMs: request.YieldMs,
		Visible: request.Visible,
		Output: contract.TerminalOutputShape{
			MaxBytes: request.Output.MaxBytes,
			Strategy: request.Output.Strategy,
			Grep:     request.Output.Grep,
		},
	}
}

func terminalExecResultFromContract(response contract.TerminalExecResponse) (terminalpkg.ExecResult, error) {
	if strings.TrimSpace(response.CommandID) == "" {
		return terminalpkg.ExecResult{}, fmt.Errorf("terminal exec response is missing command_id")
	}
	signal, err := terminalSignalFromContract(response.Signal)
	if err != nil {
		return terminalpkg.ExecResult{}, err
	}
	var terminalID *terminalpkg.ID
	if response.TerminalID != nil {
		terminalID, err = terminalIDFromContract(response.TerminalID)
		if err != nil {
			return terminalpkg.ExecResult{}, err
		}
	} else if response.StillRunning {
		return terminalpkg.ExecResult{}, errors.New("terminal response is missing terminal_id for a running command")
	}
	spill, err := terminalSpillFromContractStrict(response.Spill)
	if err != nil {
		return terminalpkg.ExecResult{}, err
	}
	return terminalpkg.ExecResult{
		ExitCode: response.ExitCode, Signal: signal, Output: response.Output,
		Truncated: response.Truncated, Untrusted: response.Untrusted, Spill: spill,
		DurationMs: response.DurationMs, CommandID: response.CommandID,
		StillRunning: response.StillRunning, TerminalID: terminalID,
	}, nil
}

func terminalSignalFromContract(signal *contract.TerminalSignal) (*string, error) {
	if signal == nil {
		return nil, nil
	}
	if !slices.Contains(contract.TerminalSignalValues(), string(*signal)) {
		return nil, fmt.Errorf("terminal response contains unsupported signal %q", *signal)
	}
	value := string(*signal)
	return &value, nil
}

func terminalIDFromContract(id *contract.TerminalID) (*terminalpkg.ID, error) {
	if id == nil {
		return nil, fmt.Errorf("terminal response is missing terminal_id")
	}
	value := strings.TrimSpace(string(*id))
	if value == "" {
		return nil, fmt.Errorf("terminal response contains an empty terminal_id")
	}
	result := terminalpkg.ID(value)
	return &result, nil
}

func terminalSpillFromContractStrict(spill *contract.TerminalSpillPayload) (*terminalpkg.SpillRef, error) {
	if spill == nil {
		return nil, nil
	}
	if strings.TrimSpace(spill.ArtifactID) == "" {
		return nil, fmt.Errorf("terminal response contains an empty spill artifact_id")
	}
	if spill.Bytes < 0 {
		return nil, fmt.Errorf("terminal response contains a negative spill byte count")
	}
	return &terminalpkg.SpillRef{ArtifactID: spill.ArtifactID, Bytes: spill.Bytes}, nil
}

func terminalInputRequestsFromContract(response contract.TerminalInputRequestsResponse) (TerminalInputRequests, error) {
	if response.Pending == nil || response.Resolved == nil {
		return TerminalInputRequests{}, fmt.Errorf("terminal input response is missing pending or resolved")
	}
	pending := make([]terminalpkg.PendingInputRequest, 0, len(response.Pending))
	for _, item := range response.Pending {
		requester, err := terminalInputActorFromContract(item.Requester)
		if err != nil {
			return TerminalInputRequests{}, err
		}
		if !terminalInputIdentityComplete(item.ID, item.TerminalID, item.ProfileID) {
			return TerminalInputRequests{}, fmt.Errorf(
				"terminal pending input response contains an incomplete identity",
			)
		}
		pending = append(pending, terminalpkg.PendingInputRequest{
			ID: terminalpkg.InputRequestID(item.ID), TerminalID: terminalpkg.ID(item.TerminalID),
			WorkspaceID: item.WorkspaceID, ProfileID: item.ProfileID, ProfileName: item.ProfileName,
			Reason: item.Reason, PromptExcerpt: item.PromptExcerpt, Redacted: item.Redacted,
			RequestedAt: item.RequestedAt, Requester: requester,
		})
	}
	resolved := make([]terminalpkg.ResolvedInputRequest, 0, len(response.Resolved))
	for _, item := range response.Resolved {
		requester, err := terminalInputActorFromContract(item.Requester)
		if err != nil {
			return TerminalInputRequests{}, err
		}
		resolvedBy, err := terminalInputActorFromContract(item.ResolvedBy)
		if err != nil {
			return TerminalInputRequests{}, err
		}
		outcome, err := terminalpkg.ParseInputResolutionOutcome(string(item.Outcome))
		if err != nil {
			return TerminalInputRequests{}, fmt.Errorf("decode terminal resolved input outcome: %w", err)
		}
		if !terminalInputIdentityComplete(item.ID, item.TerminalID, item.ProfileID) {
			return TerminalInputRequests{}, fmt.Errorf(
				"terminal resolved input response contains an incomplete identity",
			)
		}
		resolved = append(resolved, terminalpkg.ResolvedInputRequest{
			ID: terminalpkg.InputRequestID(item.ID), TerminalID: terminalpkg.ID(item.TerminalID),
			WorkspaceID: item.WorkspaceID, ProfileID: item.ProfileID, ProfileName: item.ProfileName,
			Requester: requester, Outcome: outcome, ResolvedBy: resolvedBy, Reason: item.Reason,
			Redacted: item.Redacted, Length: item.Length, RequestedAt: item.RequestedAt,
			ResolvedAt: item.ResolvedAt,
		})
	}
	return TerminalInputRequests{Pending: pending, Resolved: resolved}, nil
}

func terminalInputIdentityComplete(id string, terminalID contract.TerminalID, profileID string) bool {
	return strings.TrimSpace(id) != "" && strings.TrimSpace(string(terminalID)) != "" &&
		strings.TrimSpace(profileID) != ""
}

func terminalInputActorFromContract(
	actor contract.TerminalInputActorPayload,
) (terminalpkg.InputActorProjection, error) {
	if !slices.Contains(contract.TerminalActorKindValues(), string(actor.Kind)) {
		return terminalpkg.InputActorProjection{}, fmt.Errorf(
			"terminal input response contains unsupported actor kind %q",
			actor.Kind,
		)
	}
	if strings.TrimSpace(actor.ID) == "" {
		return terminalpkg.InputActorProjection{}, fmt.Errorf("terminal input response contains an empty actor id")
	}
	return terminalpkg.InputActorProjection{Kind: terminalpkg.ActorKind(actor.Kind), ID: actor.ID}, nil
}

func terminalRecordingRequest(action string) (contract.TerminalRecordingRequest, error) {
	if !slices.Contains(contract.TerminalRecordingActionValues(), action) {
		return contract.TerminalRecordingRequest{}, fmt.Errorf("unsupported terminal recording action %q", action)
	}
	return contract.TerminalRecordingRequest{Action: contract.TerminalRecordingAction(action)}, nil
}

func terminalRecordingFromContract(payload contract.TerminalRecordingPayload) (terminalpkg.RecordingRef, error) {
	if strings.TrimSpace(payload.ID) == "" || strings.TrimSpace(string(payload.TerminalID)) == "" ||
		strings.TrimSpace(payload.ProfileID) == "" {
		return terminalpkg.RecordingRef{}, fmt.Errorf("terminal recording response contains an incomplete identity")
	}
	if !slices.Contains(contract.TerminalRecordingStateValues(), string(payload.State)) {
		return terminalpkg.RecordingRef{}, fmt.Errorf(
			"terminal recording response contains unsupported state %q",
			payload.State,
		)
	}
	if payload.Bytes < 0 {
		return terminalpkg.RecordingRef{}, fmt.Errorf("terminal recording response contains a negative byte count")
	}
	return terminalpkg.RecordingRef{
		ID: payload.ID, State: string(payload.State), TerminalID: terminalpkg.ID(payload.TerminalID),
		ProfileID: payload.ProfileID, Digest: payload.Digest, StartedAt: payload.StartedAt,
		StoppedAt: payload.StoppedAt, Bytes: payload.Bytes, ExpiresAt: payload.ExpiresAt,
	}, nil
}
