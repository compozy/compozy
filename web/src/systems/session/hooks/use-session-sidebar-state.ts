import { useEffect } from "react";
import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

import type { SessionListGroupBy, SessionListSort } from "../lib/session-list-preferences";
import {
  mergeSessionSidebarPreferences,
  migrateSessionSidebarPreferences,
  prunedSessionSidebarScopes,
  type SessionSidebarContext,
} from "../lib/session-sidebar-persistence";
import {
  DEFAULT_SCOPE_STATE,
  isRecord,
  sessionFoldSet,
  toggledFlag,
  toggledGroup,
  withoutFalseFlag,
  type SessionFoldAll,
  type SessionFoldSet,
  type SessionSidebarScopeState,
  type SessionWindowViewMode,
} from "../lib/session-sidebar-scope";
import type { SessionThreadSegment } from "../lib/session-thread-branch";
import { normalizeSessionBoardDrawerWidth } from "../lib/session-board-drawer-width";
import {
  normalizeSessionSidebarWidth,
  SESSION_SIDEBAR_DEFAULT_WIDTH,
} from "../lib/session-sidebar-width";

export type { SessionFoldAll, SessionFoldSet, SessionSidebarScopeState, SessionWindowViewMode };

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
    hideChildren: DEFAULT_SCOPE_STATE.hideChildren,
    collapsedBoardColumns: previousShell?.collapsedBoardColumns ?? {},
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
    hideChildrenChanged: (context, event: { scopeId: string; hideChildren: boolean }) => {
      // The shell entry seeds new windows, and this filter never seeds.
      if (event.scopeId === SHELL_WORKTREE_SCOPE) return;
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        hideChildren: event.hideChildren,
      }));
    },
    boardColumnCollapsedChanged: (
      context,
      event: { scopeId: string; columnId: string; collapsed: boolean }
    ) =>
      withScopeState(context, event.scopeId, state => ({
        ...state,
        collapsedBoardColumns: withoutFalseFlag(
          state.collapsedBoardColumns,
          event.columnId,
          event.collapsed
        ),
      })),
    sortChanged: (context, event: { scopeId: string; sort: SessionListSort }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, sort: event.sort })),
    groupByChanged: (context, event: { scopeId: string; groupBy: SessionListGroupBy }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, groupBy: event.groupBy })),
    showEmptyProjectsChanged: (context, event: { scopeId: string; show: boolean }) =>
      withScopeState(context, event.scopeId, state => ({
        ...state,
        showEmptyProjects: event.show,
      })),
    /** Every fold follows the choice again: older exceptions end here. Board columns are apart. */
    foldAllChanged: (context, event: { scopeId: string; foldAll: Exclude<SessionFoldAll, null> }) =>
      withScopeState(context, event.scopeId, state => ({
        ...state,
        foldAll: event.foldAll,
        collapsedThreads: {},
        subagentThreads: {},
        collapsedFolders: {},
        collapsedGroups: {},
      })),
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
      const scopeWidth = resolveSessionSidebarScope(context.byScope, event.scopeId).drawerWidth;
      // A drag back to this window's own width still makes it the width new windows start from.
      const shellWidth = context.byScope[SHELL_WORKTREE_SCOPE]?.drawerWidth ?? null;
      if (scopeWidth === drawerWidth && shellWidth === drawerWidth) return;
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
        collapsedFolders: toggledFlag(
          state.collapsedFolders,
          event.folderId,
          state.foldAll === "collapsed"
        ),
      }));
    },
    groupToggled: (context, event: { scopeId: string; groupId: string }) => {
      if (!event.groupId) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        collapsedGroups: toggledGroup(
          state.collapsedGroups,
          event.groupId,
          state.foldAll === "collapsed"
        ),
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

const SESSION_SIDEBAR_SCHEMA_VERSION = 3;

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
  /** Threads opened one by one, which stay open under Collapse all. */
  openThreadIds: string[];
  /** Parent sessions whose subagent list is open. */
  subagentThreadIds: string[];
  /** Shows one list under a session row, or folds its branch (`null`). */
  setThreadBranch: (sessionId: string, segment: SessionThreadSegment | null) => void;
  collapsedFolderIds: string[];
  /** Which folders are folded, `foldAll` included. */
  folderFolds: SessionFoldSet;
  toggleFolder: (folderId: string) => void;
  collapsedGroupIds: string[];
  /** Which groups are folded, `foldAll` included. */
  groupFolds: SessionFoldSet;
  /** A group's stored fold alone; `undefined` follows `foldAll`. */
  groupFlag: (groupId: string) => boolean | undefined;
  toggleGroup: (groupId: string) => void;
  /** The last Collapse all or Expand all in this window. */
  foldAll: SessionFoldAll;
  setFoldAll: (foldAll: Exclude<SessionFoldAll, null>) => void;
  /** Project grouping lists projects with no sessions too (#1085); new windows inherit it. */
  showEmptyProjects: boolean;
  setShowEmptyProjects: (show: boolean) => void;
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
    openThreadIds: Object.keys(state.collapsedThreads).filter(id => !state.collapsedThreads[id]),
    subagentThreadIds: flaggedIds(state.subagentThreads),
    setThreadBranch: (sessionId: string, segment: SessionThreadSegment | null) =>
      sessionSidebarStore.trigger.threadBranchChanged({ scopeId, sessionId, segment }),
    collapsedFolderIds: flaggedIds(state.collapsedFolders),
    folderFolds: sessionFoldSet(state.collapsedFolders, state.foldAll),
    toggleFolder: (folderId: string) =>
      sessionSidebarStore.trigger.folderToggled({ scopeId, folderId }),
    collapsedGroupIds: flaggedIds(state.collapsedGroups),
    groupFolds: sessionFoldSet(state.collapsedGroups, state.foldAll),
    groupFlag: groupId => state.collapsedGroups[groupId],
    toggleGroup: (groupId: string) =>
      sessionSidebarStore.trigger.groupToggled({ scopeId, groupId }),
    foldAll: state.foldAll,
    setFoldAll: foldAll => sessionSidebarStore.trigger.foldAllChanged({ scopeId, foldAll }),
    showEmptyProjects: state.showEmptyProjects,
    setShowEmptyProjects: (show: boolean) =>
      sessionSidebarStore.trigger.showEmptyProjectsChanged({ scopeId, show }),
    width: state.width ?? SESSION_SIDEBAR_DEFAULT_WIDTH,
    setWidth: (nextWidth: number) =>
      sessionSidebarStore.trigger.widthChanged({ scopeId, width: nextWidth }),
    resetWidth: () => sessionSidebarStore.trigger.widthReset({ scopeId }),
  };
}

function flaggedIds(flags: Record<string, boolean>): string[] {
  return Object.entries(flags).flatMap(([id, collapsed]) => (collapsed ? [id] : []));
}
