import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

import { parseSessionRowKey } from "./session-row-ref";
import type { SessionSidebarContext } from "./session-sidebar-persistence";
import {
  DEFAULT_SCOPE_STATE,
  normalizedComputerScope,
  withoutRemoteFoldFlags,
  type SessionSidebarScopeState,
  type SessionWindowComputerScope,
} from "./session-sidebar-scope";

/**
 * A scope with no entry of its own follows the shell (last-used) state, so a
 * new window opens looking like the last one the operator touched. Its order,
 * grouping, computers layer and computer scope are the exception: read them
 * through `resolveSessionWindowOrder`.
 */
export function resolveSessionSidebarScope(
  byScope: Record<string, SessionSidebarScopeState>,
  scopeId: string
): SessionSidebarScopeState {
  return byScope[scopeId] ?? byScope[SHELL_WORKTREE_SCOPE] ?? DEFAULT_SCOPE_STATE;
}

/**
 * A window's own order, grouping, computers layer and computer scope. A window
 * never inherits them from the shell (last-used) state: until it chooses, order
 * and layer read `null` (daemon seed) and scope reads `"all"` (Q6).
 */
export function resolveSessionWindowOrder(
  byScope: Record<string, SessionSidebarScopeState>,
  scopeId: string
): Pick<SessionSidebarScopeState, "sort" | "groupBy" | "computers" | "computerScope"> {
  return byScope[scopeId] ?? NO_WINDOW_ORDER;
}

export const NO_WINDOW_ORDER = {
  sort: null,
  groupBy: null,
  computers: null,
  computerScope: "all" as SessionWindowComputerScope,
} as const;

/** The state a gesture starts from: inherited rail state, never an inherited order or scope. */
export function scopeBaseState(
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
 * board view, its filter, the order, grouping, computers layer and computer
 * scope stay with their window: a new window starts at All computers (Q6) and
 * the daemon's seed layer.
 */
export function withScopeState(
  context: SessionSidebarContext,
  scopeId: string,
  update: (state: SessionSidebarScopeState) => SessionSidebarScopeState
): SessionSidebarContext {
  const next = update(scopeBaseState(context.byScope, scopeId));
  const previousShell = context.byScope[SHELL_WORKTREE_SCOPE];
  const shell: SessionSidebarScopeState = {
    ...next,
    view: DEFAULT_SCOPE_STATE.view,
    hideChildren: DEFAULT_SCOPE_STATE.hideChildren,
    showAutomation: DEFAULT_SCOPE_STATE.showAutomation,
    collapsedBoardColumns: previousShell?.collapsedBoardColumns ?? {},
    sort: previousShell?.sort ?? null,
    groupBy: previousShell?.groupBy ?? null,
    computers: previousShell?.computers ?? null,
    computerScope: DEFAULT_SCOPE_STATE.computerScope,
    // Only a drag sets the drawer width new windows start from (`drawerWidthChanged`).
    drawerWidth: previousShell?.drawerWidth ?? null,
  };
  return {
    ...context,
    byScope: { ...context.byScope, [SHELL_WORKTREE_SCOPE]: shell, [scopeId]: next },
  };
}

/**
 * Account sign-out or switch (C3, Rule 23): resets every window's computer scope
 * to `"all"` and removes every remote computer's fold flags. Local folds stay.
 */
function withoutComputerThreadFlags(flags: Record<string, boolean>, target?: ComputerId) {
  const entries = Object.entries(flags);
  const kept = entries.filter(([key]) => {
    const { computerId } = parseSessionRowKey(key);
    return target ? computerId !== target : !isRemoteComputerId(computerId);
  });
  return kept.length === entries.length ? flags : Object.fromEntries(kept);
}

export function evictRemoteSessionSidebarState(
  context: SessionSidebarContext
): SessionSidebarContext | undefined {
  let changed = false;
  const byScope: Record<string, SessionSidebarScopeState> = {};
  for (const [scopeId, state] of Object.entries(context.byScope)) {
    const nextComputerScope = isRemoteComputerId(state.computerScope)
      ? DEFAULT_SCOPE_STATE.computerScope
      : state.computerScope;
    const nextGroups = withoutRemoteFoldFlags(state.collapsedGroups);
    const nextFolders = withoutRemoteFoldFlags(state.collapsedFolders);
    const nextThreads = withoutComputerThreadFlags(state.collapsedThreads);
    const nextSubagents = withoutComputerThreadFlags(state.subagentThreads);
    if (
      nextComputerScope !== state.computerScope ||
      nextGroups !== state.collapsedGroups ||
      nextFolders !== state.collapsedFolders ||
      nextThreads !== state.collapsedThreads ||
      nextSubagents !== state.subagentThreads
    ) {
      changed = true;
      byScope[scopeId] = {
        ...state,
        computerScope: nextComputerScope,
        collapsedGroups: nextGroups,
        collapsedFolders: nextFolders,
        collapsedThreads: nextThreads,
        subagentThreads: nextSubagents,
      };
    } else {
      byScope[scopeId] = state;
    }
  }
  return changed ? { ...context, byScope } : undefined;
}

/**
 * Removed or disabled computer (C2, C4): resets any window scoped to that
 * computer back to `"all"` and drops its fold flags.
 */
export function clearRemovedComputerFromSidebarState(
  context: SessionSidebarContext,
  computerId: ComputerId
): SessionSidebarContext | undefined {
  const target = normalizedComputerScope(computerId);
  if (target === "all" || target === "local") return undefined;
  let changed = false;
  const byScope: Record<string, SessionSidebarScopeState> = {};
  for (const [scopeId, state] of Object.entries(context.byScope)) {
    const nextScope =
      state.computerScope === target ? DEFAULT_SCOPE_STATE.computerScope : state.computerScope;
    const nextGroups = withoutRemoteFoldFlags(state.collapsedGroups, target);
    const nextFolders = withoutRemoteFoldFlags(state.collapsedFolders, target);
    const nextThreads = withoutComputerThreadFlags(state.collapsedThreads, target);
    const nextSubagents = withoutComputerThreadFlags(state.subagentThreads, target);
    if (
      nextScope !== state.computerScope ||
      nextGroups !== state.collapsedGroups ||
      nextFolders !== state.collapsedFolders ||
      nextThreads !== state.collapsedThreads ||
      nextSubagents !== state.subagentThreads
    ) {
      changed = true;
      byScope[scopeId] = {
        ...state,
        computerScope: nextScope,
        collapsedGroups: nextGroups,
        collapsedFolders: nextFolders,
        collapsedThreads: nextThreads,
        subagentThreads: nextSubagents,
      };
    } else {
      byScope[scopeId] = state;
    }
  }
  return changed ? { ...context, byScope } : undefined;
}
