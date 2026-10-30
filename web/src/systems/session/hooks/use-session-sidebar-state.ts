import { useEffect } from "react";
import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

import type { SessionListGroupBy, SessionListSort } from "../lib/session-list-preferences";
import {
  DEFAULT_SCOPE_STATE,
  isRecord,
  normalizedCollapsedFlags,
  normalizedScopeState,
  type SessionSidebarScopeState,
  type SessionWindowViewMode,
} from "../lib/session-sidebar-scope";
import type { SessionThreadSegment } from "../lib/session-thread-branch";
import { normalizeSessionBoardDrawerWidth } from "../lib/session-board-drawer-width";
import {
  normalizeSessionSidebarWidth,
  SESSION_SIDEBAR_DEFAULT_WIDTH,
} from "../lib/session-sidebar-width";

export type { SessionSidebarScopeState, SessionWindowViewMode };

interface SessionSidebarContext {
  /**
   * Keyed by window scope (the window id, or `shell` outside a window). The
   * shell entry is the last-used state: every gesture also writes it, and a
   * window with no entry of its own starts from it.
   */
  byScope: Record<string, SessionSidebarScopeState>;
  /**
   * The window-manager partition (workspace + profile) whose snapshot listed
   * each window scope. A snapshot only lists its own partition's windows, so a
   * prune may drop only entries it owns: switching workspace or profile must
   * not erase the other partition's windows, which are still saved. An entry
   * no snapshot has listed yet is never pruned.
   */
  partitionByScope: Record<string, string>;
}

export const SESSION_SIDEBAR_STORAGE_KEY = "compozy:session:sidebar:v1";

/**
 * A scope with no entry of its own follows the shell (last-used) state, so a
 * new window opens looking like the last one the operator touched. Its order
 * and grouping are the exception: read them through `resolveSessionWindowOrder`.
 */
export function resolveSessionSidebarScope(
  byScope: Record<string, SessionSidebarScopeState>,
  scopeId: string
): SessionSidebarScopeState {
  return byScope[scopeId] ?? byScope[SHELL_WORKTREE_SCOPE] ?? DEFAULT_SCOPE_STATE;
}

/**
 * A window's own order and grouping. A window never inherits them from the
 * shell (last-used) state: until it chooses, both read `null` and the daemon's
 * value seeds it, so one window's choice never reaches a new window.
 */
export function resolveSessionWindowOrder(
  byScope: Record<string, SessionSidebarScopeState>,
  scopeId: string
): Pick<SessionSidebarScopeState, "sort" | "groupBy"> {
  return byScope[scopeId] ?? NO_WINDOW_ORDER;
}

const NO_WINDOW_ORDER = { sort: null, groupBy: null } as const;

/** The state a gesture starts from: inherited rail state, never an inherited order. */
function scopeBaseState(
  byScope: Record<string, SessionSidebarScopeState>,
  scopeId: string
): SessionSidebarScopeState {
  const state = resolveSessionSidebarScope(byScope, scopeId);
  return Object.hasOwn(byScope, scopeId) || scopeId === SHELL_WORKTREE_SCOPE
    ? state
    : { ...state, ...NO_WINDOW_ORDER };
}

function mergeSessionSidebarPreferences(
  persisted: Partial<SessionSidebarContext>,
  current: SessionSidebarContext
): SessionSidebarContext {
  if (!isRecord(persisted.byScope)) {
    return current;
  }
  const byScope = { ...current.byScope };
  for (const [scopeId, state] of Object.entries(persisted.byScope)) {
    byScope[scopeId] = normalizedScopeState(state);
  }
  const partitionByScope = { ...current.partitionByScope };
  if (isRecord(persisted.partitionByScope)) {
    for (const [scopeId, partition] of Object.entries(persisted.partitionByScope)) {
      if (typeof partition === "string" && scopeId !== SHELL_WORKTREE_SCOPE) {
        partitionByScope[scopeId] = partition;
      }
    }
  }
  return { byScope, partitionByScope };
}

/**
 * Writes one scope and makes it the last-used state new windows inherit. The
 * board view, its filter, and the order and grouping stay with their window: a
 * window opened to show a session must never land on another window's board.
 */
