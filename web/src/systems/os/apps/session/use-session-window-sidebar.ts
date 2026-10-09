import { useScopedWorktreeFilter } from "@/systems/workspace";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";

import { runViewTransition } from "@compozy/ui";

import { useOsReducedMotion } from "../../hooks/use-os-reduced-motion";
import { useOsShell } from "../../hooks/use-os-shell";
import { useAttentionJump } from "../../hooks/use-attention-jump";
import {
  type SessionLifecycleActionHandlers,
  type SessionPayload,
  useSessionCreateActions,
  useSessionLifecycleActions,
  useSessionListView,
  useSessionSidebarState,
  type SessionListViewModel,
} from "@/systems/session";

/** A session to open by id; the agent name lets this window retarget in place. */
export interface SessionWindowSidebarTarget {
  sessionId: string;
  agentName: string;
  workspaceId: string;
}

export interface SessionWindowSidebarModel {
  open: boolean;
  toggle: () => void;
  sessions: SessionPayload[];
  disconnected: boolean;
  collapsedThreadIds: string[];
  view: SessionListViewModel;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  /** Drill-in by id (subagent rows): this window, or a split with `newWindow`. */
  openSession: (target: SessionWindowSidebarTarget, options: { newWindow: boolean }) => void;
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
  transportDisconnected = false,
  windowId,
  workspaceId,
  sessionId,
}: {
  /** The window's live stream is down; the rail must not read "connected" (US-018.AC-1). */
  transportDisconnected?: boolean;
  windowId: string;
  workspaceId: string;
  sessionId?: string;
}): SessionWindowSidebarModel {
  const sidebar = useSessionSidebarState();
  const { coordinator } = useOsShell();
  const jumpToSession = useAttentionJump();
  const { openForAgent } = useSessionCreateActions();
  const lifecycle = useSessionLifecycleActions({ workspaceId });
  const worktree = useScopedWorktreeFilter(workspaceId, useWorktreeScopeId(), {
    enabled: sidebar.open,
  });
  const view = useSessionListView({
    workspaceId,
    worktreeId: worktree.worktreeId,
    enabled: sidebar.open && worktree.resolved && workspaceId !== "",
  });

  const reducedMotion = useOsReducedMotion();

  const openSession = (target: SessionWindowSidebarTarget, options: { newWindow: boolean }) => {
    if (sessionId !== undefined && target.sessionId === sessionId) return;
    const targetWorkspaceId = target.workspaceId || workspaceId;
    if (options.newWindow || targetWorkspaceId !== workspaceId || !target.agentName) {
      jumpToSession({
        sessionId: target.sessionId,
        agentName: target.agentName || undefined,
        workspaceId: targetWorkspaceId,
        ...(options.newWindow ? { placement: "split" as const } : {}),
      });
      return;
    }
    // Same window, different conversation: the pane (named per window in
    // session-window-content) cross-fades instead of snapping to the skeleton.
    void runViewTransition(
      () => {
        void coordinator.userRetarget(windowId, {
          app: "session",
          instanceKey: target.sessionId,
          route: {
            pathname: `/agents/${encodeURIComponent(target.agentName)}/sessions/${encodeURIComponent(target.sessionId)}`,
            search: {},
          },
        });
      },
      { reduced: reducedMotion }
    );
  };

  const onSelectSession = (target: SessionPayload) =>
    openSession(
      {
        sessionId: target.id,
        agentName: target.agent_name,
        workspaceId: target.workspace_id ?? workspaceId,
      },
      { newWindow: false }
    );

  return {
    open: sidebar.open,
    toggle: sidebar.toggle,
    sessions: view.catalog?.sessions ?? [],
    disconnected:
      sidebar.open && worktree.resolved && (view.catalog?.failed || transportDisconnected),
    collapsedThreadIds: sidebar.collapsedThreadIds,
    view,
    onToggleThread: sidebar.toggleThread,
    onSelectSession,
    openSession,
    onNewSession: () => openForAgent(""),
    sessionActions: lifecycle.actions,
    rowDeleteDialog: lifecycle.deleteDialog,
    rowRenameDialog: lifecycle.renameDialog,
  };
}
