import { create } from "zustand";

import { readActiveWorkspaceId, writeActiveWorkspaceId } from "@/lib/session-storage";

export interface ActiveWorkspaceState {
  selectedWorkspaceId: string | null;
  setSelectedWorkspaceId: (workspaceId: string | null) => void;
  clearSelectedWorkspaceId: () => void;
}

export const useActiveWorkspaceStore = create<ActiveWorkspaceState>(set => ({
  selectedWorkspaceId: readActiveWorkspaceId(),
  setSelectedWorkspaceId: workspaceId => {
    writeActiveWorkspaceId(workspaceId);
    set({ selectedWorkspaceId: workspaceId });
  },
  clearSelectedWorkspaceId: () => {
    writeActiveWorkspaceId(null);
    set({ selectedWorkspaceId: null });
  },
}));

export function resetActiveWorkspaceStoreForTests(): void {
  writeActiveWorkspaceId(null);
  useActiveWorkspaceStore.setState({ selectedWorkspaceId: null });
}
