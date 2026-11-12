import type { BreadthMode } from "@/systems/workspace";

import type { GlobalShortcutRegistrationWire } from "./desktop-shell-bridge";
import type { OsDesktopRuntimeStore, OsWindowRoute } from "./os-types";

export interface WindowManagerClientContextInput {
  scopeGlobal: boolean;
  focusedSessionState: string | null;
  workspaceTrusted: boolean;
  destinationIntent: {
    pathname: string;
    search: Readonly<Record<string, unknown>>;
  } | null;
  globalShortcuts: readonly GlobalShortcutRegistrationWire[];
}

/** The `client_context` frame the window-manager stream sends for this client. */
export function serializeWindowManagerClientContext(
  context: WindowManagerClientContextInput
): string {
  return JSON.stringify({
    type: "client_context",
    context: {
      scope_global: context.scopeGlobal,
      ...(context.focusedSessionState === null
        ? {}
        : { focused_session_state: context.focusedSessionState }),
      workspace_trusted: context.workspaceTrusted,
      ...(context.destinationIntent === null
        ? {}
        : { destination_intent: context.destinationIntent }),
      global_shortcuts: context.globalShortcuts,
    },
  });
}

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
