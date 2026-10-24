import type { BreadthMode } from "@/systems/workspace";

import type { WindowManagerClientContextInput } from "../hooks/use-window-manager-stream";
import type { OsDesktopRuntimeStore, OsWindowRoute } from "./os-types";

export function selectPaletteDestinationRoute(
  state: OsDesktopRuntimeStore,
  paletteWindowId: string | undefined
): OsWindowRoute | null {
  const windowId = paletteWindowId ?? state.focusedId;
  if (windowId === null) return null;
  return state.windows[windowId]?.route ?? null;
}

export function resolveLivePaletteClientContext(input: {
  breadthMode: BreadthMode;
  focusedSessionState: string | null | undefined;
  registeredWorkspaceTrusted: boolean | undefined;
  destinationRoute: OsWindowRoute | null;
  globalShortcuts: WindowManagerClientContextInput["globalShortcuts"];
}): WindowManagerClientContextInput {
  return {
    // The daemon's `scope.global` predicate names the globe: All projects on.
    scopeGlobal: input.breadthMode === "all",
    focusedSessionState: input.focusedSessionState ?? null,
    workspaceTrusted: input.registeredWorkspaceTrusted ?? false,
    destinationIntent: input.destinationRoute,
    globalShortcuts: input.globalShortcuts,
  };
}
