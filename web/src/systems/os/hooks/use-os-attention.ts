import { useDocumentVisible } from "@/hooks/use-document-visible";

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
import { useUserHomeDir, type WorkspacePayload } from "@/systems/workspace";

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
  activeWorkspace: WorkspacePayload | null | undefined,
  sessionCatalogStreamStatus: SessionCatalogStreamStatus
): OsAttentionModel {
  const userHomeDir = useUserHomeDir();
  const documentVisible = useDocumentVisible();
  const workspaceId = activeWorkspace?.id ?? null;
  const sessionsEnabled = workspaceId !== null;
  const taskScope = taskScopeForActiveWorkspace(activeWorkspace, userHomeDir);
  const tasksEnabled = taskScope !== null;
  const sessionsQuery = useSessions(workspaceId, {
    enabled: sessionsEnabled,
    filters: { include_health: true, limit: 100, sort: "last_activity" },
  });
  const archivedSessionsQuery = useSessions(workspaceId, {
    enabled: sessionsEnabled,
    filters: { archive: "only", include_health: true, limit: 100, sort: "last_activity" },
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

  const sessions = sessionsQuery.data ?? [];
  const archivedSessions = archivedSessionsQuery.data ?? [];
  const dashboard = dashboardQuery.data ?? null;
  const tasks = tasksQuery.data ?? [];
  const sessionsDisconnected =
    !sessionsEnabled ||
    sessionCatalogStreamStatus !== "live" ||
    sessionsQuery.isError ||
    archivedSessionsQuery.isError ||
    sessionsQuery.data === undefined ||
    archivedSessionsQuery.data === undefined;
  const tasksDisconnected =
    !tasksEnabled ||
    dashboardQuery.isError ||
    dashboardQuery.data === undefined ||
    (dashboard?.freshness.stale ?? true);
  const taskRowsDisconnected =
    tasksDisconnected || tasksQuery.isError || tasksQuery.data === undefined;

  const badges = deriveAttentionBadges({
    sessions,
    sessionsStale: sessionsDisconnected,
    dashboard,
    tasksStale: tasksDisconnected,
  });
  const rows = deriveAttentionRows({
    sessions,
    tasks,
    sessionRowsStale: sessionsDisconnected,
    taskRowsStale: taskRowsDisconnected,
  });
  return {
    badges,
    notificationCount: attentionCount(badges),
    rows,
    sessions,
    archivedSessions,
    archivedSessionsTotal: archivedSessionsQuery.total,
    sessionsDisconnected,
    tasksDisconnected: taskRowsDisconnected,
    loading:
      (sessionsEnabled && (sessionsQuery.isLoading || archivedSessionsQuery.isLoading)) ||
      (tasksEnabled && (dashboardQuery.isLoading || tasksQuery.isLoading)),
  };
}
