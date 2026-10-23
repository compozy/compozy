import {
  ownerDataScope,
  sameDataScope,
  useDataScope,
  useScopedWorktreeFilter,
  useWindowDataWorkspace,
} from "@/systems/workspace";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";

import { runViewTransition } from "@compozy/ui";

import { useOsReducedMotion } from "../../hooks/use-os-reduced-motion";
import { useOsShell } from "../../hooks/use-os-shell";
import { useSessionListLifecycleActions } from "../../hooks/use-session-list-lifecycle-actions";
import { useAttentionJump } from "../../hooks/use-attention-jump";
import { boundRoute } from "../../lib/open-target";
import {
  type SessionLifecycleActionHandlers,
  type SessionListTarget,
  type SessionPayload,
  type SessionThreadBranchChange,
  useSessionCreateActions,
  useSessionListView,
  useSessionSidebarState,
  useSessionTransportState,
  useTransportGraceElapsed,
  type SessionListViewModel,
  type SessionCreateOpenOptions,
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
  subagentThreadIds: string[];
  view: SessionListViewModel;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  /** Drill-in by id (subagent rows): this window, or a split with `newWindow`. */
  openSession: (target: SessionWindowSidebarTarget, options: { newWindow: boolean }) => void;
  onNewSession: (options?: SessionCreateOpenOptions) => void;
  sessionActions: SessionLifecycleActionHandlers;
  rowDeleteDialog: ReturnType<typeof useSessionListLifecycleActions>["deleteDialog"];
  rowRenameDialog: ReturnType<typeof useSessionListLifecycleActions>["renameDialog"];
}

/**
 * In-window sessions rail view-model. It follows the topbar breadth — the
 * aggregate under All projects, the focused project or User otherwise — and
 * shares the shell's catalog query key, so the list is cache-warm on first
 * open and stays live through the catalog stream. Row activation retargets
 * this window in place (rebinding it under All projects), unless the target
 * belongs elsewhere or already owns a window — that window wins focus instead.
 */
export function useSessionWindowSidebar({
  transportDisconnected = false,
  windowId,
  sessionId,
}: {
  /** The window's live stream is down; the rail must not read "connected" (US-018.AC-1). */
  transportDisconnected?: boolean;
  windowId: string;
  sessionId?: string;
}): SessionWindowSidebarModel {
  const scope = useDataScope();
  const windowScope = useWindowDataWorkspace();
  const sidebar = useSessionSidebarState();
  const transport = useSessionTransportState();
  const transportGraceElapsed = useTransportGraceElapsed(transport.degradedAt);
  const { coordinator } = useOsShell();
  const jumpToSession = useAttentionJump();
  const { openForAgent } = useSessionCreateActions();
  const lifecycle = useSessionListLifecycleActions();
  // Only a project narrows to a worktree.
  const worktree = useScopedWorktreeFilter(
    scope?.kind === "project" ? scope.workspaceId : null,
    useWorktreeScopeId(),
    { enabled: sidebar.open }
  );
  const view = useSessionListView({
    scope,
    worktreeId: worktree.worktreeId,
    enabled: sidebar.open && worktree.resolved && scope !== null,
  });

  const reducedMotion = useOsReducedMotion();

  const openSession = (target: SessionWindowSidebarTarget, options: { newWindow: boolean }) => {
    if (sessionId !== undefined && target.sessionId === sessionId) return;
    const targetScope = ownerDataScope(target.workspaceId);
    const allProjects = scope?.kind === "all";
    const sameScope = windowScope !== null && sameDataScope(windowScope, targetScope);
    if (options.newWindow || !target.agentName || (!allProjects && !sameScope)) {
      jumpToSession({
        sessionId: target.sessionId,
        agentName: target.agentName || undefined,
        workspaceId: target.workspaceId,
        ...(options.newWindow ? { placement: "split" as const } : {}),
      });
      return;
    }
    const route = {
      pathname: `/agents/${encodeURIComponent(target.agentName)}/sessions/${encodeURIComponent(target.sessionId)}`,
      search: {},
    };
    // Same window, different conversation: the pane (named per window in
    // session-window-content) cross-fades instead of snapping to the skeleton.
    void runViewTransition(
      () => {
        void coordinator.userRetarget(windowId, {
          app: "session",
          instanceKey: target.sessionId,
          // Under All projects the window follows the session it now shows.
          route: allProjects ? boundRoute({ binding: targetScope, route }) : route,
        });
      },
      { reduced: reducedMotion }
    );
  };

  const onSelectSession = (target: SessionListTarget) =>
    openSession(
      {
        sessionId: target.id,
        agentName: target.agent_name,
        workspaceId: target.workspace_id ?? "",
      },
      { newWindow: false }
    );

  return {
    open: sidebar.open,
    toggle: sidebar.toggle,
    sessions: view.catalog?.sessions ?? [],
    disconnected:
      sidebar.open &&
      worktree.resolved &&
      (view.catalog?.failed || (transportDisconnected && transportGraceElapsed)),
    collapsedThreadIds: sidebar.collapsedThreadIds,
    subagentThreadIds: sidebar.subagentThreadIds,
    view,
    onThreadBranchChange: sidebar.setThreadBranch,
    onSelectSession,
    openSession,
    onNewSession: options => openForAgent("", options),
    sessionActions: lifecycle.actions,
    rowDeleteDialog: lifecycle.deleteDialog,
    rowRenameDialog: lifecycle.renameDialog,
  };
}
