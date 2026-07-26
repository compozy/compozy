package contract

import "github.com/compozy/agh/internal/hooks"

func windowManagerNamedHookTypes() map[string]NamedType {
	return map[string]NamedType{
		"WindowManagerLayoutAppliedPayload": {
			Name:  "WindowManagerLayoutAppliedPayload",
			Value: hooks.WindowManagerLayoutAppliedPayload{},
		},
		"WindowManagerDesktopCreatedPayload": {
			Name:  "WindowManagerDesktopCreatedPayload",
			Value: hooks.WindowManagerDesktopCreatedPayload{},
		},
		"WindowManagerDesktopDeletedPayload": {
			Name:  "WindowManagerDesktopDeletedPayload",
			Value: hooks.WindowManagerDesktopDeletedPayload{},
		},
		"WindowManagerWindowMovedPayload": {
			Name:  "WindowManagerWindowMovedPayload",
			Value: hooks.WindowManagerWindowMovedPayload{},
		},
		"WindowManagerObservationPatch": {
			Name:  "WindowManagerObservationPatch",
			Value: hooks.WindowManagerObservationPatch{},
		},
	}
}
