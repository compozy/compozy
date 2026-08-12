import { useOsSessionsModal } from "../../hooks/use-os-sessions-modal";
import {
  type SessionLifecycleActionHandlers,
  type SessionPayload,
  useSessionCreateActions,
  useSessionLifecycleActions,
  useSessions,
  useSessionSidebarState,
} from "@/systems/session";

export interface SessionWindowSidebarModel {
  open: boolean;
  toggle: () => void;
  sessions: SessionPayload[];
  disconnected: boolean;
  collapsedAgentIds: readonly string[];
  collapsedThreadIds: string[];
  onToggleGroup: (agentName: string) => void;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  onNewSession: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  rowDeleteDialog: ReturnType<typeof useSessionLifecycleActions>["deleteDialog"];
  rowRenameDialog: ReturnType<typeof useSessionLifecycleActions>["renameDialog"];
}

/**
 * In-window sessions rail view-model. Shares the shell's catalog query key so
 * the list is cache-warm on first open and stays live through the catalog
 * stream; row activation retargets this window in place, unless the target
 * session already owns a window — that window wins focus instead.
 */
export function useSessionWindowSidebar({
  windowId,
  workspaceId,
  sessionId,
}: {
  windowId: string;
  workspaceId: string;
  sessionId: string;
}): SessionWindowSidebarModel {
  const sidebar = useSessionSidebarState();
  const { coordinator, manager, collapsedAgentIds } = useOsSessionsModal();
  const { openForAgent } = useSessionCreateActions();
  const lifecycle = useSessionLifecycleActions({ workspaceId });
  const sessionsQuery = useSessions(workspaceId, {
    enabled: sidebar.open,
    filters: { include_health: true, limit: 100, sort: "last_activity" },
  });

  const onSelectSession = (target: SessionPayload) => {
    if (target.id === sessionId) return;
    void coordinator.userRetarget(windowId, {
      app: "session",
      instanceKey: target.id,
      route: {
        pathname: `/agents/${encodeURIComponent(target.agent_name)}/sessions/${encodeURIComponent(target.id)}`,
        search: {},
      },
    });
  };

  return {
    open: sidebar.open,
    toggle: sidebar.toggle,
    sessions: sessionsQuery.data ?? [],
    disconnected: sidebar.open && sessionsQuery.isError,
    collapsedAgentIds,
    collapsedThreadIds: sidebar.collapsedThreadIds,
    onToggleGroup: agentName => manager.toggleRailGroup(agentName),
    onToggleThread: sidebar.toggleThread,
    onSelectSession,
    onNewSession: () => openForAgent(""),
    sessionActions: lifecycle.actions,
    rowDeleteDialog: lifecycle.deleteDialog,
    rowRenameDialog: lifecycle.renameDialog,
  };
}
