import { useCallback, useEffect, useRef, useState, type ReactElement, type ReactNode } from "react";

import {
  useQueryClient,
  type MutationCacheNotifyEvent,
  type QueryCacheNotifyEvent,
} from "@tanstack/react-query";

import { isStaleWorkspaceError } from "@/lib/api-client";

import { ActiveWorkspaceContext } from "../lib/active-workspace-context";
import { useActiveWorkspace } from "../hooks/use-active-workspace";
import { useSyncWorkspaces } from "../hooks/use-workspaces";
import type { WorkspaceSyncResult } from "../types";
import { AppShellBoundary } from "./app-shell-boundary";
import { WorkspaceOnboarding } from "./workspace-onboarding";
import { WorkspacePicker } from "./workspace-picker";

export interface AppShellContainerProps {
  children: ReactNode;
}

export function AppShellContainer({ children }: AppShellContainerProps): ReactElement {
  const queryClient = useQueryClient();
  const workspace = useActiveWorkspace();
  const syncWorkspaces = useSyncWorkspaces();
  const clearActiveWorkspaceSelection = workspace.clearActiveWorkspaceSelection;
  const [showPicker, setShowPicker] = useState(false);
  const [staleSignal, setStaleSignal] = useState<string | null>(null);
  const [syncMessage, setSyncMessage] = useState<string | null>(null);
  const lastResolvedRef = useRef<string | null>(null);
  const activeWorkspaceIdRef = useRef<string | null>(workspace.activeWorkspaceId);
  const selectedWorkspaceIdRef = useRef<string | null>(workspace.selectedWorkspaceId);

  useEffect(() => {
    activeWorkspaceIdRef.current = workspace.activeWorkspaceId;
    selectedWorkspaceIdRef.current = workspace.selectedWorkspaceId;
  }, [workspace.activeWorkspaceId, workspace.selectedWorkspaceId]);

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

  useEffect(() => {
    const handlePossibleStaleWorkspace = (error: unknown) => {
      if (!isStaleWorkspaceError(error)) {
        return;
      }
      const staleWorkspaceId = selectedWorkspaceIdRef.current ?? activeWorkspaceIdRef.current;
      setStaleSignal(staleWorkspaceId ?? "unknown");
      clearActiveWorkspaceSelection();
      setShowPicker(true);
    };

    const unsubscribeQueries = queryClient
      .getQueryCache()
      .subscribe((event: QueryCacheNotifyEvent) => {
        if (event.type === "updated") {
          handlePossibleStaleWorkspace(event.query.state.error);
        }
      });
    const unsubscribeMutations = queryClient
      .getMutationCache()
      .subscribe((event: MutationCacheNotifyEvent) => {
        if (event.type === "updated") {
          handlePossibleStaleWorkspace(event.mutation.state.error);
        }
      });

    return () => {
      unsubscribeQueries();
      unsubscribeMutations();
    };
  }, [clearActiveWorkspaceSelection, queryClient]);

  const handleSwitchWorkspace = useCallback(() => {
    setShowPicker(true);
  }, []);

  const handleSelect = useCallback(
    (workspaceId: string) => {
      workspace.setActiveWorkspaceId(workspaceId);
      setStaleSignal(null);
      setSyncMessage(null);
      setShowPicker(false);
    },
    [workspace]
  );

  const handleSyncWorkspaces = useCallback(async () => {
    setSyncMessage(null);
    try {
      const result = await syncWorkspaces.mutateAsync();
      setSyncMessage(formatWorkspaceSyncResult(result));
      setStaleSignal(null);
    } catch {
      // Mutation state owns the displayed error.
    }
  }, [syncWorkspaces]);

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
    staleSignal !== null ||
    workspace.status === "many" ||
    (showPicker && workspace.workspaces.length > 1);
  if (shouldShowPicker) {
    return (
      <WorkspacePicker
        isSyncing={syncWorkspaces.isPending}
        onSelect={handleSelect}
        onSync={() => {
          void handleSyncWorkspaces();
        }}
        syncError={syncWorkspaces.error?.message ?? null}
        syncMessage={syncMessage}
        staleWorkspaceId={staleSignal}
        workspaces={workspace.workspaces}
      />
    );
  }

  if (!workspace.activeWorkspace) {
    return (
      <WorkspacePicker
        isSyncing={syncWorkspaces.isPending}
        onSelect={handleSelect}
        onSync={() => {
          void handleSyncWorkspaces();
        }}
        syncError={syncWorkspaces.error?.message ?? null}
        syncMessage={syncMessage}
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

function formatWorkspaceSyncResult(result: WorkspaceSyncResult): string {
  const checked = result.checked;
  const removed = result.removed;
  const missing = result.missing;
  const synced = result.synced;
  const warnings = result.warnings?.length ?? 0;
  return [
    `${checked} checked`,
    `${synced} synced`,
    `${missing} missing`,
    `${removed} removed`,
    `${warnings} warning${warnings === 1 ? "" : "s"}`,
  ].join(" · ");
}
