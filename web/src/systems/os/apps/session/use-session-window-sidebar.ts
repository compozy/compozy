import { useDataScope, useScopedWorktreeFilter } from "@/systems/workspace";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";

import { useSessionListLifecycleActions } from "../../hooks/use-session-list-lifecycle-actions";
import {
  useSessionWindowOpenSession,
  type SessionWindowSidebarTarget,
} from "./use-session-window-open-session";
import {
  type SessionLifecycleActionHandlers,
  type SessionListTarget,
  type SessionPayload,
  type SessionThreadBranchChange,
  useSessionCreateActions,
  useSessionListView,
  useSessionSidebarState,
  useSessionTransportState,
  useSessionWindowPresentation,
  useTransportGraceElapsed,
  type SessionListViewModel,
  type SessionCreateOpenOptions,
} from "@/systems/session";

export type { SessionWindowSidebarTarget };

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
  rowProcessDialog: ReturnType<typeof useSessionListLifecycleActions>["processDialog"];
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
  const sidebar = useSessionSidebarState();
  const presentation = useSessionWindowPresentation();
  // Over its board the window shows no rail, so the rail's list is never read there.
  const railShown = sidebar.open && presentation === "window";
  const transport = useSessionTransportState();
  const transportGraceElapsed = useTransportGraceElapsed(transport.degradedAt);
  const { openForAgent } = useSessionCreateActions();
  const lifecycle = useSessionListLifecycleActions();
  // Only a project narrows to a worktree.
  const worktree = useScopedWorktreeFilter(
    scope?.kind === "project" ? scope.workspaceId : null,
    useWorktreeScopeId(),
    { enabled: railShown }
  );
  const view = useSessionListView({
    scope,
    worktreeId: worktree.worktreeId,
    enabled: railShown && worktree.resolved && scope !== null,
    // An open rail waiting on its destination or worktree filter is loading, not empty.
    pending: railShown && (!worktree.resolved || scope === null),
  });

  const openSession = useSessionWindowOpenSession({ windowId, sessionId });

  const onSelectSession = (target: SessionListTarget) =>
    openSession(
      {
        sessionId: target.id,
        agentName: target.agent_name,
        workspaceId: target.workspace_id ?? "",
        ...(target.subagentId ? { subagentId: target.subagentId } : {}),
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
    rowProcessDialog: lifecycle.processDialog,
    rowRenameDialog: lifecycle.renameDialog,
  };
}
