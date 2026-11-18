import { useEffect, useEffectEvent } from "react";
import { shallowEqual } from "@xstate/store";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsShell } from "../../hooks/use-os-shell";
import type { OsWindowRoute } from "../../lib/os-types";
import { settingsRouteWithMovedShortcutCommand } from "@/systems/settings";

const DEFAULT_SETTINGS_ROUTE: OsWindowRoute = { pathname: "/settings", search: {} };

/**
 * The route a settings window shows, from its WM location (not router matches) so an unfocused
 * settings window keeps its own section, and whether its search shortcut is live.
 *
 * A window still stored at `/settings/layouts?command=<id>` (persisted, agent-opened, or an old
 * deep link) shows Shortcuts on this render, and its stored route is replaced so history and
 * reloads read the new section. The stored route is an external system, so syncing it is one of
 * the narrow cases for an effect.
 */
export function useSettingsWindowRoute(windowId: string): {
  route: OsWindowRoute;
  searchEnabled: boolean;
} {
  const { manager } = useOsShell();
  const { stored, searchEnabled } = useDesktop(state => {
    const win = state.windows[windowId];
    return {
      stored: win?.route ?? DEFAULT_SETTINGS_ROUTE,
      searchEnabled:
        !!win &&
        state.focusedId === windowId &&
        win.desktopId === state.activeDesktopId &&
        win.shown,
    };
  }, shallowEqual);
  const moved = settingsRouteWithMovedShortcutCommand(stored);
  const replace = useEffectEvent(() => {
    if (moved !== null) manager.navigateWindow(windowId, moved, "replace");
  });
  const movedCommand = moved === null ? null : String(moved.search.command);

  useEffect(() => {
    if (movedCommand !== null) replace();
  }, [movedCommand, windowId]);

  return { route: moved ?? stored, searchEnabled };
}
