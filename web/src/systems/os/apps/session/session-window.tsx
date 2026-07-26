import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { toast } from "sonner";

import { Spinner } from "@agh/ui";

import { sessionByIdOptions } from "@/systems/session";

import { useDesktop } from "../../hooks/use-desktop";
import { useOsShell } from "../../hooks/use-os-shell";
import { matchSessionInstance } from "../../lib/app-registry";
import { SessionWindowNotice, SessionWindowView } from "./session-window-view";

const SESSION_AGENT_PATTERN = /^\/agents\/([^/]+)\/sessions\//;

/**
 * Session window controller: parses `agent + session` identity from
 * the window's WM location and resolves the owning workspace the same way the
 * route loader does (session-by-id), then rehosts the existing session view.
 */
export function SessionWindow({ windowId }: { windowId: string }) {
  const { coordinator } = useOsShell();
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? "");
  const sessionId = matchSessionInstance(pathname);
  const agentMatch = SESSION_AGENT_PATTERN.exec(pathname);
  const agentName = agentMatch ? decodeURIComponent(agentMatch[1]) : null;

  const sessionQuery = useQuery({
    ...sessionByIdOptions(sessionId ?? ""),
    enabled: sessionId !== null,
  });

  useEffect(() => {
    if (!sessionQuery.error?.message.includes("not found") || agentName === null) return;
    toast.error("Session not found");
    void coordinator.userClose(windowId).then(closed => {
      if (!closed) return;
      void coordinator.userOpen({
        app: "agents",
        route: {
          pathname: `/agents/${encodeURIComponent(agentName)}`,
          search: {},
        },
      });
    });
  }, [agentName, coordinator, sessionQuery.error, windowId]);

  if (sessionId === null || agentName === null) {
    return <SessionWindowNotice message="This window does not point at a session." />;
  }
  if (sessionQuery.isLoading) {
    return (
      <div
        className="flex min-h-full items-center justify-center"
        data-testid="session-route-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }

  const workspaceId = sessionQuery.data?.workspace_id?.trim() || null;
  if (workspaceId === null) {
    return (
      <SessionWindowNotice
        message={sessionQuery.error?.message ?? "Session workspace unavailable"}
      />
    );
  }

  return (
    <div className="flex min-h-full min-w-0 flex-col">
      <SessionWindowView
        name={agentName}
        id={sessionId}
        workspaceId={workspaceId}
        onDeleteSuccess={() => {
          void coordinator.userClose(windowId).then(closed => {
            if (!closed) return;
            void coordinator.userOpen({
              app: "agents",
              route: {
                pathname: `/agents/${encodeURIComponent(agentName)}`,
                search: {},
              },
            });
          });
        }}
      />
    </div>
  );
}
