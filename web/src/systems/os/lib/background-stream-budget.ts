import type { OsDesktopRuntimeStore, OsWindow } from "./os-types";

export interface BackgroundStreamBudgetState {
  activeDesktopId: OsDesktopRuntimeStore["activeDesktopId"];
  connectionStatus: OsDesktopRuntimeStore["connectionStatus"];
  windows: Readonly<Record<string, Pick<OsWindow, "app" | "desktopId" | "shown">>>;
}

/**
 * Leaves browser connection capacity to authoritative and visible-window streams. A New tab
 * window holds no stream: it is what an empty desktop shows, so it reserves nothing.
 */
export function backgroundStreamsWithinConnectionBudget(
  state: BackgroundStreamBudgetState
): boolean {
  return (
    state.connectionStatus === "connected" &&
    !Object.values(state.windows).some(
      win => win.desktopId === state.activeDesktopId && win.shown && win.app !== "new-tab"
    )
  );
}

/** Continuity streams may coexist with windows, but yield during an active authority handshake. */
export function continuityStreamsWithinConnectionBudget(
  state: Pick<BackgroundStreamBudgetState, "connectionStatus">
): boolean {
  return state.connectionStatus !== "connecting" && state.connectionStatus !== "reconnecting";
}