function withScopeState(
  context: SessionSidebarContext,
  scopeId: string,
  update: (state: SessionSidebarScopeState) => SessionSidebarScopeState
): SessionSidebarContext {
  const next = update(scopeBaseState(context.byScope, scopeId));
  const previousShell = context.byScope[SHELL_WORKTREE_SCOPE];
  const shell = {
    ...next,
    view: DEFAULT_SCOPE_STATE.view,
    hideChildren: false,
    sort: previousShell?.sort ?? null,
    groupBy: previousShell?.groupBy ?? null,
    // Only a drag sets the drawer width new windows start from (`drawerWidthChanged`).
    drawerWidth: previousShell?.drawerWidth ?? null,
  };
  return {
    ...context,
    byScope: { ...context.byScope, [SHELL_WORKTREE_SCOPE]: shell, [scopeId]: next },
  };
}

function toggledFlag(flags: Record<string, boolean>, id: string): Record<string, boolean> {
  return { ...flags, [id]: !(flags[id] ?? false) };
}

/** Subagent lists start closed, so only an open one is stored. */
function withoutFalseFlag(
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
 * operator ever toggled would pile up if an expand kept a `false` flag. Groups
 * start expanded, so an expand just drops the entry: only folds are stored.
 */
function toggledGroup(flags: Record<string, boolean>, id: string): Record<string, boolean> {
  if (!flags[id]) return { ...flags, [id]: true };
  return Object.fromEntries(Object.entries(flags).filter(([key]) => key !== id));
}

/**
 * Live scopes are claimed for the snapshot's partition; a closed scope is
 * dropped only when that same partition claimed it. `undefined` = no change.
 */
function prunedSessionSidebarScopes(
  context: SessionSidebarContext,
  partition: string,
  liveScopeIds: readonly string[]
): SessionSidebarContext | undefined {
  const live = new Set(liveScopeIds);
  const byScope = { ...context.byScope };
  const partitionByScope = { ...context.partitionByScope };
  let changed = false;
  // Claim before seeding: a window can appear in the snapshot a commit
  // before its rail mounts and seeds an entry.
  for (const scopeId of live) {
    if (partitionByScope[scopeId] !== partition) {
      partitionByScope[scopeId] = partition;
      changed = true;
    }
  }
  for (const [scopeId, owner] of Object.entries(partitionByScope)) {
    if (owner === partition && !live.has(scopeId)) {
      delete partitionByScope[scopeId];
      delete byScope[scopeId];
      changed = true;
    }
  }
  return changed ? { byScope, partitionByScope } : undefined;
}

export const sessionSidebarLogic = createStoreLogic({
  context: (): SessionSidebarContext => ({ byScope: {}, partitionByScope: {} }),
  on: {
    sidebarToggled: (context, event: { scopeId: string }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, open: !state.open })),
    sidebarVisibilityChanged: (context, event: { scopeId: string; open: boolean }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, open: event.open })),
    threadBranchChanged: (
      context,
      event: { scopeId: string; sessionId: string; segment: SessionThreadSegment | null }
    ) => {
      if (!event.sessionId) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        collapsedThreads: {
          ...state.collapsedThreads,
          [event.sessionId]: event.segment !== "children",
        },
        subagentThreads: withoutFalseFlag(
          state.subagentThreads,
          event.sessionId,
          event.segment === "subagents"
        ),
      }));
    },
    viewChanged: (context, event: { scopeId: string; view: SessionWindowViewMode }) => {
      if (resolveSessionSidebarScope(context.byScope, event.scopeId).view === event.view) return;
      return withScopeState(context, event.scopeId, state => ({ ...state, view: event.view }));
    },
    hideChildrenChanged: (context, event: { scopeId: string; hideChildren: boolean }) =>
      withScopeState(context, event.scopeId, state => ({
        ...state,
        hideChildren: event.hideChildren,
      })),
    sortChanged: (context, event: { scopeId: string; sort: SessionListSort }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, sort: event.sort })),
    groupByChanged: (context, event: { scopeId: string; groupBy: SessionListGroupBy }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, groupBy: event.groupBy })),
    widthChanged: (context, event: { scopeId: string; width: number }) => {
      const width = normalizeSessionSidebarWidth(event.width);
      if (width === null) {
        return;
      }
      if (resolveSessionSidebarScope(context.byScope, event.scopeId).width === width) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({ ...state, width }));
    },
    drawerWidthChanged: (context, event: { scopeId: string; width: number }) => {
      const drawerWidth = normalizeSessionBoardDrawerWidth(event.width);
      if (drawerWidth === null) return;
      if (resolveSessionSidebarScope(context.byScope, event.scopeId).drawerWidth === drawerWidth) {
        return;
      }
      const next = withScopeState(context, event.scopeId, state => ({ ...state, drawerWidth }));
      // The width a new window starts from is the one last dragged, in any window.
      const shell = next.byScope[SHELL_WORKTREE_SCOPE];
      return {
        ...next,
        byScope: { ...next.byScope, [SHELL_WORKTREE_SCOPE]: { ...shell, drawerWidth } },
      };
    },
    widthReset: (context, event: { scopeId: string }) => {
      if (resolveSessionSidebarScope(context.byScope, event.scopeId).width === null) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({ ...state, width: null }));
    },
    folderToggled: (context, event: { scopeId: string; folderId: string }) => {
      if (!event.folderId) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        collapsedFolders: toggledFlag(state.collapsedFolders, event.folderId),
      }));
    },
    groupToggled: (context, event: { scopeId: string; groupId: string }) => {
      if (!event.groupId) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        collapsedGroups: toggledGroup(state.collapsedGroups, event.groupId),
      }));
    },
    groupLensResolved: (context, event: { from: string; to: string }) => {
      if (event.from === event.to) return;
      const prefix = `${event.from}\u0000`;
      const byScope = { ...context.byScope };
      let changed = false;
      // Resolve each window independently, including the default new windows inherit.
      for (const [scopeId, state] of Object.entries(byScope)) {
        const entries = Object.entries(state.collapsedGroups).filter(([id]) =>
          id.startsWith(prefix)
        );
        if (entries.length === 0) continue;
        const collapsedGroups = { ...state.collapsedGroups };
        for (const [id, collapsed] of entries) {
          const resolved = `${event.to}\u0000${id.slice(prefix.length)}`;
          collapsedGroups[resolved] ??= collapsed;
          delete collapsedGroups[id];
        }
        byScope[scopeId] = { ...state, collapsedGroups };
        changed = true;
      }
      return changed ? { ...context, byScope } : undefined;
    },
    /**
     * Pins a window to the state it opened with. Without its own entry a window
     * would keep following the shell, i.e. every other window's gestures.
     */
    scopeSeeded: (context, event: { scopeId: string }) => {
      if (Object.hasOwn(context.byScope, event.scopeId)) {
        return;
      }
      return {
        ...context,
        byScope: {
          ...context.byScope,
          [event.scopeId]: scopeBaseState(context.byScope, event.scopeId),
        },
      };
    },
    /**
     * Windows close from any client, so no single owner can release a scope
     * deterministically — the shell prunes against the live window set instead.
     */
    scopesPruned: (context, event: { partition: string; liveScopeIds: readonly string[] }) =>
      prunedSessionSidebarScopes(context, event.partition, event.liveScopeIds),
  },
});

