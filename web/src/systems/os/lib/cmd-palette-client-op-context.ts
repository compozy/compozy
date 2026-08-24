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
  /** Switches the active profile through the canonical selection route. */
  useProfile(profile: string): void;
  cycleWorkspace(direction: "previous" | "next"): void;
  cycleSession(direction: "previous" | "next"): void;
  focusAttention(): void;
  /** Opens a fresh tab and hands the palette its destination intent. */
  openNewTab(stackTargetWindowId: string | null): void;
  /** Tab and window activation owns exactly one URL write (US-020). */
  activateWindow(windowId: string): void;
  /**
   * Raises the palette on a command's argument or confirmation step. A chord or
   * a menu item can reach a command that declares either, so the surface that
   * asks has to be reachable from outside the palette tree.
   */
  openPaletteExecution(): void;
}

export interface PaletteClientOpContext {
  readonly manager: WindowManagerController;
  readonly shell: PaletteShellHandlers;
  /** Same port local dispatch uses for `navigate` actions. */
  readonly navigate: (app: string, pathname: string | null) => void;
  /** Same port local dispatch uses for `url` actions. */
  readonly openUrl: (url: string) => void;
}

export type PaletteClientOpHandler = (
  context: PaletteClientOpContext,
  payload: unknown
) => void | Promise<unknown>;

export function currentState(context: PaletteClientOpContext): OsDesktopRuntimeStore {
  return context.manager.getState();
}
