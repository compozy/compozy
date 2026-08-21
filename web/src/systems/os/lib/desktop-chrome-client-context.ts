import type { WorkspaceScopeMode } from "@/systems/workspace";

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
  scope: WorkspaceScopeMode;
  focusedSessionState: string | null | undefined;
  registeredWorkspaceTrusted: boolean | undefined;
  destinationRoute: OsWindowRoute | null;
  globalShortcuts: WindowManagerClientContextInput["globalShortcuts"];
}): WindowManagerClientContextInput {
  return {
    scopeGlobal: input.scope === "global",
    focusedSessionState: input.focusedSessionState ?? null,
    workspaceTrusted: input.registeredWorkspaceTrusted ?? false,
    destinationIntent: input.destinationRoute,
    globalShortcuts: input.globalShortcuts,
  };
}
