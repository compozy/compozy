import { useScopedWorktreeFilter } from "@/systems/workspace";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";

import { useOsSessionsModal } from "../../hooks/use-os-sessions-modal";
import { useAttentionJump } from "../../hooks/use-attention-jump";
import {
  type SessionLifecycleActionHandlers,
  type SessionPayload,
  sessionListSortParam,
  useSessionCreateActions,
  useSessionLifecycleActions,
  useSessionListView,
  useSessions,
  useSessionSidebarState,
  type SessionListViewModel,
} from "@/systems/session";

export interface SessionWindowSidebarModel {
  open: boolean;
  toggle: () => void;
  sessions: SessionPayload[];
  disconnected: boolean;
  collapsedAgentIds: readonly string[];
  collapsedThreadIds: string[];
  view: SessionListViewModel;
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
  const jumpToSession = useAttentionJump();
  const { openForAgent } = useSessionCreateActions();
  const lifecycle = useSessionLifecycleActions({ workspaceId });
  const view = useSessionListView();
  // Scoped to this window: two session windows can hold different worktrees and
  // must list different sessions.
  const worktree = useScopedWorktreeFilter(workspaceId, useWorktreeScopeId(), {
    enabled: sidebar.open,
  });
  const sessionsQuery = useSessions(workspaceId, {
    enabled: sidebar.open && worktree.resolved,
    filters: {
      include_health: true,
      limit: 100,
      // Ordering is served, not re-sorted client-side: the daemon owns the
      // attention band and its tie-breaks.
      sort: sessionListSortParam(view.sort),
      worktree: worktree.worktreeId,
    },
  });

  const onSelectSession = (target: SessionPayload) => {
    if (target.id === sessionId) return;
    if (target.workspace_id !== workspaceId) {
      jumpToSession({
        sessionId: target.id,
        agentName: target.agent_name,
        workspaceId: target.workspace_id ?? workspaceId,
      });
      return;
    }
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
    disconnected: sidebar.open && worktree.resolved && sessionsQuery.isError,
    collapsedAgentIds,
    collapsedThreadIds: sidebar.collapsedThreadIds,
    view,
    onToggleGroup: agentName => manager.toggleRailGroup(agentName),
    onToggleThread: sidebar.toggleThread,
    onSelectSession,
    onNewSession: () => openForAgent(""),
    sessionActions: lifecycle.actions,
    rowDeleteDialog: lifecycle.deleteDialog,
    rowRenameDialog: lifecycle.renameDialog,
  };
}
