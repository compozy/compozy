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
	return contract.TerminalInfoPayloadFromDomain(info, profileName)
}

func terminalExitFromDomain(exit *terminalpkg.Exit) *terminalExitPayload {
	if exit == nil {
		return nil
	}
	return &terminalExitPayload{Cause: exit.Cause, Code: exit.Code, Signal: exit.Signal, At: exit.At}
}
