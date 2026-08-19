import type { OsDesktopRuntimeStore, WindowManagerController } from "./os-types";

/**
 * Everything a `client_op` needs to run in the attached client.
 *
 * The seam owns *which* operation runs; this object owns *what it can touch*.
 * Handlers read live state through `manager.getState()` rather than a captured
 * snapshot, because a command can be dispatched long after the surface that
 * offered it rendered.
 */
export interface PaletteShellHandlers {
  openPalette(): void;
  openPaletteView(viewId: string): void;
  openCheatsheet(): void;
  openDesktops(): void;
  openWorkspaces(): void;
  openNewSession(): void;
  toggleSessions(): void;
  toggleSidebar(): void;
  toggleGlobalScope(): void;
  cycleWorkspace(direction: "previous" | "next"): void;
  cycleSession(direction: "previous" | "next"): void;
  focusAttention(): void;
  /** Opens a fresh tab and hands the palette its destination intent. */
  openNewTab(stackTargetWindowId: string | null): void;
  /** Tab and window activation owns exactly one URL write (US-020). */
  activateWindow(windowId: string): void;
}

export interface PaletteClientOpContext {
  readonly manager: WindowManagerController;
  readonly shell: PaletteShellHandlers;
}

export type PaletteClientOpHandler = (
  context: PaletteClientOpContext,
  payload: unknown
) => void | Promise<unknown>;

export function currentState(context: PaletteClientOpContext): OsDesktopRuntimeStore {
  return context.manager.getState();
}
