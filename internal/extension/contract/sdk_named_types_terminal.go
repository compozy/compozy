package contract

import "github.com/compozy/compozy/internal/hooks"

func terminalNamedHookTypes() map[string]NamedType {
	return map[string]NamedType{
		"TerminalContext":       {Name: "TerminalContext", Value: hooks.TerminalContext{}},
		"TerminalExit":          {Name: "TerminalExit", Value: hooks.TerminalExit{}},
		"TerminalOpenedPayload": {Name: "TerminalOpenedPayload", Value: hooks.TerminalOpenedPayload{}},
		"TerminalClosedPayload": {Name: "TerminalClosedPayload", Value: hooks.TerminalClosedPayload{}},
		"TerminalLeaseChangedPayload": {
			Name:  "TerminalLeaseChangedPayload",
			Value: hooks.TerminalLeaseChangedPayload{},
		},
		"TerminalCommandStartedPayload": {
			Name:  "TerminalCommandStartedPayload",
			Value: hooks.TerminalCommandStartedPayload{},
		},
		"TerminalCommandFinishedPayload": {
			Name:  "TerminalCommandFinishedPayload",
			Value: hooks.TerminalCommandFinishedPayload{},
		},
		"TerminalInputRequestedPayload": {
			Name:  "TerminalInputRequestedPayload",
			Value: hooks.TerminalInputRequestedPayload{},
		},
		"TerminalInputProvidedPayload": {
			Name:  "TerminalInputProvidedPayload",
			Value: hooks.TerminalInputProvidedPayload{},
		},
		"TerminalRecordingStartedPayload": {
			Name:  "TerminalRecordingStartedPayload",
			Value: hooks.TerminalRecordingStartedPayload{},
		},
		"TerminalRecordingStoppedPayload": {
			Name:  "TerminalRecordingStoppedPayload",
			Value: hooks.TerminalRecordingStoppedPayload{},
		},
		"TerminalSubscriberEvictedPayload": {
			Name:  "TerminalSubscriberEvictedPayload",
			Value: hooks.TerminalSubscriberEvictedPayload{},
		},
		"TerminalLimitRejectedPayload": {
			Name:  "TerminalLimitRejectedPayload",
			Value: hooks.TerminalLimitRejectedPayload{},
		},
		"TerminalObservationPatch": {Name: "TerminalObservationPatch", Value: hooks.TerminalObservationPatch{}},
	}
}
