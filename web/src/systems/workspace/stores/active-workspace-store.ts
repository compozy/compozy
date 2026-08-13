import { createStore } from "@xstate/store";
import { persist } from "@xstate/store/persist";

import type { WorkspaceScopeMode } from "../lib/workspace-scope-mode";

export type { WorkspaceScopeMode };

export interface ActiveWorkspaceContext {
  scope: WorkspaceScopeMode;
  selectedWorkspaceId: string | null;
}

export const ACTIVE_WORKSPACE_PERSIST_KEY = "compozy:active-workspace:v3";

const initialActiveWorkspaceContext: ActiveWorkspaceContext = {
  scope: "global",
  selectedWorkspaceId: null,
};

export const activeWorkspaceStore = createStore({
  context: initialActiveWorkspaceContext,
  on: {
    workspaceSelected: (context, event: { workspaceId: string }) => ({
      ...context,
      scope: "workspace" as const,
      selectedWorkspaceId: event.workspaceId,
    }),
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
    }),
  },
}).with(
  persist({
    name: ACTIVE_WORKSPACE_PERSIST_KEY,
    pick: context => ({
      scope: context.scope,
      selectedWorkspaceId: context.selectedWorkspaceId,
    }),
  })
);

export const activeWorkspaceSelectors = {
  selectedWorkspaceId: activeWorkspaceStore.select(context => context.selectedWorkspaceId),
  scope: activeWorkspaceStore.select(context => context.scope),
};

export function setActiveWorkspaceId(workspaceId: string | null): void {
  if (workspaceId === null) {
    activeWorkspaceStore.trigger.workspaceSelectionCleared();
    return;
  }
  activeWorkspaceStore.trigger.workspaceSelected({ workspaceId });
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
