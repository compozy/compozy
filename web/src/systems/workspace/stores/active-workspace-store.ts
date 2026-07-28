import type { StateCreator } from "zustand";

export interface ActiveWorkspaceState {
  selectedWorkspaceId: string | null;
}

export interface ActiveWorkspaceActions {
  setSelectedWorkspaceId: (workspaceId: string | null) => void;
  clearSelectedWorkspaceId: () => void;
}

export type ActiveWorkspaceStore = ActiveWorkspaceState & ActiveWorkspaceActions;

export const initialActiveWorkspaceState: ActiveWorkspaceState = {
  selectedWorkspaceId: null,
};

export const createActiveWorkspaceStore: StateCreator<ActiveWorkspaceStore> = set => ({
  ...initialActiveWorkspaceState,
  setSelectedWorkspaceId: selectedWorkspaceId => set({ selectedWorkspaceId }),
  clearSelectedWorkspaceId: () => set(initialActiveWorkspaceState),
});
