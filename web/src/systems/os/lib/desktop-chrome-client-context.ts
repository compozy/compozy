import type { WorkspaceScopeMode } from "@/systems/workspace";

import type { WindowManagerClientContextInput } from "../hooks/use-window-manager-stream";
import type { OsWindowRoute } from "./os-types";

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
