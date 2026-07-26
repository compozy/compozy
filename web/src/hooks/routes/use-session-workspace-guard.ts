import { useEffect } from "react";
import { useMatchRoute, useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import { useActiveWorkspace } from "@/systems/workspace";

interface UseSessionWorkspaceGuardOptions {
  sessionWorkspaceId: string | undefined;
  agentName: string;
  sessionId?: string;
  sessionName?: string | null;
}

export function useSessionWorkspaceGuard({
  sessionWorkspaceId,
  agentName,
  sessionId,
  sessionName,
}: UseSessionWorkspaceGuardOptions): void {
  const matchRoute = useMatchRoute();
  const navigate = useNavigate();
  const { activeWorkspaceId, setActiveWorkspaceId, workspaces } = useActiveWorkspace();
  const pendingSessionRoute = matchRoute({
    to: "/agents/$name/sessions/$id",
    pending: true,
    includeSearch: false,
  });
  const isNavigatingToDifferentSession = Boolean(
    sessionId && pendingSessionRoute && pendingSessionRoute.id !== sessionId
  );

  useEffect(() => {
    if (isNavigatingToDifferentSession) return;
    if (sessionWorkspaceId && activeWorkspaceId && activeWorkspaceId !== sessionWorkspaceId) {
      const ownerWorkspace = workspaces.find(workspace => workspace.id === sessionWorkspaceId);
      const workspaceLabel = ownerWorkspace?.name?.trim() || sessionWorkspaceId;
      const sessionLabel = sessionName?.trim() || sessionId || "This session";
      const toastId = `session-workspace-mismatch:${sessionWorkspaceId}:${sessionId ?? agentName}`;
      toast.warning(`Session "${sessionLabel}" belongs to workspace "${workspaceLabel}".`, {
        id: toastId,
        action: sessionId
          ? {
              label: "Switch back",
              onClick: () => {
                setActiveWorkspaceId(sessionWorkspaceId);
                void navigate({
                  to: "/agents/$name/sessions/$id",
                  params: { name: agentName, id: sessionId },
                  replace: true,
                });
              },
            }
          : undefined,
      });
      void navigate({ to: "/agents/$name", params: { name: agentName }, replace: true });
    }
  }, [
    activeWorkspaceId,
    agentName,
    isNavigatingToDifferentSession,
    navigate,
    sessionId,
    sessionName,
    sessionWorkspaceId,
    setActiveWorkspaceId,
    workspaces,
  ]);
}
