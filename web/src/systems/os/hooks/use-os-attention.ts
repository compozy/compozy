import { useDocumentVisible } from "@/hooks/use-document-visible";
import { useQuery } from "@tanstack/react-query";

import { useLoopNodeExists, useLoopRequestAttention } from "@/systems/loops";
import { useProfileReadScope } from "@/systems/profiles";
import {
  deriveAttentionBadges,
  type OsAttentionBadges,
  type OsAttentionSections,
  type OsAttentionRow,
} from "../lib/attention-model";
import {
  sessionListSortParam,
  type SessionCatalogStreamStatus,
  type SessionPayload,
  useSessionListPreferences,
  useSessionCatalog,
  sessionFacetsOptions,
} from "@/systems/session";
import { taskScopeForActiveWorkspace, useTaskDashboard, useTasks } from "@/systems/tasks";
import {
  projectTerminalBadge,
  terminalInputRequestsQuery,
  terminalScope,
  terminalScopeKey,
} from "@/systems/terminal";
import {
  useActiveWorkspace,
  useScopedWorktreeFilter,
  type WorkspacePayload,
  type WorkspaceScopeMode,
} from "@/systems/workspace";

import { useBellNotifications } from "./use-bell-notifications";

import { useAttentionPolicy } from "./use-attention-policy";
import { useAttentionSummary } from "./use-attention-summary";
import { useFocusedWorktreeScopeId } from "./use-worktree-scope";

const ATTENTION_REFETCH_INTERVAL_MS = 5_000;

export interface OsAttentionModel {
  badges: OsAttentionBadges;
  notificationCount: number;
  notificationTotal?: number;
  notificationSnapshot?: string;
  acknowledging?: boolean;
  acknowledgementError?: string | null;
  onAcknowledge?: (row?: OsAttentionRow) => void;
  sections: OsAttentionSections;
  sessions: SessionPayload[];
  attentionSessionsDisconnected: boolean;
  sessionsDisconnected: boolean;
  tasksDisconnected: boolean;
  loopRequestsDisconnected: boolean;
  loading: boolean;
}

/**
 * The shell's attention view model. Session and terminal totals come from
 * scoped daemon projections; bell rows come from the notification ledger.
 * The optional modal page follows the focused window's workspace/worktree,
 * while the production session list owns explicit continuation separately.
 */
export function useOsAttention(
  runtimeWorkspace: WorkspacePayload | null | undefined,
  sessionCatalogStreamStatus: SessionCatalogStreamStatus,
  archived: boolean,
  modalEnabled = true
): OsAttentionModel {
  const { scope, activeWorkspaceId, workspaces } = useActiveWorkspace();
  const documentVisible = useDocumentVisible();
  const workspaceId = runtimeWorkspace?.id ?? null;
  const sessions = useSessionAttentionSources({
    archived,
    scope,
    sessionCatalogStreamStatus,
    workspaceId,
    modalEnabled,
  });
  const tasks = useTaskAttentionSources({ activeWorkspaceId, documentVisible, scope });
  const loops = useLoopAttentionSources({ documentVisible, workspaceId, workspaces });
  const terminal = useTerminalAttentionSources({
    documentVisible,
    workspaceId,
  });
  const policy = useAttentionPolicy();

  const baseBadges = deriveAttentionBadges({
    summary: sessions.summary.summary,
    summaryStale: sessions.summary.stale,
    dashboard: tasks.dashboard,
    tasksStale: tasks.disconnected,
    loopsPending: loops.requests.pendingCount,
  });
  const badges = {
    ...baseBadges,
    ...(terminal.badge === undefined ? {} : { terminal: terminal.badge }),
  };
  const notifications = useBellNotifications(policy.mutedWorkspaceIds);
  return {
    badges,
    notificationCount: notifications.count,
    notificationTotal: notifications.total,
    notificationSnapshot: notifications.snapshot,
    acknowledging: notifications.pending,
    acknowledgementError: notifications.error,
    onAcknowledge: notifications.acknowledge,
    sections: notifications.sections,
    sessions: sessions.modal,
    attentionSessionsDisconnected: notifications.stale,
    sessionsDisconnected: sessions.disconnected,
    tasksDisconnected: notifications.stale,
    loopRequestsDisconnected: notifications.stale,
    loading: notifications.loading,
  };
}

