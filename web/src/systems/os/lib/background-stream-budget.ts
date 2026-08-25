import type { OsDesktopRuntimeStore, OsWindow } from "./os-types";

export interface BackgroundStreamBudgetState {
  activeDesktopId: OsDesktopRuntimeStore["activeDesktopId"];
  connectionStatus: OsDesktopRuntimeStore["connectionStatus"];
  windows: Readonly<
    Record<string, Pick<OsWindow, "app" | "desktopId" | "minimized" | "stackActive">>
  >;
}

/** Leaves browser connection capacity to authoritative and visible-window streams. */
export function backgroundStreamsWithinConnectionBudget(
  state: BackgroundStreamBudgetState
): boolean {
  return (
    state.connectionStatus === "connected" &&
    !Object.values(state.windows).some(
      win => win.desktopId === state.activeDesktopId && !win.minimized && win.stackActive
    )
  );
}

/** Continuity streams may coexist with windows, but yield during an active authority handshake. */
export function continuityStreamsWithinConnectionBudget(
  state: Pick<BackgroundStreamBudgetState, "connectionStatus">
): boolean {
  return state.connectionStatus === "connected" || state.connectionStatus === "disconnected";
}
