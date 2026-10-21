import { createStore } from "@xstate/store";
import { persist } from "@xstate/store/persist";

import { USER_FOCUS, type BreadthMode, type Focus } from "../lib/destination";
import {
  ACTIVE_WORKSPACE_SCHEMA_VERSION,
  migrateActiveWorkspace,
  type PersistedActiveWorkspace,
} from "./active-workspace-migration";
import { persistWorktreePrune, prunedWorktreeScopes } from "./active-workspace-worktree-pruning";

/**
 * Scope id for surfaces outside a retained OS window (shell chrome, standalone
 * routes, Storybook). Real windows key by their own daemon-issued window id.
 */
export const SHELL_WORKTREE_SCOPE = "shell";

export interface ActiveWorkspaceContext {
  /** The globe: All projects, or only the focus. */
  breadthMode: BreadthMode;
  /** Last focus — a project or User. Turning All projects off returns to it. */
  focus: Focus;
  /**
   * Remembered project: the last project focused. Equals the focus while a
   * project is focused and survives User focus and All projects.
   */
  selectedWorkspaceId: string | null;
  /**
   * Per-scope worktree selection: each OS window carries its own worktree
   * without a second window-manager binding. Window ids are unique across
   * partitions, so a project's windows keep their picks while another
   * destination is focused.
   */
  worktreeByScope: Record<string, string | null>;
  /**
   * The window-manager partition (workspace + profile) whose snapshot listed
   * each window scope. A snapshot only lists its own partition's windows, so
   * a prune drops only entries that partition claimed; an entry no snapshot
   * has claimed yet is never pruned.
   */
  worktreePartitionByScope: Record<string, string>;
}

export const ACTIVE_WORKSPACE_PERSIST_KEY = "compozy:active-workspace:v4";

const initialActiveWorkspaceContext: ActiveWorkspaceContext = {
  breadthMode: "all",
  focus: USER_FOCUS,
  selectedWorkspaceId: null,
  worktreeByScope: {},
  worktreePartitionByScope: {},
};

function withScope(
  context: ActiveWorkspaceContext,
  scopeId: string,
  worktreeId: string | null
): ActiveWorkspaceContext {
  return { ...context, worktreeByScope: { ...context.worktreeByScope, [scopeId]: worktreeId } };
}

/** The shell's ambient pick names a worktree of the remembered project; it cannot follow another. */
function withoutShellScope(byScope: Record<string, string | null>): Record<string, string | null> {
  const { [SHELL_WORKTREE_SCOPE]: _shell, ...windows } = byScope;
  return windows;
}

/** Focus a project; a different project keeps window picks and drops the ambient one. */
function focusedProject(context: ActiveWorkspaceContext, workspaceId: string) {
  const sameProject = context.selectedWorkspaceId === workspaceId;
  return {
    ...context,
    breadthMode: "focused" as const,
    focus: { kind: "project" as const, workspaceId },
    selectedWorkspaceId: workspaceId,
    worktreeByScope: sameProject
      ? context.worktreeByScope
      : withoutShellScope(context.worktreeByScope),
  };
}

/**
 * A scope with no entry of its own follows the shell selection: the selected
 * worktree is the default target for new work. An explicit entry — including
 * `null`, the workspace root — overrides inheritance.
 */
export function resolveScopedWorktreeId(
  byScope: Record<string, string | null>,
  scopeId: string
): string | null {
  if (Object.hasOwn(byScope, scopeId)) return byScope[scopeId] ?? null;
  return byScope[SHELL_WORKTREE_SCOPE] ?? null;
}