function useSessionAttentionSources({
  archived,
  scope,
  sessionCatalogStreamStatus,
  workspaceId,
  modalEnabled,
}: {
  modalEnabled: boolean;
  archived: boolean;
  scope: WorkspaceScopeMode;
  sessionCatalogStreamStatus: SessionCatalogStreamStatus;
  workspaceId: string | null;
}) {
  const enabled = scope === "global" || workspaceId !== null;
  const worktree = useScopedWorktreeFilter(workspaceId, useFocusedWorktreeScopeId(), {
    enabled: enabled && scope === "workspace",
  });
  // The modal renders the same order the operator chose everywhere else.
  const listPreferences = useSessionListPreferences();
  const summary = useAttentionSummary(sessionCatalogStreamStatus);
  // Sessions-modal content: a shell surface, so it follows the focused window's
  // scope exactly like the menubar chip. Distinct query key, no shared snapshot.
  const modalSessionsQuery = useSessionCatalog(
    workspaceId,
    {
      include_health: true,
      limit: 100,
      sort: sessionListSortParam(listPreferences.sort),
      worktree: worktree.worktreeId,
      ...(archived ? { archive: "only" as const } : {}),
    },
    modalEnabled && enabled && worktree.resolved,
    { facets: false }
  );
  return {
    disconnected:
      !enabled ||
      (worktree.resolved &&
        (sessionCatalogStreamStatus !== "live" ||
          (modalEnabled && (modalSessionsQuery.failed || modalSessionsQuery.loading)))),
    loading: enabled && (!worktree.resolved || summary.loading || modalSessionsQuery.loading),
    modal: modalSessionsQuery.sessions,
    summary,
  };
}

function useTaskAttentionSources({
  activeWorkspaceId,
  documentVisible,
  scope,
}: {
  activeWorkspaceId: string | null;
  documentVisible: boolean;
  scope: WorkspaceScopeMode;
}) {
  const taskScope = taskScopeForActiveWorkspace(scope, activeWorkspaceId);
  const enabled = taskScope !== null;
  const dashboardQuery = useTaskDashboard(taskScope ?? {}, {
    enabled,
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
      enabled,
      refetchIntervalMs: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
    }
  );
  const dashboard = dashboardQuery.data ?? null;
  const disconnected =
    !enabled ||
    dashboardQuery.isError ||
    dashboardQuery.data === undefined ||
    (dashboard?.freshness.stale ?? true);
  return {
    dashboard,
    disconnected,
    loading: enabled && (dashboardQuery.isLoading || tasksQuery.isLoading),
    rows: tasksQuery.data ?? [],
    rowsDisconnected: disconnected || tasksQuery.isError || tasksQuery.data === undefined,
  };
}

function useLoopAttentionSources({
  documentVisible,
  workspaceId,
  workspaces,
}: {
  documentVisible: boolean;
  workspaceId: string | null;
  workspaces: WorkspacePayload[];
}) {
  const enabled = workspaceId !== null;
  const loopWorkspaceId = workspaceId ?? "";
  return {
    waitingPresent: useLoopNodeExists(loopWorkspaceId, "waiting", enabled),
    attentionPresent: useLoopNodeExists(loopWorkspaceId, "attention", enabled),
    requests: useLoopRequestAttention(workspaces, true, documentVisible),
  };
}

function useTerminalAttentionSources({
  documentVisible,
  workspaceId,
}: {
  documentVisible: boolean;
  workspaceId: string | null;
}) {
  const profile = useProfileReadScope();
  const enabled = workspaceId !== null;
  const terminalReadScope = terminalScope(workspaceId ?? "", profile.destination);
  const terminalRequests = useQuery({
    ...terminalInputRequestsQuery(terminalReadScope),
    enabled,
    refetchInterval: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
  });
  const terminalFacets = useQuery({
    ...sessionFacetsOptions({
      workspace_id: workspaceId ?? undefined,
      profile: profile.destination,
    }),
    enabled,
    refetchInterval: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
  });
  const ready =
    enabled &&
    !terminalRequests.isError &&
    terminalRequests.data !== undefined &&
    !terminalFacets.isError &&
    terminalFacets.data !== undefined;
  const profileId = profile.destinationOwner?.id;
  return {
    badge:
      !ready || !profileId
        ? undefined
        : projectTerminalBadge({
            scopeKey: terminalScopeKey(
              terminalReadScope.key.workspaceId,
              terminalReadScope.key.profileKey
            ),
            profileId,
            inputRequests: terminalRequests.data?.pending ?? [],
            pendingApprovalCount: terminalFacets.data?.facets.terminal_approvals ?? 0,
          }).count,
    loading: terminalRequests.isLoading || terminalFacets.isLoading,
    ready,
  };
}
