import { createStore } from "@xstate/store";
import { persist } from "@xstate/store/persist";

import type { WorkspaceScopeMode } from "../lib/workspace-scope-mode";

export type { WorkspaceScopeMode };

/**
 * Scope id for surfaces outside a retained OS window (shell chrome, standalone
 * routes, Storybook). Real windows key by their own daemon-issued window id.
 */
export const SHELL_WORKTREE_SCOPE = "shell";

export interface ActiveWorkspaceContext {
  scope: WorkspaceScopeMode;
  /**
   * Remembered project workspace. Survives Global mode so turning Global off
   * returns to the same project. Never the operator-home registration.
   */
  selectedWorkspaceId: string | null;
  /**
   * Per-scope worktree selection: each OS window carries its own worktree
   * without a second window-manager binding.
   */
  worktreeByScope: Record<string, string | null>;
}

export const ACTIVE_WORKSPACE_PERSIST_KEY = "compozy:active-workspace:v4";

const initialActiveWorkspaceContext: ActiveWorkspaceContext = {
  scope: "global",
  selectedWorkspaceId: null,
  worktreeByScope: {},
};

function withScope(
  context: ActiveWorkspaceContext,
  scopeId: string,
  worktreeId: string | null
): ActiveWorkspaceContext {
  return { ...context, worktreeByScope: { ...context.worktreeByScope, [scopeId]: worktreeId } };
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
      if (context.selectedWorkspaceId === event.workspaceId) {
        const base = { ...context, scope: "workspace" as const };
        if (event.scopeId === undefined) return base;
        return {
          ...base,
          worktreeByScope: {
            ...context.worktreeByScope,
            [event.scopeId]: null,
            [SHELL_WORKTREE_SCOPE]: null,
          },
        };
      }
      return {
        ...context,
        scope: "workspace" as const,
        selectedWorkspaceId: event.workspaceId,
        worktreeByScope: {},
      };
    },
    globalScopeEnabled: context => ({
      ...context,
      scope: "global" as const,
    }),
    globalScopeDisabled: context => {
      if (!context.selectedWorkspaceId) return;
      return {
        ...context,
        scope: "workspace" as const,
      };
    },
    workspaceSelectionCleared: () => ({
      scope: "global" as const,
      selectedWorkspaceId: null,
      worktreeByScope: {},
    }),
    /** Workspace and worktree move together so the pair is never inconsistent. */
    worktreeSelected: (
      context,
      event: { scopeId: string; workspaceId: string; worktreeId: string }
    ) => {
      const rebased =
        context.selectedWorkspaceId === event.workspaceId
          ? { ...context, scope: "workspace" as const }
          : {
              ...context,
              scope: "workspace" as const,
              selectedWorkspaceId: event.workspaceId,
              worktreeByScope: {},
            };
      return withScope(
        withScope(rebased, event.scopeId, event.worktreeId),
        SHELL_WORKTREE_SCOPE,
        event.worktreeId
      );
    },
    worktreeSelectionCleared: (context, event: { scopeId: string }) =>
      withScope(context, event.scopeId, null),
    /**
     * Windows close from any client, so no single owner can release a scope
     * deterministically — readers prune against the live window set instead.
     */
    worktreeScopesPruned: (context, event: { liveScopeIds: readonly string[] }) => {
      const live = new Set<string>([SHELL_WORKTREE_SCOPE, ...event.liveScopeIds]);
      const worktreeByScope: Record<string, string | null> = {};
      for (const [scopeId, worktreeId] of Object.entries(context.worktreeByScope)) {
        if (live.has(scopeId)) worktreeByScope[scopeId] = worktreeId;
      }
      return { ...context, worktreeByScope };
    },
  },
}).with(
  persist({
    name: ACTIVE_WORKSPACE_PERSIST_KEY,
    pick: context => ({
      scope: context.scope,
      selectedWorkspaceId: context.selectedWorkspaceId,
      worktreeByScope: context.worktreeByScope,
    }),
  })
);

export const activeWorkspaceSelectors = {
  selectedWorkspaceId: activeWorkspaceStore.select(context => context.selectedWorkspaceId),
  scope: activeWorkspaceStore.select(context => context.scope),
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

export function enableGlobalScope(): void {
  activeWorkspaceStore.trigger.globalScopeEnabled();
}

export function disableGlobalScope(): void {
  activeWorkspaceStore.trigger.globalScopeDisabled();
}

/**
 * Flip relative to the *resolved* scope, not the stored one — the store may
 * hold a stale `workspace` request the resolver already coerced to global.
 */
export function toggleGlobalScope(input: { scope: WorkspaceScopeMode; canDisable: boolean }): void {
  if (input.scope === "global") {
    if (!input.canDisable) return;
    disableGlobalScope();
    return;
  }
  enableGlobalScope();
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

export function pruneWorktreeScopes(liveScopeIds: readonly string[]): void {
  activeWorkspaceStore.trigger.worktreeScopesPruned({ liveScopeIds });
}
