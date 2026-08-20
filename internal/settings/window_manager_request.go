package settings

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
)

func hasWindowManagerMutation(req SectionUpdateRequest) bool {
	return req.WindowManager != nil ||
		req.WindowManagerShortcuts != nil ||
		req.WindowManagerGlobalShortcuts != nil ||
		req.WindowManagerAliases != nil
}

func mergeWindowManagerRequest(
	current compozyconfig.WindowManagerConfig,
	aliases map[string]string,
	req SectionUpdateRequest,
) (compozyconfig.WindowManagerConfig, map[string]string) {
	desired := cloneWindowManagerConfig(current)
	if req.WindowManager != nil {
		desired = cloneWindowManagerConfig(*req.WindowManager)
	}
	if req.WindowManagerShortcuts != nil {
		desired.Shortcuts = windowmanager.CloneShortcutMap(*req.WindowManagerShortcuts)
	}
	if req.WindowManagerGlobalShortcuts != nil {
		desired.GlobalShortcuts = windowmanager.CloneGlobalShortcutMap(
			*req.WindowManagerGlobalShortcuts,
		)
	}
	desiredAliases := cloneAliases(aliases)
	if req.WindowManagerAliases != nil {
		desiredAliases = cloneAliases(*req.WindowManagerAliases)
	}
	return desired, desiredAliases
}
