package contract

import "github.com/compozy/compozy/internal/hooks"

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
		"WindowManagerWindowOpenedPayload": {
			Name:  "WindowManagerWindowOpenedPayload",
			Value: hooks.WindowManagerWindowOpenedPayload{},
		},
		"WindowManagerWindowClosedPayload": {
			Name:  "WindowManagerWindowClosedPayload",
			Value: hooks.WindowManagerWindowClosedPayload{},
		},
		"WindowManagerStackGroupedPayload": {
			Name:  "WindowManagerStackGroupedPayload",
			Value: hooks.WindowManagerStackGroupedPayload{},
		},
		"WindowManagerStackUngroupedPayload": {
			Name:  "WindowManagerStackUngroupedPayload",
			Value: hooks.WindowManagerStackUngroupedPayload{},
		},
		"WindowManagerStackActivatedPayload": {
			Name:  "WindowManagerStackActivatedPayload",
			Value: hooks.WindowManagerStackActivatedPayload{},
		},
		"WindowManagerObservationPatch": {
			Name:  "WindowManagerObservationPatch",
			Value: hooks.WindowManagerObservationPatch{},
		},
	}
}