/**
 * Schema 3 keys the rail by window scope. Schema 2 was one global rail for
 * every window: it becomes the shell (last-used) entry, so every existing and
 * new window starts exactly where the operator left the old global rail.
 * Older blobs (schema 0, unversioned) persisted the whole context on every
 * transition — collapsing a thread wrote the old closed default back as
 * `open: false` — so their `open` is not a choice the operator made: drop it
 * and keep the per-thread collapse.
 */
const SESSION_SIDEBAR_SCHEMA_VERSION = 3;

// Runs only for a blob older than the current schema (the persist extension
// skips it on a version match); the merge normalizes every scope entry.
function migrateSessionSidebarPreferences(
  persisted: unknown,
  version: string | number
): SessionSidebarContext {
  const legacy = isRecord(persisted) ? persisted : {};
  const shell =
    version === 2
      ? legacy
      : { collapsedThreads: normalizedCollapsedFlags(legacy.collapsedThreads) };
  return {
    byScope: { [SHELL_WORKTREE_SCOPE]: shell as unknown as SessionSidebarScopeState },
    partitionByScope: {},
  };
}

export const sessionSidebarStore = sessionSidebarLogic.createStore().with(
  persist({
    name: SESSION_SIDEBAR_STORAGE_KEY,
    version: SESSION_SIDEBAR_SCHEMA_VERSION,
    // Window cleanup is derived from the daemon, not an operator gesture.
    // Persisting it would let a background document overwrite another tab's choice.
    filter: event => event.type !== "scopesPruned",
    migrate: migrateSessionSidebarPreferences,
    merge: mergeSessionSidebarPreferences,
  })
);

/**
 * Drops rail state for closed windows of one window-manager partition (the
 * workspace + profile whose snapshot lists `liveScopeIds`); other partitions'
 * windows and the shell entry stay.
 */
export function pruneSessionSidebarScopes(
  partition: string,
  liveScopeIds: readonly string[]
): void {
  sessionSidebarStore.trigger.scopesPruned({ partition, liveScopeIds });
  persistSessionSidebarPrune(partition, liveScopeIds);
}

