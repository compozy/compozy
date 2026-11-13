import type { ReadonlyAtom } from "@xstate/store";
import { createContext } from "react";

/** A router location as a navigation guard reads it. */
export interface GuardedLocation {
  pathname: string;
  search: Record<string, unknown>;
  /** The router's id for the matched route, when the guard knows it. */
  routeId?: string;
}

/** Unsaved edits an editor holds in a window, as an operator close of that window sees them. */
export interface UnsavedWindowEdits {
  /** Names the edits in the close question when they are the only ones it covers. */
  readonly entityName: string;
  /** Drops the edits: the operator chose to discard them with the window. */
  discard(): void;
}

/** An operator close the shell holds until the operator answers for the unsaved edits it covers. */
export interface UnsavedCloseQuestion {
  /** The edits that show the question: the first held among the close targets. */
  presenter: UnsavedWindowEdits;
  /** How many of the close targets hold unsaved edits. */
  windowCount: number;
  /** `true` discards every covered edit and lets the close go on; `false` cancels it. */
  answer(discard: boolean): void;
}

/** How editors take part in the shell's operator close admission. */
export interface UnsavedWindowEditsPort {
  /**
   * Marks window `windowId` as holding unsaved edits until the returned release runs: an operator
   * close that covers the window asks first.
   */
  hold(windowId: string, edits: UnsavedWindowEdits): () => void;
  /** The operator close being asked about, or `null`. */
  readonly question: ReadonlyAtom<UnsavedCloseQuestion | null>;
}

/**
 * What a navigation guard inside a window asks the shell, which owns window placement. Domain
 * systems read it here rather than from the `os` system, which depends on them.
 */
export interface WindowNavigationGuardPort {
  /**
   * Whether a router navigation to `next` takes window `windowId`'s content away: the shell
   * would land it in that window with content other than what the window shows, or leave the
   * shell so every window goes. A navigation that lands in another window, or keeps what the
   * window shows, does not. A window the shell does not know answers `true`.
   */
  replacesWindowContent(windowId: string, next: GuardedLocation): boolean;
  /** Whether the window is on screen for this client now. */
  isWindowShown(windowId: string): boolean;
  /** Stable for the shell's life, unlike the port object around it. */
  readonly unsavedEdits: UnsavedWindowEditsPort;
}

/** Provided by the desktop shell; `null` outside it, where a guard holds every navigation. */
export const WindowNavigationGuardContext = createContext<WindowNavigationGuardPort | null>(null);
