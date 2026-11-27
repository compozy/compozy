import { useEffect } from "react";
import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { onAccountScopeEvicted } from "@/lib/computer-scope";
import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

import type {
  SessionListComputerLayout,
  SessionListGroupBy,
  SessionListSort,
} from "../lib/session-list-preferences";
import { sessionListShelfStore, type SessionListShelf } from "../lib/session-list-shelf";
import {
  closedSessionSidebarScopes,
  mergeSessionSidebarPreferences,
  migrateSessionSidebarPreferences,
  prunedSessionSidebarScopes,
  type SessionSidebarContext,
} from "../lib/session-sidebar-persistence";
import {
  isRecord,
  normalizedComputerScope,
  sessionFoldSet,
  toggledFlag,
  toggledGroup,
  withoutFalseFlag,
  type SessionFoldAll,
  type SessionFoldSet,
  type SessionSidebarScopeState,
  type SessionWindowComputerScope,
  type SessionWindowViewMode,
} from "../lib/session-sidebar-scope";
import {
  clearRemovedComputerFromSidebarState,
  evictRemoteSessionSidebarState,
  resolveSessionSidebarScope,
  resolveSessionWindowOrder,
  scopeBaseState,
  withScopeState,
} from "../lib/session-sidebar-window-events";
import type { SessionThreadSegment } from "../lib/session-thread-branch";
import { normalizeSessionBoardDrawerWidth } from "../lib/session-board-drawer-width";
import {
  normalizeSessionSidebarWidth,
  SESSION_SIDEBAR_DEFAULT_WIDTH,
} from "../lib/session-sidebar-width";

export type {
  SessionFoldAll,
  SessionFoldSet,
  SessionSidebarScopeState,
  SessionWindowComputerScope,
  SessionWindowViewMode,
};
export { resolveSessionSidebarScope, resolveSessionWindowOrder };

export const SESSION_SIDEBAR_STORAGE_KEY = "compozy:session:sidebar:v1";

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
    showAutomationChanged: (context, event: { scopeId: string; show: boolean }) => {
      // Like the board's filter, showing automation runs never seeds new windows.
      if (event.scopeId === SHELL_WORKTREE_SCOPE) return;
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        showAutomation: event.show,
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
    computersChanged: (context, event: { scopeId: string; computers: SessionListComputerLayout }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, computers: event.computers })),
    computerScopeChanged: (
      context,
      event: { scopeId: string; computerScope: SessionWindowComputerScope }
    ) =>
      withScopeState(context, event.scopeId, state => ({
        ...state,
        computerScope: normalizedComputerScope(event.computerScope),
      })),
    remoteAccountEvicted: context => evictRemoteSessionSidebarState(context),
    remoteComputerRemoved: (context, event: { computerId: string }) =>
      clearRemovedComputerFromSidebarState(context, event.computerId),
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
    groupToggled: (
      context,
      event: { scopeId: string; groupId: string; defaultCollapsed?: boolean }
    ) => {
      if (!event.groupId) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        collapsedGroups:
          event.defaultCollapsed !== undefined
            ? toggledFlag(
                state.collapsedGroups,
                event.groupId,
                state.foldAll === "collapsed"
                  ? true
                  : state.foldAll === "groups" || state.foldAll === "expanded"
                    ? false
                    : event.defaultCollapsed
              )
            : toggledGroup(state.collapsedGroups, event.groupId, state.foldAll === "collapsed"),
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

onAccountScopeEvicted(() => {
  sessionSidebarStore.trigger.remoteAccountEvicted();
});

/**
 * Drops rail state for closed windows of one window-manager partition (the
 * workspace + profile whose snapshot lists `liveScopeIds`); other partitions'
 * windows and the shell entry stay.
 */
export function pruneSessionSidebarScopes(
  partition: string,
  liveScopeIds: readonly string[]
): void {
  const closed = closedSessionSidebarScopes(
    sessionSidebarStore.getSnapshot().context.partitionByScope,
    partition,
    liveScopeIds
  );
  sessionSidebarStore.trigger.scopesPruned({ partition, liveScopeIds });
  // A closed window's shelf goes with its rail entry.
  if (closed.length > 0) sessionListShelfStore.trigger.scopesReleased({ scopeIds: closed });
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
  toggleGroup: (groupId: string, defaultCollapsed?: boolean) => void;
  /** The last Collapse all, Expand groups or Expand all in this window. */
  foldAll: SessionFoldAll;
  setFoldAll: (foldAll: Exclude<SessionFoldAll, null>) => void;
  /** Project grouping lists projects with no sessions too (#1085); new windows inherit it. */
  showEmptyProjects: boolean;
  setShowEmptyProjects: (show: boolean) => void;
  /** "Show automation runs" (#1355); per window, never inherited. */
  showAutomation: boolean;
  setShowAutomation: (show: boolean) => void;
  /** The list's shelf (active, archive, or Later); per window, kept across selections, never persisted. */
  shelf: SessionListShelf;
  setShelf: (shelf: SessionListShelf) => void;
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
  const shelf = useSelector(
    sessionListShelfStore,
    snapshot => snapshot.context.byScope[scopeId] ?? "active"
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
    toggleGroup: (groupId: string, defaultCollapsed?: boolean) =>
      sessionSidebarStore.trigger.groupToggled({ scopeId, groupId, defaultCollapsed }),
    foldAll: state.foldAll,
    setFoldAll: foldAll => sessionSidebarStore.trigger.foldAllChanged({ scopeId, foldAll }),
    showEmptyProjects: state.showEmptyProjects,
    setShowEmptyProjects: (show: boolean) =>
      sessionSidebarStore.trigger.showEmptyProjectsChanged({ scopeId, show }),
    showAutomation: state.showAutomation,
    setShowAutomation: (show: boolean) =>
      sessionSidebarStore.trigger.showAutomationChanged({ scopeId, show }),
    shelf,
    setShelf: (nextShelf: SessionListShelf) =>
      sessionListShelfStore.trigger.shelfChanged({ scopeId, shelf: nextShelf }),
    width: state.width ?? SESSION_SIDEBAR_DEFAULT_WIDTH,
    setWidth: (nextWidth: number) =>
      sessionSidebarStore.trigger.widthChanged({ scopeId, width: nextWidth }),
    resetWidth: () => sessionSidebarStore.trigger.widthReset({ scopeId }),
  };
}

function flaggedIds(flags: Record<string, boolean>): string[] {
  return Object.entries(flags).flatMap(([id, collapsed]) => (collapsed ? [id] : []));
}