/**
 * The store's persist filter skips `scopesPruned` so this document's possibly
 * stale rail choices never overwrite another tab's. The claims and cleanup
 * still have to survive a reload — otherwise a window closed before the next
 * gesture keeps its entry forever — so apply the same prune to the stored
 * blob alone, leaving every stored choice as it is.
 */
function persistSessionSidebarPrune(partition: string, liveScopeIds: readonly string[]): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    const raw = window.localStorage.getItem(SESSION_SIDEBAR_STORAGE_KEY);
    const stored: unknown = raw === null ? null : JSON.parse(raw);
    if (
      !isRecord(stored) ||
      stored.version !== SESSION_SIDEBAR_SCHEMA_VERSION ||
      !isRecord(stored.context)
    ) {
      // Nothing stored yet, or an older schema the next gesture migrates.
      return;
    }
    const persisted = mergeSessionSidebarPreferences(stored.context, {
      byScope: {},
      partitionByScope: {},
    });
    const next = prunedSessionSidebarScopes(persisted, partition, liveScopeIds);
    if (next) {
      window.localStorage.setItem(
        SESSION_SIDEBAR_STORAGE_KEY,
        JSON.stringify({ ...stored, context: next })
      );
    }
  } catch {
    // Storage unavailable or corrupt: the next gesture rewrites the blob.
  }
}

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === SESSION_SIDEBAR_STORAGE_KEY) {
      void rehydrateStore(sessionSidebarStore);
    }
  });
}

export interface UseSessionSidebarStateResult {
  open: boolean;
  toggle: () => void;
  setOpen: (open: boolean) => void;
  collapsedThreadIds: string[];
  /** Parent sessions whose subagent list is open. */
  subagentThreadIds: string[];
  /** Shows one list under a session row, or folds its branch (`null`). */
  setThreadBranch: (sessionId: string, segment: SessionThreadSegment | null) => void;
  collapsedFolderIds: string[];
  toggleFolder: (folderId: string) => void;
  collapsedGroupIds: string[];
  toggleGroup: (groupId: string) => void;
  /** Rail width in pixels: the operator's choice in this window, else the default. */
  width: number;
  setWidth: (width: number) => void;
  resetWidth: () => void;
}

/**
 * Sessions-rail state for the window scope rendering this hook: the window
 * itself inside an OS window, the focused window in shell chrome (so ⌘B, the
 * palette toggle, and session cycling act on what the operator is looking at),
 * and the shell scope outside the desktop.
 */
export function useSessionSidebarState(): UseSessionSidebarStateResult {
  const scopeId = useWorktreeScopeId();
  const seeded = useSelector(sessionSidebarStore, snapshot =>
    Object.hasOwn(snapshot.context.byScope, scopeId)
  );
  const state = useSelector(sessionSidebarStore, snapshot =>
    resolveSessionSidebarScope(snapshot.context.byScope, scopeId)
  );

  useEffect(() => {
    if (!seeded && scopeId !== SHELL_WORKTREE_SCOPE) {
      sessionSidebarStore.trigger.scopeSeeded({ scopeId });
    }
  }, [scopeId, seeded]);

  return {
    open: state.open,
    toggle: () => sessionSidebarStore.trigger.sidebarToggled({ scopeId }),
    setOpen: (nextOpen: boolean) =>
      sessionSidebarStore.trigger.sidebarVisibilityChanged({ scopeId, open: nextOpen }),
    collapsedThreadIds: flaggedIds(state.collapsedThreads),
    subagentThreadIds: flaggedIds(state.subagentThreads),
    setThreadBranch: (sessionId: string, segment: SessionThreadSegment | null) =>
      sessionSidebarStore.trigger.threadBranchChanged({ scopeId, sessionId, segment }),
    collapsedFolderIds: flaggedIds(state.collapsedFolders),
    toggleFolder: (folderId: string) =>
      sessionSidebarStore.trigger.folderToggled({ scopeId, folderId }),
    collapsedGroupIds: flaggedIds(state.collapsedGroups),
    toggleGroup: (groupId: string) =>
      sessionSidebarStore.trigger.groupToggled({ scopeId, groupId }),
    width: state.width ?? SESSION_SIDEBAR_DEFAULT_WIDTH,
    setWidth: (nextWidth: number) =>
      sessionSidebarStore.trigger.widthChanged({ scopeId, width: nextWidth }),
    resetWidth: () => sessionSidebarStore.trigger.widthReset({ scopeId }),
  };
}

function flaggedIds(flags: Record<string, boolean>): string[] {
  return Object.entries(flags).flatMap(([id, collapsed]) => (collapsed ? [id] : []));
}
