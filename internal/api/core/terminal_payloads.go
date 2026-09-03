package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

const (
	terminalPayloadKey   = "terminal"
	terminalIDPayloadKey = "terminal_id"
	terminalModeWrite    = "write"
)

type terminalExitPayload = contract.TerminalExitPayload
type terminalInfoPayload = contract.TerminalInfoPayload

func terminalInfoFromDomain(info terminalpkg.Info, profileName string) terminalInfoPayload {
	return contract.TerminalInfoPayloadFromDomain(info, profileName)
}

func terminalExitFromDomain(exit *terminalpkg.Exit) *terminalExitPayload {
	if exit == nil {
		return nil
	}
	var signal *contract.TerminalSignal
	if exit.Signal != nil {
		signal = new(contract.TerminalSignal(*exit.Signal))
	}
	return &terminalExitPayload{
		Cause: contract.TerminalExitCause(exit.Cause), Code: exit.Code, Signal: signal, At: exit.At,
	}
}
