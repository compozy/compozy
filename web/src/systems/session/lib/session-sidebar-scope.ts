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
 * The window's last Collapse all or Expand all; `null` until the operator uses either. It is the
 * default of every fold, loaded or not, so it also reaches rows and groups read later. A fold
 * flag stored afterwards is an exception to it.
 */
export type SessionFoldAll = "collapsed" | "expanded" | null;

/** A fold answers for any id: flags alone cannot name what `foldAll` covers. */
export type SessionFoldSet = Pick<ReadonlySet<string>, "has">;

/**
 * Open subagent lists. After Expand all a row with no child sessions opens its subagents
 * (`whenChildless`); a row with both keeps its children, since one list shows at a time.
 */
export interface SessionSubagentFolds extends SessionFoldSet {
  whenChildless?: SessionFoldSet;
}

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
  /** The board's "Hide child sessions" filter: on until turned off, remembered per window. */
  hideChildren: boolean;
  /**
   * Board columns start expanded; a fold is remembered per column id and applies to every
   * swimlane of this window's board (#1075). Window-only, like the board's filter.
   */
  collapsedBoardColumns: Record<string, boolean>;
  /**
   * Session order and grouping, shared by this window's List and Kanban. `null`
   * until the operator picks one here: the window then follows the daemon's
   * `shell.sessions` value, which only seeds windows and never couples them.
   */
  sort: SessionListSort | null;
  groupBy: SessionListGroupBy | null;
  /**
   * Project grouping lists every registered project, even one with no sessions
   * in the current view (#1085). Off by default: only projects with sessions show.
   */
  showEmptyProjects: boolean;
  /** Collapse all / Expand all, shared by this window's List and Kanban. */
  foldAll: SessionFoldAll;
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
  hideChildren: true,
  collapsedBoardColumns: {},
  sort: null,
  groupBy: null,
  showEmptyProjects: false,
  foldAll: null,
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
    // Absent in blobs written before #1118: the default (on) holds. A stored `false` stays, even
    // though the old default wrote it for every window: it cannot be told from a real choice.
    hideChildren:
      typeof value.hideChildren === "boolean"
        ? value.hideChildren
        : DEFAULT_SCOPE_STATE.hideChildren,
    collapsedBoardColumns: normalizedCollapsedFlags(value.collapsedBoardColumns),
    sort: oneOf(SESSION_LIST_SORTS, value.sort),
    groupBy: oneOf(SESSION_LIST_GROUP_BYS, value.groupBy),
    // Absent in blobs written before #1085: the default (off) holds.
    showEmptyProjects: value.showEmptyProjects === true,
    // Absent in blobs written before #1132: every fold keeps its own flag.
    foldAll: value.foldAll === "collapsed" || value.foldAll === "expanded" ? value.foldAll : null,
  };
}

export function toggledFlag(
  flags: Record<string, boolean>,
  id: string,
  collapsedByDefault = false
): Record<string, boolean> {
  return { ...flags, [id]: !(flags[id] ?? collapsedByDefault) };
}

/** A stored flag wins; anything else follows the window's `foldAll`. */
export function sessionFoldSet(
  flags: Record<string, boolean>,
  foldAll: SessionFoldAll
): SessionFoldSet {
  return { has: id => flags[id] ?? foldAll === "collapsed" };
}

/** What a session row's branch reads: the stored folds under the window's `foldAll`. */
export function sessionThreadFolds(input: {
  collapsedThreadIds: readonly string[];
  openThreadIds: readonly string[];
  subagentThreadIds: readonly string[];
  foldAll: SessionFoldAll;
  /** A search shows what it matches: `foldAll` never hides a matching child. */
  filtering: boolean;
}): { collapsedThreads: SessionFoldSet; subagentThreads: SessionSubagentFolds } {
  const collapsed = new Set(input.collapsedThreadIds);
  const open = new Set(input.openThreadIds);
  const subagents = new Set(input.subagentThreadIds);
  const foldAll = input.filtering ? null : input.foldAll;
  return {
    collapsedThreads: {
      has: id => collapsed.has(id) || (foldAll === "collapsed" && !open.has(id)),
    },
    subagentThreads: {
      has: id => subagents.has(id),
      ...(foldAll === "expanded"
        ? { whenChildless: { has: (id: string) => !collapsed.has(id) && !open.has(id) } }
        : {}),
    },
  };
}

/** Subagent lists start closed, so only an open one is stored. */
export function withoutFalseFlag(
  flags: Record<string, boolean>,
  id: string,
  value: boolean
): Record<string, boolean> {
  if (value) return { ...flags, [id]: true };
  if (!Object.hasOwn(flags, id)) return flags;
  return Object.fromEntries(Object.entries(flags).filter(([key]) => key !== id));
}

/**
 * Group ids carry their whole lens, so every scope, profile, and grouping the
 * operator ever toggled would pile up if an expand kept a `false` flag. A group
 * back at its default just drops the entry: only exceptions to it are stored.
 */
export function toggledGroup(
  flags: Record<string, boolean>,
  id: string,
  collapsedByDefault = false
): Record<string, boolean> {
  const collapsed = !(flags[id] ?? collapsedByDefault);
  if (collapsed !== collapsedByDefault) return { ...flags, [id]: collapsed };
  return Object.fromEntries(Object.entries(flags).filter(([key]) => key !== id));
}

function oneOf<T extends string>(values: readonly T[], value: unknown): T | null {
  return values.find(candidate => candidate === value) ?? null;
}
