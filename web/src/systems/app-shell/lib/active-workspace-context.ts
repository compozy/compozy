import { createContext, useContext } from "react";

import type { Workspace } from "../types";

export interface ActiveWorkspaceContextValue {
  activeWorkspace: Workspace;
  workspaces: Workspace[];
  onSwitchWorkspace: () => void;
}

export const ActiveWorkspaceContext = createContext<ActiveWorkspaceContextValue | null>(null);

export function useActiveWorkspaceContext(): ActiveWorkspaceContextValue {
  const value = useContext(ActiveWorkspaceContext);
  if (!value) {
    throw new Error(
      "useActiveWorkspaceContext must be called inside an AppShellContainer provider"
    );
  }
  return value;
}
