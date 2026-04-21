import { useCallback, useEffect, useRef, useState, type ReactElement, type ReactNode } from "react";

import { useActiveWorkspace } from "../hooks/use-active-workspace";
import { ActiveWorkspaceContext } from "../lib/active-workspace-context";
import { AppShellBoundary } from "./app-shell-boundary";
import { WorkspaceOnboarding } from "./workspace-onboarding";
import { WorkspacePicker } from "./workspace-picker";

export interface AppShellContainerProps {
  children: ReactNode;
}

export function AppShellContainer({ children }: AppShellContainerProps): ReactElement {
  const workspace = useActiveWorkspace();
  const [showPicker, setShowPicker] = useState(false);
  const [staleSignal, setStaleSignal] = useState<string | null>(null);
  const lastResolvedRef = useRef<string | null>(null);

  useEffect(() => {
    if (workspace.isStaleSelection && workspace.selectedWorkspaceId) {
      setStaleSignal(workspace.selectedWorkspaceId);
    }
  }, [workspace.isStaleSelection, workspace.selectedWorkspaceId]);

  useEffect(() => {
    if (workspace.activeWorkspaceId && workspace.activeWorkspaceId !== lastResolvedRef.current) {
      lastResolvedRef.current = workspace.activeWorkspaceId;
      setStaleSignal(null);
    }
  }, [workspace.activeWorkspaceId]);

  const handleSwitchWorkspace = useCallback(() => {
    setShowPicker(true);
  }, []);

  const handleSelect = useCallback(
    (workspaceId: string) => {
      workspace.setActiveWorkspaceId(workspaceId);
      setStaleSignal(null);
      setShowPicker(false);
    },
    [workspace]
  );

  if (workspace.status === "loading") {
    return (
      <AppShellBoundary
        description="The daemon is enumerating registered workspaces."
        eyebrow="Loading"
        testId="app-shell-loading"
        title="Loading workspaces"
      />
    );
  }

  if (workspace.status === "error") {
    const detail = workspace.error?.message ?? "Unable to reach the daemon workspace service.";
    return (
      <AppShellBoundary
        description="Unable to load workspaces from the daemon."
        detail={detail}
        eyebrow="Workspace"
        testId="app-shell-error"
        title="Unable to load workspaces"
      />
    );
  }

  if (workspace.status === "empty") {
    return <WorkspaceOnboarding onWorkspaceResolved={handleSelect} />;
  }

  const shouldShowPicker =
    workspace.status === "many" || (showPicker && workspace.workspaces.length > 1);
  if (shouldShowPicker) {
    return (
      <WorkspacePicker
        onSelect={handleSelect}
        staleWorkspaceId={staleSignal}
        workspaces={workspace.workspaces}
      />
    );
  }

  if (!workspace.activeWorkspace) {
    return (
      <WorkspacePicker
        onSelect={handleSelect}
        staleWorkspaceId={staleSignal}
        workspaces={workspace.workspaces}
      />
    );
  }

  return (
    <ActiveWorkspaceContext.Provider
      value={{
        activeWorkspace: workspace.activeWorkspace,
        workspaces: workspace.workspaces,
        onSwitchWorkspace: handleSwitchWorkspace,
      }}
    >
      {children}
    </ActiveWorkspaceContext.Provider>
  );
}
