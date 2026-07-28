import { createStore } from "@xstate/store";
import { persist } from "@xstate/store/persist";

export interface ActiveWorkspaceContext {
  selectedWorkspaceId: string | null;
}

const initialActiveWorkspaceContext: ActiveWorkspaceContext = {
  selectedWorkspaceId: null,
};

export const activeWorkspaceStore = createStore({
  context: initialActiveWorkspaceContext,
  on: {
    workspaceSelected: (context, event: { workspaceId: string }) => ({
      ...context,
      selectedWorkspaceId: event.workspaceId,
    }),
    workspaceSelectionCleared: context => ({
      ...context,
      selectedWorkspaceId: null,
    }),
  },
}).with(
  persist({
    name: "compozy:active-workspace:v2",
    pick: context => ({ selectedWorkspaceId: context.selectedWorkspaceId }),
  })
);

export const activeWorkspaceSelectors = {
  selectedWorkspaceId: activeWorkspaceStore.select(context => context.selectedWorkspaceId),
};

export function setActiveWorkspaceId(workspaceId: string | null): void {
  if (workspaceId === null) {
    activeWorkspaceStore.trigger.workspaceSelectionCleared();
    return;
  }
  activeWorkspaceStore.trigger.workspaceSelected({ workspaceId });
}

export function clearActiveWorkspaceSelection(): void {
  activeWorkspaceStore.trigger.workspaceSelectionCleared();
}
