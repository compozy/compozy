import { useDocumentVisible } from "@/hooks/use-document-visible";

import { useLoopNodeExists } from "@/systems/loops/hooks/use-loop-node-exists";
import {
  attentionCount,
  deriveAttentionBadges,
  deriveAttentionRows,
  type OsAttentionBadges,
  type OsAttentionRow,
} from "../lib/attention-model";
import {
  type SessionCatalogStreamStatus,
  type SessionPayload,
  useSessions,
} from "@/systems/session";
import { taskScopeForActiveWorkspace, useTaskDashboard, useTasks } from "@/systems/tasks";
import {
  useActiveWorkspace,
  useScopedWorktreeFilter,
  type WorkspacePayload,
} from "@/systems/workspace";

import { useFocusedWorktreeScopeId } from "./use-worktree-scope";

const ATTENTION_REFETCH_INTERVAL_MS = 5_000;

export interface OsAttentionModel {
  badges: OsAttentionBadges;
  notificationCount: number;
  rows: OsAttentionRow[];
  sessions: SessionPayload[];
  archivedSessions: SessionPayload[];
  archivedSessionsTotal: number;
  sessionsDisconnected: boolean;
  tasksDisconnected: boolean;
  loading: boolean;
}

export function useOsAttention(
  runtimeWorkspace: WorkspacePayload | null | undefined,
  sessionCatalogStreamStatus: SessionCatalogStreamStatus
): OsAttentionModel {
  const { scope, activeWorkspaceId } = useActiveWorkspace();
  const documentVisible = useDocumentVisible();
  const workspaceId = runtimeWorkspace?.id ?? null;
  const sessionsEnabled = workspaceId !== null;
  const taskScope = taskScopeForActiveWorkspace(scope, activeWorkspaceId);
  const tasksEnabled = taskScope !== null;
  const worktree = useScopedWorktreeFilter(workspaceId, useFocusedWorktreeScopeId(), {
    enabled: sessionsEnabled && scope === "workspace",
  });
  const scopedSessionsEnabled = sessionsEnabled && worktree.resolved;
  // Attention authority: workspace-wide and never worktree-filtered. A pending
  // approval in another worktree is still a notification, so deriving badges or
  // rows from a scoped page would silently hide work.
  const attentionSessionsQuery = useSessions(workspaceId, {
    enabled: sessionsEnabled,
    filters: { include_health: true, limit: 100, sort: "last_activity" },
  });
  // Sessions-modal content: a shell surface, so it follows the focused window's
  // scope exactly like the menubar chip. Distinct query key, no shared snapshot.
  const modalSessionsQuery = useSessions(workspaceId, {
    enabled: scopedSessionsEnabled,
    filters: {
      include_health: true,
      limit: 100,
      sort: "last_activity",
      worktree: worktree.worktreeId,
    },
  });
  const archivedSessionsQuery = useSessions(workspaceId, {
    enabled: scopedSessionsEnabled,
    filters: {
      archive: "only",
      include_health: true,
      limit: 100,
      sort: "last_activity",
      worktree: worktree.worktreeId,
    },
  });
  const dashboardQuery = useTaskDashboard(taskScope ?? {}, {
    enabled: tasksEnabled,
    refetchIntervalMs: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
  });
  const tasksQuery = useTasks(
    {
      ...taskScope,
      approval_state: "pending",
      limit: 100,
      sort: "recent",
    },
    {
      enabled: tasksEnabled,
      refetchIntervalMs: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
    }
  );
  const loopWorkspaceId = workspaceId ?? "";
  const loopWaitingPresent = useLoopNodeExists(loopWorkspaceId, "waiting", sessionsEnabled);
  const loopAttentionPresent = useLoopNodeExists(loopWorkspaceId, "attention", sessionsEnabled);

  const attentionSessions = attentionSessionsQuery.data ?? [];
  const modalSessions = modalSessionsQuery.data ?? [];
  const archivedSessions = archivedSessionsQuery.data ?? [];
  const dashboard = dashboardQuery.data ?? null;
  const tasks = tasksQuery.data ?? [];
  // Staleness follows each consumer: badges must not read fresh off the scoped
  // page, and the modal must not read fresh off the unscoped one.
  const attentionSessionsStale =
    !sessionsEnabled ||
    sessionCatalogStreamStatus !== "live" ||
    attentionSessionsQuery.isError ||
    attentionSessionsQuery.data === undefined;
  const sessionsDisconnected =
    !sessionsEnabled ||
    (worktree.resolved &&
      (sessionCatalogStreamStatus !== "live" ||
        modalSessionsQuery.isError ||
        archivedSessionsQuery.isError ||
        modalSessionsQuery.data === undefined ||
        archivedSessionsQuery.data === undefined));
  const tasksDisconnected =
    !tasksEnabled ||
    dashboardQuery.isError ||
    dashboardQuery.data === undefined ||
    (dashboard?.freshness.stale ?? true);
  const taskRowsDisconnected =
    tasksDisconnected || tasksQuery.isError || tasksQuery.data === undefined;

  const badges = deriveAttentionBadges({
    sessions: attentionSessions,
    sessionsStale: attentionSessionsStale,
    dashboard,
    tasksStale: tasksDisconnected,
  });
  const rows = deriveAttentionRows({
    sessions: attentionSessions,
    tasks,
    sessionRowsStale: attentionSessionsStale,
    taskRowsStale: taskRowsDisconnected,
    loopWaitingPresent,
    loopAttentionPresent,
  });
  return {
    badges,
    notificationCount: attentionCount(badges),
    rows,
    sessions: modalSessions,
    archivedSessions,
    archivedSessionsTotal: archivedSessionsQuery.total,
    sessionsDisconnected,
    tasksDisconnected: taskRowsDisconnected,
    loading:
      (sessionsEnabled &&
        (!worktree.resolved ||
          attentionSessionsQuery.isLoading ||
          modalSessionsQuery.isLoading ||
          archivedSessionsQuery.isLoading)) ||
      (tasksEnabled && (dashboardQuery.isLoading || tasksQuery.isLoading)),
  };
}
