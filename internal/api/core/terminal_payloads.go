package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type terminalControllerPayload = contract.TerminalControllerPayload
type terminalRunPayload = contract.TerminalRunPayload
type terminalExitPayload = contract.TerminalExitPayload
type terminalInfoPayload = contract.TerminalInfoPayload

func terminalInfoFromDomain(info terminalpkg.Info, profileName string) terminalInfoPayload {
	payload := terminalInfoPayload{
		ID: info.ID, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: profileName,
		Title: info.Title, Shell: info.Shell, Cwd: info.Cwd, Mode: info.Mode, State: info.State,
		Lease: info.Lease, Viewers: info.Viewers, Capabilities: info.Capabilities, CreatedAt: info.CreatedAt,
	}
	if info.Controller != nil {
		payload.Controller = &terminalControllerPayload{Kind: info.Controller.Kind, ID: info.Controller.ID}
	}
	if info.BoundRun != nil {
		payload.BoundRun = &terminalRunPayload{SessionID: info.BoundRun.SessionID, RunID: info.BoundRun.RunID}
	}
	if info.Exit != nil {
		payload.Exit = terminalExitFromDomain(info.Exit)
	}
	return payload
}

func terminalExitFromDomain(exit *terminalpkg.Exit) *terminalExitPayload {
	if exit == nil {
		return nil
	}
	return &terminalExitPayload{Cause: exit.Cause, Code: exit.Code, Signal: exit.Signal, At: exit.At}
}
