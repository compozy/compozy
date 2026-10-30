import {
  SESSION_LIST_GROUP_BYS,
  SESSION_LIST_SORTS,
  type SessionListGroupBy,
  type SessionListSort,
} from "./session-list-preferences";
import { normalizeSessionBoardDrawerWidth } from "./session-board-drawer-width";
import { normalizeSessionSidebarWidth } from "./session-sidebar-width";

/** The session window body: the list + transcript, or the sessions board. */
export type SessionWindowViewMode = "list" | "kanban";

/**
 * One window's sessions presentation. Per-window by design: two session
 * windows side by side open, close, collapse, resize, and switch views independently.
 */
export interface SessionSidebarScopeState {
  /** The rail is open until the operator closes it in this window. */
  open: boolean;
  /** Child sessions start unfolded; a fold is remembered per root session id. */
  collapsedThreads: Record<string, boolean>;
  /**
   * Subagent lists start closed; an open one is remembered per parent session
   * id. Opening it folds the children, so `collapsedThreads` alone still says
   * which child rows are on screen.
   */
  subagentThreads: Record<string, boolean>;
  /** Folders start expanded; a collapse is remembered per folder id. */
  collapsedFolders: Record<string, boolean>;
  /**
   * Group-by groups start expanded; a collapse is remembered per group id,
   * which the list qualifies with its scope, profile lens, and grouping.
   */
  collapsedGroups: Record<string, boolean>;
  /**
   * The rail width the operator dragged to in this window, in pixels. `null`
   * follows the default, so a reset tracks it.
   */
  width: number | null;
  /**
   * The board's session drawer width in this window (#1052). `null` follows the default; the
   * last width dragged anywhere seeds windows that never chose one.
   */
  drawerWidth: number | null;
  /** List | Kanban (US-011). Window-only: new windows always open in List. */
  view: SessionWindowViewMode;
  /** The board's "Hide child sessions" filter, remembered per window. */
  hideChildren: boolean;
  /**
   * Session order and grouping, shared by this window's List and Kanban. `null`
   * until the operator picks one here: the window then follows the daemon's
   * `shell.sessions` value, which only seeds windows and never couples them.
   */
  sort: SessionListSort | null;
  groupBy: SessionListGroupBy | null;
}

export const DEFAULT_SCOPE_STATE: SessionSidebarScopeState = {
  open: true,
  collapsedThreads: {},
  subagentThreads: {},
  collapsedFolders: {},
  collapsedGroups: {},
  width: null,
  drawerWidth: null,
  view: "list",
  hideChildren: false,
  sort: null,
  groupBy: null,
};

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function normalizedCollapsedFlags(value: unknown): Record<string, boolean> {
  if (!isRecord(value)) {
    return {};
  }
  return Object.fromEntries(
    Object.entries(value).filter(
      (entry): entry is [string, boolean] => typeof entry[1] === "boolean"
    )
  );
}

export function normalizedScopeState(value: unknown): SessionSidebarScopeState {
  if (!isRecord(value)) {
    return DEFAULT_SCOPE_STATE;
  }
  return {
    open: typeof value.open === "boolean" ? value.open : DEFAULT_SCOPE_STATE.open,
    collapsedThreads: normalizedCollapsedFlags(value.collapsedThreads),
    subagentThreads: normalizedCollapsedFlags(value.subagentThreads),
    collapsedFolders: normalizedCollapsedFlags(value.collapsedFolders),
    collapsedGroups: normalizedCollapsedFlags(value.collapsedGroups),
    width: normalizeSessionSidebarWidth(value.width),
    drawerWidth: normalizeSessionBoardDrawerWidth(value.drawerWidth),
    view: value.view === "kanban" ? "kanban" : "list",
    hideChildren: value.hideChildren === true,
    sort: oneOf(SESSION_LIST_SORTS, value.sort),
    groupBy: oneOf(SESSION_LIST_GROUP_BYS, value.groupBy),
  };
}

function oneOf<T extends string>(values: readonly T[], value: unknown): T | null {
  return values.find(candidate => candidate === value) ?? null;
}
