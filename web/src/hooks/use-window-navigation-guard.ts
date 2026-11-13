import { createContext } from "react";

/** A router location as a navigation guard reads it. */
export interface GuardedLocation {
  pathname: string;
  search: Record<string, unknown>;
}

/**
 * What a navigation guard inside a window asks the shell, which owns window placement. Domain
 * systems read it here rather than from the `os` system, which depends on them.
 */
export interface WindowNavigationGuardPort {
  /**
   * Whether a router navigation to `next` takes window `windowId`'s content away: the shell
   * would land it in that window with a route other than the one the window shows. A navigation
   * that lands in another window, or brings the window back to its own route, does not. A window
   * the shell does not know answers `true`.
   */
  replacesWindowContent(windowId: string, next: GuardedLocation): boolean;
  /** Whether the window is on screen for this client now. */
  isWindowShown(windowId: string): boolean;
}

/** Provided by the desktop shell; `null` outside it, where a guard holds every navigation. */
export const WindowNavigationGuardContext = createContext<WindowNavigationGuardPort | null>(null);