export const activeWorkspaceStore = createStore({
  context: initialActiveWorkspaceContext,
  on: {
    workspaceSelected: (context, event: { workspaceId: string; scopeId?: string }) => {
      const next = focusedProject(context, event.workspaceId);
      if (context.selectedWorkspaceId !== event.workspaceId || event.scopeId === undefined) {
        return next;
      }
      return {
        ...next,
        worktreeByScope: {
          ...context.worktreeByScope,
          [event.scopeId]: null,
          [SHELL_WORKTREE_SCOPE]: null,
        },
      };
    },
    userFocused: context => ({
      ...context,
      breadthMode: "focused" as const,
      focus: USER_FOCUS,
    }),
    allProjectsEnabled: context => ({
      ...context,
      breadthMode: "all" as const,
    }),
    /** Always possible: the focus is a project or User, and User is always a target. */
    allProjectsDisabled: context => ({
      ...context,
      breadthMode: "focused" as const,
    }),
    workspaceSelectionCleared: () => initialActiveWorkspaceContext,
    /** Workspace and worktree move together so the pair is never inconsistent. */
    worktreeSelected: (
      context,
      event: { scopeId: string; workspaceId: string; worktreeId: string }
    ) =>
      withScope(
        withScope(focusedProject(context, event.workspaceId), event.scopeId, event.worktreeId),
        SHELL_WORKTREE_SCOPE,
        event.worktreeId
      ),
    worktreeRemoved: (context, event: { workspaceId: string; worktreeId: string }) => {
      if (context.selectedWorkspaceId !== event.workspaceId) return;
      return {
        ...context,
        worktreeByScope: Object.fromEntries(
          Object.entries(context.worktreeByScope).map(([key, id]) => [
            key,
            id === event.worktreeId ? null : id,
          ])
        ),
      };
    },
    worktreeSelectionCleared: (context, event: { scopeId: string }) =>
      withScope(context, event.scopeId, null),
    /**
     * Windows close from any client, so no single owner can release a scope
     * deterministically — readers prune against a fresh partition snapshot.
     */
    worktreeScopesPruned: (
      context,
      event: { partition: string; liveScopeIds: readonly string[] }
    ) => prunedWorktreeScopes(context, event.partition, event.liveScopeIds, SHELL_WORKTREE_SCOPE),
  },
}).with(
  persist({
    name: ACTIVE_WORKSPACE_PERSIST_KEY,
    version: ACTIVE_WORKSPACE_SCHEMA_VERSION,
    // Window cleanup is derived from the daemon, not a new navigation preference.
    // Persisting it would let a background document overwrite another tab's choice.
    filter: event => event.type !== "worktreeScopesPruned",
    migrate: migrateActiveWorkspace,
    pick: (context): PersistedActiveWorkspace => ({
      breadthMode: context.breadthMode,
      focus: context.focus,
      selectedWorkspaceId: context.selectedWorkspaceId,
      worktreeByScope: context.worktreeByScope,
      worktreePartitionByScope: context.worktreePartitionByScope,
    }),
  })
);

export const activeWorkspaceSelectors = {
  selectedWorkspaceId: activeWorkspaceStore.select(context => context.selectedWorkspaceId),
  breadthMode: activeWorkspaceStore.select(context => context.breadthMode),
  focus: activeWorkspaceStore.select(context => context.focus),
  worktreeByScope: activeWorkspaceStore.select(context => context.worktreeByScope),
};

export function setActiveWorkspaceId(
  workspaceId: string | null,
  options: { scopeId?: string } = {}
): void {
  if (workspaceId === null) {
    activeWorkspaceStore.trigger.workspaceSelectionCleared();
    return;
  }
  activeWorkspaceStore.trigger.workspaceSelected({ workspaceId, scopeId: options.scopeId });
}

/** Focus the User destination: its own desktop, User-scope items only. */
export function focusUser(): void {
  activeWorkspaceStore.trigger.userFocused();
}

/** Focus a destination from the switcher; both leave All projects. */
export function focusDestination(focus: Focus, options: { scopeId?: string } = {}): void {
  if (focus.kind === "user") {
    focusUser();
    return;
  }
  setActiveWorkspaceId(focus.workspaceId, options);
}

export function enableAllProjects(): void {
  activeWorkspaceStore.trigger.allProjectsEnabled();
}

export function disableAllProjects(): void {
  activeWorkspaceStore.trigger.allProjectsDisabled();
}

/**
 * Flip relative to the *resolved* breadth, not the stored one. Turning All
 * projects off returns to the resolved focus, which the resolver already
 * coerced to User when the remembered project is gone.
 */
export function toggleAllProjects(input: { breadthMode: BreadthMode; focus: Focus }): void {
  if (input.breadthMode === "focused") {
    enableAllProjects();
    return;
  }
  focusDestination(input.focus);
}

export function clearActiveWorkspaceSelection(): void {
  activeWorkspaceStore.trigger.workspaceSelectionCleared();
}

export function setActiveWorktreeId(
  scopeId: string,
  workspaceId: string,
  worktreeId: string | null
): void {
  if (worktreeId === null) {
    activeWorkspaceStore.trigger.worktreeSelectionCleared({ scopeId });
    return;
  }
  activeWorkspaceStore.trigger.worktreeSelected({ scopeId, workspaceId, worktreeId });
}

export function clearActiveWorktreeSelection(scopeId: string): void {
  activeWorkspaceStore.trigger.worktreeSelectionCleared({ scopeId });
}

/**
 * Drop worktree picks of windows that closed in `partition` (workspace +
 * profile). `liveScopeIds` must come from a snapshot of exactly that
 * partition fetched since it mounted.
 */
export function pruneWorktreeScopes(partition: string, liveScopeIds: readonly string[]): void {
  activeWorkspaceStore.trigger.worktreeScopesPruned({ partition, liveScopeIds });
  persistWorktreePrune(ACTIVE_WORKSPACE_PERSIST_KEY, ACTIVE_WORKSPACE_SCHEMA_VERSION, state =>
    prunedWorktreeScopes(state, partition, liveScopeIds, SHELL_WORKTREE_SCOPE)
  );
}
