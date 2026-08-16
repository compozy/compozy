import { useDocumentVisible } from "@/hooks/use-document-visible";

import { useLoopNodeExists } from "@/systems/loops";
import {
  attentionCount,
  deriveAttentionBadges,
  deriveAttentionSections,
  type OsAttentionBadges,
  type OsAttentionSections,
} from "../lib/attention-model";
import {
  sessionListSortParam,
  type SessionCatalogStreamStatus,
  type SessionPayload,
  useSessionListPreferences,
  useSessions,
} from "@/systems/session";
import { taskScopeForActiveWorkspace, useTaskDashboard, useTasks } from "@/systems/tasks";
import {
  useActiveWorkspace,
  useScopedWorktreeFilter,
  type WorkspacePayload,
} from "@/systems/workspace";

import { useAttentionPolicy } from "./use-attention-policy";
import { useAttentionSessions } from "./use-attention-rows";
import { useAttentionSummary } from "./use-attention-summary";
import { useFocusedWorktreeScopeId } from "./use-worktree-scope";

const ATTENTION_REFETCH_INTERVAL_MS = 5_000;

export interface OsAttentionModel {
  badges: OsAttentionBadges;
  notificationCount: number;
  sections: OsAttentionSections;
  sessions: SessionPayload[];
  archivedSessions: SessionPayload[];
  archivedSessionsTotal: number;
  attentionSessionsDisconnected: boolean;
  sessionsDisconnected: boolean;
  tasksDisconnected: boolean;
  loading: boolean;
}

/**
 * The shell's attention view model.
 *
 * Counts and rows have separate authorities on purpose. Counts come from the
 * daemon's cross-workspace summary projection, so they stay exact past any page
 * size and past any worktree scope. Rows come from server-filtered
 * cross-workspace reads. The modal and archived legs stay worktree-scoped
 * because they are a window's content, not an attention signal.
 */
export function useOsAttention(
  runtimeWorkspace: WorkspacePayload | null | undefined,
  sessionCatalogStreamStatus: SessionCatalogStreamStatus
): OsAttentionModel {
  const { scope, activeWorkspaceId, workspaces } = useActiveWorkspace();
  const documentVisible = useDocumentVisible();
  const workspaceId = runtimeWorkspace?.id ?? null;
  const sessionsEnabled = workspaceId !== null;
  const taskScope = taskScopeForActiveWorkspace(scope, activeWorkspaceId);
  const tasksEnabled = taskScope !== null;
  const worktree = useScopedWorktreeFilter(workspaceId, useFocusedWorktreeScopeId(), {
    enabled: sessionsEnabled && scope === "workspace",
  });
  const scopedSessionsEnabled = sessionsEnabled && worktree.resolved;
  const policy = useAttentionPolicy();
  // The modal renders the same order the operator chose everywhere else.
  const listPreferences = useSessionListPreferences();
  const summary = useAttentionSummary(sessionCatalogStreamStatus);
  const attention = useAttentionSessions(sessionCatalogStreamStatus, sessionsEnabled);
  // Sessions-modal content: a shell surface, so it follows the focused window's
  // scope exactly like the menubar chip. Distinct query key, no shared snapshot.
  const modalSessionsQuery = useSessions(workspaceId, {
    enabled: scopedSessionsEnabled,
    loadAll: listPreferences.scope === "all",
    filters: {
      include_health: true,
      limit: 100,
      sort: sessionListSortParam(listPreferences.sort),
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

  const modalSessions = modalSessionsQuery.data ?? [];
  const archivedSessions = archivedSessionsQuery.data ?? [];
  const dashboard = dashboardQuery.data ?? null;
  const tasks = tasksQuery.data ?? [];
  // Staleness follows each consumer: counts must not read fresh off a stale
  // summary, and the modal must not read fresh off the attention rows.
  const sessionsDisconnected =
    !sessionsEnabled ||
    (worktree.resolved &&
      (sessionCatalogStreamStatus !== "live" ||
        modalSessionsQuery.isError ||
        archivedSessionsQuery.isError ||
        modalSessionsQuery.data === undefined ||
        archivedSessionsQuery.data === undefined));
  const attentionSessionsDisconnected = attention.stale || summary.stale;
  const tasksDisconnected =
    !tasksEnabled ||
    dashboardQuery.isError ||
    dashboardQuery.data === undefined ||
    (dashboard?.freshness.stale ?? true);
  const taskRowsDisconnected =
    tasksDisconnected || tasksQuery.isError || tasksQuery.data === undefined;

  const badges = deriveAttentionBadges({
    summary: summary.summary,
    summaryStale: summary.stale,
    dashboard,
    tasksStale: tasksDisconnected,
  });
  const sections = deriveAttentionSections({
    sessions: attention.sessions,
    sessionRowsStale: attention.stale,
    workspaceLabels: new Map(workspaces.map(workspace => [workspace.id, workspace.name])),
    mutedWorkspaceIds: policy.mutedWorkspaceIds,
    tasks,
    taskRowsStale: taskRowsDisconnected,
    loopWaitingPresent,
    loopAttentionPresent,
  });
  return {
    badges,
    notificationCount: attentionCount(badges),
    sections,
    sessions: modalSessions,
    archivedSessions,
    archivedSessionsTotal: archivedSessionsQuery.total,
    attentionSessionsDisconnected,
    sessionsDisconnected,
    tasksDisconnected: taskRowsDisconnected,
    loading:
      (sessionsEnabled &&
        (!worktree.resolved ||
          summary.loading ||
          attention.loading ||
          modalSessionsQuery.isLoading ||
          archivedSessionsQuery.isLoading)) ||
      (tasksEnabled && (dashboardQuery.isLoading || tasksQuery.isLoading)),
  };
}
